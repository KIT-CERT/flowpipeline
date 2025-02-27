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
	DefaultDNSResolutionTimeout = 1 * time.Second
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
			log.Printf("[error] ResolveIPs: Invalid 'queuelength' parameter %s, using default of %u\n", queueLength, DefaultQueueLength)
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
			result []string
			err    error
		)
		// Source IP (with support for addrstrings segment)
		ctx, cancel := context.WithTimeout(context.Background(), DefaultDNSResolutionTimeout)
		defer cancel()
		if flow.SourceIP != "" {
			result, err = net.DefaultResolver.LookupAddr(ctx, flow.SourceIP)
		} else {
			result, err = net.DefaultResolver.LookupAddr(ctx, flow.SrcAddrObj().String())
		}
		if err == nil {
			flow.SrcHostName = result[0]
		}
		// Destination IP (with support for addrstrings segment)
		ctx, cancel = context.WithTimeout(context.Background(), DefaultDNSResolutionTimeout)
		defer cancel()
		if flow.DestinationIP != "" {
			result, err = net.DefaultResolver.LookupAddr(ctx, flow.DestinationIP)
		} else {
			result, err = net.DefaultResolver.LookupAddr(ctx, flow.DstAddrObj().String())
		}
		if err == nil {
			flow.DstHostName = result[0]
		}
		// Next Hop IP (with support for addrstrings segment)
		ctx, cancel = context.WithTimeout(context.Background(), DefaultDNSResolutionTimeout)
		defer cancel()
		if flow.NextHopIP != "" {
			result, err = net.DefaultResolver.LookupAddr(ctx, flow.NextHopIP)
		} else {
			result, err = net.DefaultResolver.LookupAddr(ctx, flow.NextHopObj().String())
		}
		if err == nil {
			flow.NextHopHostName = result[0]
		}
		// Sampler Address IP (with support for addrstrings segment)
		ctx, cancel = context.WithTimeout(context.Background(), DefaultDNSResolutionTimeout)
		defer cancel()
		if flow.SamplerIP != "" {
			result, err = net.DefaultResolver.LookupAddr(ctx, flow.SamplerIP)
		} else {
			result, err = net.DefaultResolver.LookupAddr(ctx, flow.SamplerAddressObj().String())
		}
		if err == nil {
			flow.SamplerHostName = result[0]
		}

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
