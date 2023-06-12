// The flowpipeline utility unifies all bwNetFlow functionality and
// provides configurable pipelines to process flows in any manner.
//
// The main entrypoint accepts command line flags to point to a configuration
// file and to establish the log level.
package main

import (
	"flag"
	"fmt"
	"github.com/bwNetFlow/flowpipeline/pipeline"
	_ "github.com/bwNetFlow/flowpipeline/segments/alert/http"
	_ "github.com/bwNetFlow/flowpipeline/segments/controlflow/branch"
	_ "github.com/bwNetFlow/flowpipeline/segments/export/clickhouse"
	_ "github.com/bwNetFlow/flowpipeline/segments/export/influx"
	_ "github.com/bwNetFlow/flowpipeline/segments/export/prometheus"
	"github.com/hashicorp/logutils"
	"github.com/pkg/profile"
	"log"
	"os"
	"os/signal"
	"path"
	"plugin"
	"strings"
	"time"

	_ "github.com/bwNetFlow/flowpipeline/segments/filter/drop"
	_ "github.com/bwNetFlow/flowpipeline/segments/filter/elephant"

	_ "github.com/bwNetFlow/flowpipeline/segments/filter/flowfilter"

	_ "github.com/bwNetFlow/flowpipeline/segments/input/bpf"
	_ "github.com/bwNetFlow/flowpipeline/segments/input/goflow"
	_ "github.com/bwNetFlow/flowpipeline/segments/input/kafkaconsumer"
	_ "github.com/bwNetFlow/flowpipeline/segments/input/packet"
	_ "github.com/bwNetFlow/flowpipeline/segments/input/stdin"

	_ "github.com/bwNetFlow/flowpipeline/segments/modify/addcid"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/addrstrings"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/anonymize"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/aslookup"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/bgp"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/dropfields"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/geolocation"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/normalize"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/protomap"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/remoteaddress"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/reversedns"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/snmp"

	_ "github.com/bwNetFlow/flowpipeline/segments/pass"

	_ "github.com/bwNetFlow/flowpipeline/segments/output/csv"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/json"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/kafkaproducer"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/lumberjack"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/sqlite"

	_ "github.com/bwNetFlow/flowpipeline/segments/print/count"
	_ "github.com/bwNetFlow/flowpipeline/segments/print/printdots"
	_ "github.com/bwNetFlow/flowpipeline/segments/print/printflowdump"
	_ "github.com/bwNetFlow/flowpipeline/segments/print/toptalkers"

	_ "github.com/bwNetFlow/flowpipeline/segments/analysis/toptalkers_metrics"
)

var Version string

type flagArray []string

func (i *flagArray) String() string {
	return strings.Join(*i, ",")
}

func (i *flagArray) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	var pluginPaths flagArray
	flag.Var(&pluginPaths, "p", "path to load segment plugins from, can be specified multiple times")
	loglevel := flag.String("l", "warning", "loglevel: one of 'debug', 'info', 'warning' or 'error'")
	version := flag.Bool("v", false, "print version")
	configfile := flag.String("c", "config.yml", "location of the config file in yml format")
	profilingType := flag.String("profiling", "", "enable profiling, one of 'cpu', 'mem', 'memheap', 'memallocs'")
	profilingPath := flag.String("profiling-path", "", "path to write profiling data to")
	profilingDuration := flag.Duration("profiling-duration", 60*time.Second, "duration of profiling")
	flag.Parse()

	if *version {
		fmt.Println(Version)
		return
	}

	if *profilingPath == "" {
		*profilingPath = "."
	}

	switch *profilingType {
	case "cpu":
		go func() {
			p := profile.Start(profile.CPUProfile, profile.ProfilePath(*profilingPath), profile.NoShutdownHook)
			time.Sleep(*profilingDuration)
			p.Stop()
			log.Printf("[info] CPU profile written to %s", path.Join(*profilingPath, "cpu.pprof"))
		}()
	case "mem":
		go func() {
			p := profile.Start(profile.MemProfile, profile.ProfilePath(*profilingPath), profile.NoShutdownHook)
			time.Sleep(*profilingDuration)
			p.Stop()
			log.Printf("[info] Memory profile written to %s", path.Join(*profilingPath, "mem.pprof"))
		}()
	case "memheap":
		go func() {
			p := profile.Start(profile.MemProfileHeap, profile.ProfilePath(*profilingPath), profile.NoShutdownHook)
			time.Sleep(*profilingDuration)
			p.Stop()
			log.Printf("[info] MemProfileHeap profile written to %s", path.Join(*profilingPath, "mem.pprof"))
		}()
	case "memallocs":
		go func() {
			p := profile.Start(profile.MemProfileAllocs, profile.ProfilePath(*profilingPath), profile.NoShutdownHook)
			time.Sleep(*profilingDuration)
			p.Stop()
			log.Printf("[info] MemProfileAllocs profile written to %s", path.Join(*profilingPath, "mem.pprof"))
		}()
	case "":
		// do nothing
	default:
		log.Fatalf("[error] Unknown profiling type: %s", *profilingType)
	}

	log.SetOutput(&logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"debug", "info", "warning", "error"},
		MinLevel: logutils.LogLevel(*loglevel),
		Writer:   os.Stderr,
	})

	for _, path := range pluginPaths {
		_, err := plugin.Open(path)
		if err != nil {
			if err.Error() == "plugin: not implemented" {
				log.Println("[error] Loading plugins is unsupported when running a static, not CGO-enabled binary.")
			} else {
				log.Printf("[error] Problem loading the specified plugin: %s", err)
			}
			return
		} else {
			log.Printf("[info] Loaded plugin: %s", path)
		}
	}

	config, err := os.ReadFile(*configfile)
	if err != nil {
		log.Printf("[error] reading config file: %s", err)
		return
	}
	pipe := pipeline.NewFromConfig(config)
	pipe.Start()
	pipe.AutoDrain()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	<-sigs

	pipe.Close()
}
