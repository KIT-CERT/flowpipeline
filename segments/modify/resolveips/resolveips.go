package resolveips

import (
	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"log"
	"strconv"
	"sync"
)

const (
	DefaultQueueLength = 65536
)

// ResolveIPs is a segment that resolves IP addresses to hostnames.
type ResolveIPs struct {
	ResolveQueue chan *FutureResolver
	segments.BaseSegment
}

func (segment *ResolveIPs) New(config map[string]string) segments.Segment {
	var NewSegment = ResolveIPs{}

	// build ingress queue
	queueLength, exists := config["queuelength"]
	if !exists || queueLength == "" {
		NewSegment.ResolveQueue = make(chan *FutureResolver, DefaultQueueLength)
	} else {
		if queuelen, err := strconv.ParseUint(queueLength, 10, 32); err == nil {
			NewSegment.ResolveQueue = make(chan *FutureResolver, queuelen)
		} else {
			log.Printf("[error] ResolveIPs: Invalid 'queuelength' parameter %s, using default of %u\n", queueLength, DefaultQueueLength)
			NewSegment.ResolveQueue = make(chan *FutureResolver, DefaultQueueLength)
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
			flow.mu.Unlock()
		}
	}(segment)

	for flow := range segment.In {
		segment.ResolveQueue <- NewFutureResolver(flow)
	}
}

type FutureResolver struct {
	mu sync.Mutex
}

func NewFutureResolver(flow *pb.EnrichedFlow) *FutureResolver {
	var fr = FutureResolver{}
	fr.mu.Lock()
	go func(flow *pb.EnrichedFlow, fr *FutureResolver) {
		// TODO: resolve stuff…
		fr.mu.Unlock()
	}(flow, &fr)

	return &fr
}

func (r *FutureResolver) WaitForResult(flow *pb.EnrichedFlow) {
}
