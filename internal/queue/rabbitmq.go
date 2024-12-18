// internal/queue/rabbitmq.go
package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fawad-mazhar/naxos/internal/config"
	"github.com/fawad-mazhar/naxos/internal/models"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	conn          *amqp.Connection
	jobChannel    *amqp.Channel
	statusChannel *amqp.Channel
	config        config.RabbitMQConfig
}

func NewRabbitMQ(cfg config.RabbitMQConfig) (*RabbitMQ, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	jobCh, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open job channel: %w", err)
	}

	statusCh, err := conn.Channel()
	if err != nil {
		jobCh.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to open status channel: %w", err)
	}

	rmq := &RabbitMQ{
		conn:          conn,
		jobChannel:    jobCh,
		statusChannel: statusCh,
		config:        cfg,
	}

	if err := rmq.setupQueues(); err != nil {
		rmq.Close()
		return nil, fmt.Errorf("failed to setup queues: %w", err)
	}

	return rmq, nil
}

func (r *RabbitMQ) setupQueues() error {
	// Setup jobs queue
	err := r.jobChannel.ExchangeDeclare(
		r.config.Exchange,     // name
		r.config.ExchangeType, // type
		true,                  // durable
		false,                 // auto-deleted
		false,                 // internal
		false,                 // no-wait
		nil,                   // arguments
	)
	if err != nil {
		return err
	}

	_, err = r.jobChannel.QueueDeclare(
		r.config.JobsQueue, // name
		true,               // durable
		false,              // delete when unused
		false,              // exclusive
		false,              // no-wait
		nil,                // arguments
	)
	if err != nil {
		return err
	}

	err = r.jobChannel.QueueBind(
		r.config.JobsQueue, // queue name
		"jobs",             // routing key
		r.config.Exchange,  // exchange
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// Setup status queue with message TTL
	args := make(amqp.Table)
	args["x-message-ttl"] = 72 * 60 * 60 * 1000 // 72 hours in milliseconds

	_, err = r.statusChannel.QueueDeclare(
		r.config.StatusQueue, // name
		true,                 // durable
		false,                // delete when unused
		false,                // exclusive
		false,                // no-wait
		args,                 // arguments - including TTL
	)
	return err
}

func (r *RabbitMQ) PublishJob(ctx context.Context, job *models.JobExecution) error {
	data, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	return r.jobChannel.PublishWithContext(ctx,
		r.config.Exchange, // exchange
		"jobs",            // routing key
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         data,
			DeliveryMode: amqp.Persistent,
		},
	)
}

func (r *RabbitMQ) ConsumeJobs(ctx context.Context) (<-chan amqp.Delivery, error) {
	return r.jobChannel.Consume(
		r.config.JobsQueue, // queue
		"",                 // consumer
		false,              // auto-ack
		false,              // exclusive
		false,              // no-local
		false,              // no-wait
		nil,                // args
	)
}

func (r *RabbitMQ) PublishStatus(ctx context.Context, status *models.StatusMessage) error {
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	return r.statusChannel.PublishWithContext(ctx,
		"",                   // exchange
		r.config.StatusQueue, // routing key
		false,                // mandatory
		false,                // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         data,
			DeliveryMode: amqp.Persistent,
		},
	)
}

func (r *RabbitMQ) Close() error {
	if err := r.jobChannel.Close(); err != nil {
		return err
	}
	if err := r.statusChannel.Close(); err != nil {
		return err
	}
	return r.conn.Close()
}
