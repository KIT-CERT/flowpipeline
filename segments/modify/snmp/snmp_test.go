package snmp

import (
	"testing"
)

// SNMPInterface Segment test
// TODO: find a way to run this elsewhere, as this currently only works by
// having the local 161/udp port forwarded to some router.
// func TestSegment_SNMPInterface(t *testing.T) {
// 	result := testSegmentWithFlows(
// 		&SNMPInterface{
// 			Community: "public",
// 			Regex:     "^_[a-z]{3}_[0-9]{5}_[0-9]{5}_ [A-Z0-9]+ (.*?) *( \\(.*)?$",
// 			ConnLimit: 1,
// 		},
// 		[]*flow.FlowMessage{
// 			&flow.FlowMessage{Type: 42, SamplerAddress: []byte{127, 0, 0, 1}, InIf: 70},
// 			&flow.FlowMessage{SamplerAddress: []byte{127, 0, 0, 1}, InIf: 70},
// 		})
// 	if result.SrcIfDesc == "" {
// 		t.Error("Segment SNMPInterface is not adding a SrcIfDesc.")
// 	}
// }

func TestSegment_SNMPInterface_instanciation(t *testing.T) {
	snmpInterface := &SNMPInterface{}
	result := snmpInterface.New(map[string]string{})
	if result == nil {
		t.Error("Segment SNMPInterface did not initiate despite good base config.")
	}

	snmpInterface = &SNMPInterface{}
	result = snmpInterface.New(map[string]string{"connlimit": "42"})
	if result == nil {
		t.Error("Segment SNMPInterface did not initiate despite good base config.")
	}

	snmpInterface = &SNMPInterface{}
	result = snmpInterface.New(map[string]string{"community": "foo", "regex": ".*"})
	if result == nil {
		t.Error("Segment SNMPInterface did not initiate despite good config.")
	}

	snmpInterface = &SNMPInterface{}
	result = snmpInterface.New(map[string]string{"community": "foo", "regex": "("})
	if result != nil {
		t.Error("Segment SNMPInterface did initiate despite bad regex config.")
	}

	snmpInterface = &SNMPInterface{}
	result = snmpInterface.New(map[string]string{"connlimit": "-8"})
	if result == nil {
		t.Error("Segment SNMPInterface did not fallback to connlimit default config.")
	}

	snmpInterface = &SNMPInterface{}
	result = snmpInterface.New(map[string]string{"connlimit": "0"})
	if result != nil {
		t.Error("Segment SNMPInterface initiated despide bad config.")
	}
}
