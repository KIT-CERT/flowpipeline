package resolveips

import (
	"context"
	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	DefaultQueueLength          = 2_000_000 // 65536
	DefaultDNSResolutionTimeout = 2 * time.Second
)

// ResolveIPs is a segment that resolves IP addresses to hostnames.
type ResolveIPs struct {
	ResolveQueue chan *futureResolver
	segments.BaseSegment
}

func (segment *ResolveIPs) New(config map[string]string) segments.Segment {
	var NewSegment = ResolveIPs{}

	// build ingress queue
	queueLength, exists := config["queuelength"]
	if !exists || queueLength == "" {
		NewSegment.ResolveQueue = make(chan *futureResolver, DefaultQueueLength)
	} else {
		if queuelen, err := strconv.ParseUint(queueLength, 10, 32); err == nil {
			NewSegment.ResolveQueue = make(chan *futureResolver, queuelen)
		} else {
			log.Printf("[error] ResolveIPs: Invalid 'queuelength' parameter %s, using default of %d\n", queueLength, DefaultQueueLength)
			NewSegment.ResolveQueue = make(chan *futureResolver, DefaultQueueLength)
		}
	}
	return &NewSegment
}

func (segment *ResolveIPs) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	// goroutine that fetches resolved flows and queues them to the next segment
	go func(segment *ResolveIPs) {
		for flow := range segment.ResolveQueue {
			segment.Out <- flow.WaitForResult()
		}
	}(segment)

	for flow := range segment.In {
		segment.ResolveQueue <- NewFutureResolver(flow)
	}
}

// FutureResolver is an interface that provides a single function that
// returns a result when it's finished
type FutureResolver interface {
	WaitForResult() *pb.EnrichedFlow
}

// futureResolver implements FutureResolver but cannot be created without the
// help of NewFutureResolver()
type futureResolver struct {
	result chan *pb.EnrichedFlow
}

func NewFutureResolver(flow *pb.EnrichedFlow) *futureResolver {
	var fr = futureResolver{
		result: make(chan *pb.EnrichedFlow), // blocking channel
	}
	go func(flow *pb.EnrichedFlow, fr *futureResolver) {
		var (
			wg sync.WaitGroup
		)
		ctx, cancel := context.WithTimeout(context.Background(), DefaultDNSResolutionTimeout)
		defer cancel()

		resolveHostName := func(ip string, getAddrObjFunc func() string, assignHostNameFunc func(string)) {
			defer wg.Done()
			var (
				result []string
				err    error
			)
			if ip != "" {
				result, err = net.DefaultResolver.LookupAddr(ctx, ip)
			} else {
				result, err = net.DefaultResolver.LookupAddr(ctx, getAddrObjFunc())
			}
			if err == nil && len(result) > 0 {
				assignHostNameFunc(result[0])
			}
		}

		wg.Add(4)
		// Source IP (with support for addrstrings segment)
		go resolveHostName(flow.SourceIP, flow.SrcAddrObj().String, func(hostName string) {
			flow.SrcHostName = hostName
		})
		// Destination IP (with support for addrstrings segment)
		go resolveHostName(flow.DestinationIP, flow.DstAddrObj().String, func(hostName string) {
			flow.DstHostName = hostName
		})
		// Next Hop IP (with support for addrstrings segment)
		go resolveHostName(flow.NextHopIP, flow.NextHopObj().String, func(hostName string) {
			flow.NextHopHostName = hostName
		})
		// Sampler Address IP (with support for addrstrings segment)
		go resolveHostName(flow.SamplerIP, flow.SamplerAddressObj().String, func(hostName string) {
			flow.SamplerHostName = hostName
		})

		wg.Wait()
		fr.result <- flow

	}(flow, &fr)

	return &fr
}

func (r *futureResolver) WaitForResult() *pb.EnrichedFlow {
	return <-r.result
}

func init() {
	segment := &ResolveIPs{}
	segments.RegisterSegment("resolveips", segment)
}
