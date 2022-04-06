package elephant

import (
	"io/ioutil"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/bwNetFlow/flowpipeline/pb"
	"github.com/bwNetFlow/flowpipeline/segments"
)

// Elephant Segment test, passthrough test
func TestSegment_Elephant_passthrough(t *testing.T) {
	segment := segments.LookupSegment("elephant").New(map[string]string{})
	if segment == nil {
		log.Fatal("[error] Configured segment 'elephant' could not be initialized properly, see previous messages.")
	}

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	in <- &pb.EnrichedFlow{Bytes: 10}
	<-out
	in <- &pb.EnrichedFlow{Bytes: 9}
	in <- &pb.EnrichedFlow{Bytes: 100}
	result := <-out
	if result.Bytes != 100 {
		t.Error("Segment Elephant is not working.")
	}
	close(in)
	wg.Wait()
}

// Elephant Segment benchmark passthrough
func BenchmarkElephant(b *testing.B) {
	log.SetOutput(ioutil.Discard)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Elephant{}

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{}
		_ = <-out
	}
	close(in)
}
