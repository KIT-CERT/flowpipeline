// Captures Netflow v9 and feeds flows to the following segments. Currently,
// this segment only uses a limited subset of goflow2 functionality.
// If no configuration option is provided, an sflow and a netflow collector will be started.
// netflowLegacy is also built in but currently not tested.
package goflow

import (
	"bytes"
	"context"
	"errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"google.golang.org/protobuf/encoding/protodelim"

	"github.com/netsampler/goflow2/v2/utils/debug"

	"github.com/netsampler/goflow2/v2/transport"
	_ "github.com/netsampler/goflow2/v2/transport/file"
	_ "github.com/netsampler/goflow2/v2/transport/kafka"

	"github.com/netsampler/goflow2/v2/metrics"
	"github.com/netsampler/goflow2/v2/utils"

	// various formatters
	"github.com/netsampler/goflow2/v2/format"
	_ "github.com/netsampler/goflow2/v2/format/binary"

	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
)

type Goflow struct {
	segments.BaseSegment
	Listen                 []url.URL // optional, default config value for this slice is "sflow://:6343,netflow://:2055"
	Workers                uint64    // optional, amount of workers to spawn for each endpoint, default is 1
	Blocking               bool      //optional, default is false
	QueueSize              int       //default is 1000000
	NumSockets             int       //default is 1
	PrometheusStatsAddress string    // optional, if set to a valid URL, goflow2 prometheus stats will be served here

	goflow_in chan *pb.EnrichedFlow
}

func (segment Goflow) New(config map[string]string) segments.Segment {

	var listen = "sflow://:6343,netflow://:2055"
	if config["listen"] != "" {
		listen = config["listen"]
	}

	var listenAddressesSlice []url.URL
	for _, listenAddress := range strings.Split(listen, ",") {
		listenAddrUrl, err := url.Parse(listenAddress)
		if err != nil {
			log.Printf("[error] Goflow: error parsing listenAddresses: %e", err)
			return nil
		}
		// Check if given Port can be parsed to int
		_, err = strconv.ParseUint(listenAddrUrl.Port(), 10, 64)
		if err != nil {
			log.Printf("[error] Goflow: Port %s could not be converted to integer", listenAddrUrl.Port())
			return nil
		}

		switch listenAddrUrl.Scheme {
		case "netflow", "sflow", "nfl":
			log.Printf("[info] Goflow: Scheme %s supported.", listenAddrUrl.Scheme)
		default:
			log.Printf("[error] Goflow: Scheme %s not supported.", listenAddrUrl.Scheme)
			return nil
		}

		listenAddressesSlice = append(listenAddressesSlice, *listenAddrUrl)
	}
	log.Printf("[info] Goflow: Configured for for %s", listen)

	var workers uint64 = 1
	if config["workers"] != "" {
		if parsedWorkers, err := strconv.ParseUint(config["workers"], 10, 32); err == nil {
			workers = parsedWorkers
			if workers == 0 {
				log.Println("[error] Goflow: Limiting workers to 0 will not work. Remove this segment or use a higher value >= 1.")
				return nil
			}
		} else {
			log.Println("[error] Goflow: Could not parse 'workers' parameter, using default 1.")
		}
	} else {
		log.Println("[info] Goflow: 'workers' set to default '1'.")
	}

	return &Goflow{
		Listen:                 listenAddressesSlice,
		Workers:                workers,
		PrometheusStatsAddress: config["prometheus_stats_address"],
	}
}

func (segment *Goflow) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	segment.goflow_in = make(chan *pb.EnrichedFlow)
	segment.startGoFlow(&channelDriver{segment.goflow_in})
	for {
		select {
		case msg, ok := <-segment.goflow_in:
			if !ok {
				// do not return here, as this might leave the
				// segment.In channel blocking in our
				// predecessor segment
				segment.goflow_in = nil // make unavailable for select
				// TODO: think about restarting goflow?
			}
			segment.Out <- msg
		case msg, ok := <-segment.In:
			if !ok {
				return
			}
			segment.Out <- msg
		}
	}
}

type channelDriver struct {
	out chan *pb.EnrichedFlow
}

func (d *channelDriver) Send(key, data []byte) error {
	msg := new(pb.ProtoProducerMessage)
	// TODO: can we shave of this Unmarshal here by writing a custom formatter
	if err := protodelim.UnmarshalFrom(bytes.NewReader(data), msg); err != nil {
		log.Println("[error] Goflow: Conversion error for received flow.")
		return nil
	}
	d.out <- &msg.EnrichedFlow
	return nil
}

func (d *channelDriver) Close(context.Context) error {
	close(d.out)
	return nil
}

type ReceiverCallback struct {
	ReceiverMetric *metrics.ReceiverMetric
	DroppedPackets uint64
	CurrentTimer   *time.Timer
}

func NewReceiverCallback(prometheusEnabled bool) *ReceiverCallback {
	var receiverMetric *metrics.ReceiverMetric = nil
	if prometheusEnabled {
		receiverMetric = metrics.NewReceiverMetric()
	}

	return &ReceiverCallback{ReceiverMetric: receiverMetric}
}

func (rcb *ReceiverCallback) Dropped(pkt utils.Message) {
	rcb.DroppedPackets++
	if rcb.ReceiverMetric != nil {
		rcb.ReceiverMetric.Dropped(pkt)
	}

	oneSecond := time.Duration(1) * time.Second
	if rcb.CurrentTimer == nil {
		rcb.CurrentTimer = time.AfterFunc(oneSecond, func() {
			log.Printf("[warn] Dropped %d packets in the last seconds.", rcb.DroppedPackets)
			rcb.CurrentTimer = nil
		})
	}
}

func (segment *Goflow) startGoFlow(transport transport.TransportInterface) {
	formatter, err := format.FindFormat("bin")
	if err != nil {
		slog.Error("error formatter", slog.String("error", err.Error()))
		os.Exit(1)
	}
	var pipes []utils.FlowPipe

	for _, listenAddrUrl := range segment.Listen {
		go func(listenAddrUrl url.URL) {
			var err error

			hostname := listenAddrUrl.Hostname()
			port, _ := strconv.ParseUint(listenAddrUrl.Port(), 10, 64)

			if segment.NumSockets == 0 {
				segment.NumSockets = 1
			}

			if segment.QueueSize == 0 {
				segment.QueueSize = 1000000
			}

			cfg := &utils.UDPReceiverConfig{
				Sockets:          segment.NumSockets,
				Workers:          int(segment.Workers),
				QueueSize:        segment.QueueSize,
				Blocking:         segment.Blocking,
				ReceiverCallback: NewReceiverCallback(segment.PrometheusStatsAddress != ""),
			}

			recv, err := utils.NewUDPReceiver(cfg)
			if err != nil {
				log.Println("error creating UDP receiver", slog.String("error", err.Error()))
				os.Exit(1)
			}

			if err != nil {
				slog.Error("error transporter", slog.String("error", err.Error()))
				os.Exit(1)
			}

			var cfgProducer = &protoproducer.ProducerConfig{}
			cfgm, err := cfgProducer.Compile()
			if err != nil {
				log.Fatal(err)
			}
			flowProducer, err := protoproducer.CreateProtoProducer(cfgm, protoproducer.CreateSamplingSystem)
			if err != nil {
				slog.Error("error producer", slog.String("error", err.Error()))
				os.Exit(1)
			}
			// enable Prometheus stats
			if segment.PrometheusStatsAddress != "" {
				flowProducer = metrics.WrapPromProducer(flowProducer)
			}

			cfgPipe := &utils.PipeConfig{
				Format:           formatter,
				Transport:        transport,
				Producer:         flowProducer,
				NetFlowTemplater: metrics.NewDefaultPromTemplateSystem, // wrap template system to get Prometheus info
			}

			var pipeline utils.FlowPipe
			switch scheme := listenAddrUrl.Scheme; scheme {
			case "netflow":
				pipeline = utils.NewNetFlowPipe(cfgPipe)
				log.Printf("[info] Goflow: Listening for Netflow v9 on port %d...", port)
			case "sflow":
				pipeline = utils.NewSFlowPipe(cfgPipe)
				log.Printf("[info] Goflow: Listening for sflow on port %d...", port)
			case "flow":
				pipeline = utils.NewFlowPipe(cfgPipe)
				log.Printf("[info] Goflow: Listening for netflow legacy on port %d...", port)
			default:
				log.Fatal("scheme does not exist", slog.String("error", listenAddrUrl.Scheme))
			}

			decodeFunc := pipeline.DecodeFlow
			// intercept panic and generate error
			decodeFunc = debug.PanicDecoderWrapper(decodeFunc)
			// wrap decoder with Prometheus metrics
			decodeFunc = metrics.PromDecoderWrapper(decodeFunc, listenAddrUrl.Scheme)
			pipes = append(pipes, pipeline)

			err = recv.Start(hostname, int(port), decodeFunc)

			if err != nil {
				log.Fatalf("[error] Goflow: %s", err.Error())
			}

		}(listenAddrUrl)
	}

	// Start prometheus server if requested
	if segment.PrometheusStatsAddress != "" {
		http.Handle("/metrics", promhttp.Handler())
		srv := http.Server{Addr: segment.PrometheusStatsAddress}
		go func() {
			err := srv.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("[critical] Failed to start goflow2 prometheus server: %s", err.Error())
			}
		}()
	}
}

func init() {
	segment := &Goflow{}
	segments.RegisterSegment("goflow", segment)
}
