package lumberjack

import (
	"github.com/BelWue/flowpipeline/pb"
	//"golang.org/x/net/publicsuffix"
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
		ok                          bool
		SourceAS                    *AutonomousSystem = nil
		DestinationAS               *AutonomousSystem = nil
		srcIP                       netip.Addr
		SourceIPString              string
		SourceAddress               = ""
		SourceMAC                   = ""
		SourceDomain                = ""
		SourceRegisteredDomain      = ""
		SourceTopLevelDomain        = ""
		dstIP                       netip.Addr
		DestinationIPString         string
		DestinationAddress          = ""
		DestinationMAC              = ""
		DestinationDomain           = ""
		DestinationRegisteredDomain = ""
		DestinationTopLevelDomain   = ""
		RelatedHosts                = make([]string, 0, 2)
	)
	// source ip & address
	if enrichedFlow.SourceIP != "" { // use IP string from addrstring segment if available
		SourceIPString = enrichedFlow.SourceIP
		SourceAddress = enrichedFlow.SourceIP
		// TODO: set SourceIP
	} else {
		// parse source address
		srcIP, ok = netip.AddrFromSlice(enrichedFlow.SrcAddr)
		if ok {
			SourceIPString = srcIP.String()
			SourceAddress = SourceIPString
		}
	}
	// use hostname from reversedns segment if available
	/*	if enrichedFlow.SrcHostName != "" {
		SourceDomain = enrichedFlow.SrcHostName
		SourceRegisteredDomain, err = publicsuffix.EffectiveTLDPlusOne(enrichedFlow.SrcHostName[:len(enrichedFlow.SrcHostName)-2])
		if err != nil {
			SourceRegisteredDomain = ""
		}
		SourceTopLevelDomain, _ = publicsuffix.PublicSuffix(enrichedFlow.SrcHostName)
		RelatedHosts = append(RelatedHosts, enrichedFlow.SrcHostName)
	}*/
	// add source MAC if set
	if enrichedFlow.SrcMac != 0 {
		if enrichedFlow.SourceMAC != "" {
			SourceMAC = enrichedFlow.SourceMAC
		} else {
			SourceMAC = enrichedFlow.SrcMacString()
		}
	}

	// destination ip & address
	if enrichedFlow.DestinationIP != "" { // use IP string from addrstring segment if available
		DestinationIPString = enrichedFlow.DestinationIP
		DestinationAddress = enrichedFlow.DestinationIP
		// TODO: set DestinationIP
	} else {
		dstIP, ok = netip.AddrFromSlice(enrichedFlow.DstAddr)
		if ok {
			DestinationIPString = dstIP.String()
			DestinationAddress = DestinationIPString
		}
	}
	// use hostname from reversedns segment if available
	/*	if enrichedFlow.DstHostName != "" {
		DestinationDomain = enrichedFlow.DstHostName
		DestinationRegisteredDomain, err = publicsuffix.EffectiveTLDPlusOne(enrichedFlow.DstHostName[:len(enrichedFlow.DstHostName)-2])
		if err != nil {
			DestinationRegisteredDomain = ""
		}
		DestinationTopLevelDomain, _ = publicsuffix.PublicSuffix(enrichedFlow.DstHostName)
		RelatedHosts = append(RelatedHosts, enrichedFlow.DstHostName)
	}*/
	// add source MAC if set
	if enrichedFlow.DstMac != 0 {
		if enrichedFlow.DestinationMAC != "" {
			DestinationMAC = enrichedFlow.DestinationMAC
		} else {
			DestinationMAC = enrichedFlow.DstMacString()
		}
	}

	// AS
	srcAS := enrichedFlow.GetSrcAs()
	if srcAS != 0 {
		SourceAS = &AutonomousSystem{
			Number: srcAS,
		}
	}
	dstAS := enrichedFlow.GetDstAs()
	if dstAS != 0 {
		DestinationAS = &AutonomousSystem{
			Number: dstAS,
		}
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
			Duration: int64(enrichedFlow.GetTimeFlowEndNs() - enrichedFlow.GetTimeFlowStartNs()),
			Provider: ECSEventDefaultProvider,
			Module:   ECSEventDefaultModule,
		},
		Source: &ECSSourceOrDest{
			Address:          SourceAddress,
			Bytes:            enrichedFlow.GetBytes(),
			IP:               SourceIPString,
			MAC:              SourceMAC,
			Packets:          enrichedFlow.GetPackets(),
			Port:             enrichedFlow.GetSrcPort(),
			Domain:           SourceDomain,
			RegisteredDomain: SourceRegisteredDomain,
			//Subdomain:        SourceSubdomain,
			TopLevelDomain:   SourceTopLevelDomain,
			AutonomousSystem: SourceAS,
		},
		Destination: &ECSSourceOrDest{
			Address:          DestinationAddress,
			Bytes:            0, // flows are uni-directional
			IP:               DestinationIPString,
			MAC:              DestinationMAC,
			Packets:          0, // flows are uni-directional
			Port:             enrichedFlow.GetDstPort(),
			Domain:           DestinationDomain,
			RegisteredDomain: DestinationRegisteredDomain,
			//Subdomain:        DestinationSubdomain,
			TopLevelDomain:   DestinationTopLevelDomain,
			AutonomousSystem: DestinationAS,
		},
		Related: &ECSRelated{
			Hash:  nil,
			Hosts: RelatedHosts,
			IP:    []string{SourceIPString, DestinationIPString},
			User:  nil,
		},
	}
	return result
}
