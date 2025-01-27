package kafkaconsumer

import (
	"bytes"
	"context"
	"log"
	"log/slog"

	"github.com/IBM/sarama"
	"github.com/bwNetFlow/flowpipeline/pb"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"
)

// Handler represents a Sarama consumer group consumer
type Handler struct {
	ready  chan bool
	flows  chan *pb.EnrichedFlow
	legacy bool
	cancel context.CancelFunc
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (h *Handler) Setup(session sarama.ConsumerGroupSession) error {
	log.Println("[info] KafkaConsumer: Received new partition set to claim:", session.Claims()) // TODO: print those
	// reopen flows channel
	h.flows = make(chan *pb.EnrichedFlow)
	// Mark the consumer as ready
	close(h.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (h *Handler) Cleanup(sarama.ConsumerGroupSession) error {
	close(h.flows)
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().

func (h *Handler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if h.legacy {
				session.MarkMessage(message, "")
				flowMsg := new(pb.LegacyEnrichedFlow)
				if err := proto.Unmarshal(message.Value, flowMsg); err == nil {
					h.flows <- flowMsg.ConvertToEnrichedFlow()
				} else {
					log.Printf("[warning] KafkaConsumer: Error decoding flow, this might be due to the use of Goflow custom fields. Original error:\n  %s", err)
				}
			} else {
				msg := new(pb.ProtoProducerMessage)
				if err := protodelim.UnmarshalFrom(bytes.NewReader(message.Value), msg); err != nil {
					slog.Error("error unmarshalling message", slog.String("error", err.Error()))
					continue
				}
				h.flows <- &msg.EnrichedFlow
			}
		case <-session.Context().Done():
			return nil
		}
	}
}
