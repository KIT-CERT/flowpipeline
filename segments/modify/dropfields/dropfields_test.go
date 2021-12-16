package dropfields

import (
	"io/ioutil"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/bwNetFlow/flowpipeline/segments"
	flow "github.com/bwNetFlow/protobuf/go"
	"github.com/hashicorp/logutils"
)

func TestMain(m *testing.M) {
	log.SetOutput(&logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"info", "warning", "error"},
		MinLevel: logutils.LogLevel("info"),
		Writer:   os.Stderr,
	})
	code := m.Run()
	os.Exit(code)
}

// DropFields Segment tests are thorough and try every combination
func TestSegment_DropFields_policyKeep(t *testing.T) {
	result := segments.TestSegment("dropfields", map[string]string{"policy": "keep", "fields": "DstAddr"},
		&flow.FlowMessage{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}},
	)
	if len(result.SrcAddr) != 0 || len(result.DstAddr) == 0 {
		t.Error("Segment DropFields is not keeping the proper fields.")
	}
}

func TestSegment_DropFields_policyDrop(t *testing.T) {
	result := segments.TestSegment("dropfields", map[string]string{"policy": "drop", "fields": "SrcAddr"},
		&flow.FlowMessage{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}},
	)
	if len(result.SrcAddr) != 0 || len(result.DstAddr) == 0 {
		t.Error("Segment DropFields is not dropping the proper fields.")
	}
}

// DropFields Segment benchmark passthrough
func BenchmarkDropFields(b *testing.B) {
	log.SetOutput(ioutil.Discard)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := DropFields{}.New(map[string]string{"policy": "drop", "fields": "SrcAddr"})

	in, out := make(chan *flow.FlowMessage), make(chan *flow.FlowMessage)
	segment.Rewire([]chan *flow.FlowMessage{in, out}, 0, 1)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &flow.FlowMessage{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}}
		_ = <-out
	}
	close(in)
}
