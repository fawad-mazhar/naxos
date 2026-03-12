// Package queue provides message queue clients for job distribution.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/fawad-mazhar/naxos/internal/config"
	"github.com/fawad-mazhar/naxos/internal/models"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	streamSetupTimeout = 10 * time.Second
	consumerAckWait    = 5 * time.Minute
	msgChannelBuffer   = 64
)

// NATS wraps a NATS JetStream connection for publishing and consuming jobs.
type NATS struct {
	conn   *nats.Conn
	js     jetstream.JetStream
	config config.NATSConfig
}

// NewNATS creates a new NATS client with JetStream support.
func NewNATS(cfg config.NATSConfig) (*NATS, error) {
	nc, err := nats.Connect(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	n := &NATS{
		conn:   nc,
		js:     js,
		config: cfg,
	}

	if err := n.ensureStream(); err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to ensure stream: %w", err)
	}

	return n, nil
}

func (n *NATS) ensureStream() error {
	ctx, cancel := context.WithTimeout(context.Background(), streamSetupTimeout)
	defer cancel()

	_, err := n.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      n.config.StreamName,
		Subjects:  []string{n.config.Subject},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("failed to create/update stream: %w", err)
	}

	return nil
}

// PublishJob publishes a job execution message to the NATS JetStream subject.
func (n *NATS) PublishJob(ctx context.Context, job *models.JobExecution) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	_, err = n.js.Publish(ctx, n.config.Subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish job: %w", err)
	}

	return nil
}

// ConsumeJobs creates a durable consumer with queue group and returns a channel of job messages.
func (n *NATS) ConsumeJobs(ctx context.Context) (<-chan jetstream.Msg, error) {
	consumer, err := n.js.CreateOrUpdateConsumer(ctx, n.config.StreamName, jetstream.ConsumerConfig{
		Durable:       n.config.QueueGroup,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		AckWait:       consumerAckWait,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	msgChan := make(chan jetstream.Msg, msgChannelBuffer)

	cctx, err := consumer.Consume(func(msg jetstream.Msg) {
		msgChan <- msg
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start consuming: %w", err)
	}

	// Stop consuming when context is cancelled
	go func() {
		<-ctx.Done()
		cctx.Stop()
		close(msgChan)
	}()

	log.Printf("Started consuming from stream %s with consumer %s", n.config.StreamName, n.config.QueueGroup)

	return msgChan, nil
}

// Close closes the NATS connection.
func (n *NATS) Close() error {
	n.conn.Close()
	return nil
}
