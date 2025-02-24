package lumberjack

import (
	"github.com/BelWue/flowpipeline/pb"
	"net/netip"
	"time"
)

const (
	ElasticCommonSchemaVersion = "8.17"
)

type ECSECS struct {
	Version string `json:"version,omitempty"`
}

type ElasticCommonSchema struct {
	// Base Fields (https://www.elastic.co/guide/en/ecs/current/ecs-principles-implementation.html#_base_fields)
	Timestamp uint64            `json:"@timestamp,omitempty"`
	ECS       *ECSECS           `json:"ecs,omitempty"`
	Message   string            `json:"message,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`

	// ECS Categorization Fields (https://www.elastic.co/guide/en/ecs/current/ecs-category-field-values-reference.html)
	Event ECSEvent `json:"event"`

	// Source (https://www.elastic.co/guide/en/ecs/current/ecs-source.html) and
	// Destination (https://www.elastic.co/guide/en/ecs/current/ecs-destination.html)
	Source      *ECSSourceOrDest `json:"source,omitempty"`
	Destination *ECSSourceOrDest `json:"destination,omitempty"`

	// Related Fields (https://www.elastic.co/guide/en/ecs/current/ecs-related.html)
	Related *ECSRelated `json:"related,omitempty"`
}

func ECSFromEnrichedFlow(enrichedFlow *pb.EnrichedFlow) *ElasticCommonSchema {
	var (
		srcIP               netip.Addr
		SourceIP            *IPAddress
		SourceIPString      string
		dstIP               netip.Addr
		DestinationIP       *IPAddress
		DestinationIPString string
		ok                  bool
	)
	srcIP, ok = netip.AddrFromSlice(enrichedFlow.SrcAddr)
	if ok {
		SourceIP = (*IPAddress)(&srcIP)
		SourceIPString = srcIP.String()
	} else {
		SourceIP = nil
		SourceIPString = ""
	}
	dstIP, ok = netip.AddrFromSlice(enrichedFlow.DstAddr)
	if ok {
		DestinationIP = (*IPAddress)(&dstIP)
		DestinationIPString = dstIP.String()
	} else {
		DestinationIP = nil
		DestinationIPString = ""
	}

	result := &ElasticCommonSchema{
		Timestamp: enrichedFlow.GetTimeFlowStartMs(),
		ECS:       &ECSECS{Version: ElasticCommonSchemaVersion},
		Event: ECSEvent{
			Kind:     ECSEventKindEvent,
			Category: []ECSEventCategory{ECSEventCategoryNetwork},
			Type:     []ECSEventType{ECSEventTypeConnection},
			Outcome:  ECSEventOutcomeSuccess,
			Created:  time.Now().UnixMilli(),
			Start:    enrichedFlow.GetTimeFlowStartMs(),
			End:      enrichedFlow.GetTimeFlowEndMs(),
			Duration: enrichedFlow.GetTimeFlowEndNs() - enrichedFlow.GetTimeFlowStartNs(),
			Provider: ECSEventDefaultProvider,
			Module:   ECSEventDefaultModule,
		},
		Source: &ECSSourceOrDest{
			Address: SourceIPString, // TODO: use DNS name if known
			Bytes:   enrichedFlow.GetBytes(),
			IP:      SourceIP,
			MAC:     enrichedFlow.SrcMacString(),
			Packets: enrichedFlow.GetPackets(),
			Port:    enrichedFlow.GetSrcPort(),
			AutonomousSystem: &AutonomousSystem{
				Number: enrichedFlow.GetSrcAs(),
			},
		},
		Destination: &ECSSourceOrDest{
			Address: DestinationIPString, // TODO: use DNS name if known
			Bytes:   0,                   // flows are uni-directional
			IP:      DestinationIP,
			MAC:     enrichedFlow.DstMacString(),
			Packets: 0, // flows are uni-directional
			Port:    enrichedFlow.GetDstPort(),
			AutonomousSystem: &AutonomousSystem{
				Number: enrichedFlow.GetDstAs(),
			},
		},
		Related: &ECSRelated{
			Hash:  nil,
			Hosts: nil,
			IP:    []string{SourceIPString, DestinationIPString},
			User:  nil,
		},
	}
	return result
}
