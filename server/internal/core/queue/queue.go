// Package queue provides a RabbitMQ-backed job queue and worker using amqp091-go.
//
// Feature flags in config.yaml:
//
//	queue.rabbitmq.enabled: false            → no AMQP connection is opened
//	workers.process.enabled: false           → consumer goroutines never start
//	workers.notification_jobs.enabled: false → email jobs are skipped
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/your-username/go-mux-backend-template/server/pkg"
)

const EmailQueue = "email_jobs"

// WelcomeEmailJob is the payload published when a new user registers.
type WelcomeEmailJob struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// Manager owns the AMQP connection and channel.
// Create one via New() and call Close() on shutdown.
type Manager struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	cfg     ManagerConfig
	logger  *pkg.Logger
}

// ManagerConfig holds the parameters needed to connect and configure the queue.
type ManagerConfig struct {
	URL             string
	DefaultAttempts int
	DefaultBackoff  time.Duration
}

// New dials RabbitMQ, opens a channel, declares all queues, and returns a Manager.
func New(cfg ManagerConfig, logger *pkg.Logger) (*Manager, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("[QUEUE] failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("[QUEUE] failed to open channel: %w", err)
	}

	m := &Manager{conn: conn, channel: ch, cfg: cfg, logger: logger}

	if err := m.declareQueues(); err != nil {
		m.Close()
		return nil, err
	}

	logger.Info("[QUEUE] RabbitMQ connected and queues declared")
	return m, nil
}

// declareQueues ensures all required queues exist on the broker.
// Durable queues survive broker restarts.
func (m *Manager) declareQueues() error {
	for _, name := range []string{EmailQueue} {
		if _, err := m.channel.QueueDeclare(name, true, false, false, false, nil); err != nil {
			return fmt.Errorf("[QUEUE] failed to declare queue %q: %w", name, err)
		}
	}
	return nil
}

// Publish serialises payload as JSON and publishes it to queueName.
// Messages are marked persistent so they survive a broker restart.
func (m *Manager) Publish(ctx context.Context, queueName string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("[QUEUE] failed to marshal payload: %w", err)
	}

	return m.channel.PublishWithContext(ctx, "", queueName, false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

// ConsumeWelcomeEmails starts concurrency worker goroutines that process
// WelcomeEmailJob messages from EmailQueue. handler is called for each message;
// on success the message is acked, on error it is nacked and requeued.
// Goroutines run until ctx is cancelled or the channel is closed.
func (m *Manager) ConsumeWelcomeEmails(ctx context.Context, concurrency int, handler func(WelcomeEmailJob) error) error {
	msgs, err := m.channel.Consume(EmailQueue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("[QUEUE] failed to register consumer: %w", err)
	}

	m.logger.Info("[QUEUE] Email job consumer started", "concurrency", concurrency, "queue", EmailQueue)

	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			for {
				select {
				case <-ctx.Done():
					m.logger.Info("[QUEUE] Worker shutting down", "worker_id", workerID)
					return
				case msg, ok := <-msgs:
					if !ok {
						m.logger.Warn("[QUEUE] Delivery channel closed", "worker_id", workerID)
						return
					}
					m.processWithRetry(msg, handler, workerID)
				}
			}
		}(i)
	}

	return nil
}

func (m *Manager) processWithRetry(msg amqp.Delivery, handler func(WelcomeEmailJob) error, workerID int) {
	var job WelcomeEmailJob
	if err := json.Unmarshal(msg.Body, &job); err != nil {
		m.logger.Error("[QUEUE] Failed to unmarshal job", "worker_id", workerID, "error", err)
		msg.Nack(false, false)
		return
	}

	m.logger.Info("[QUEUE] Processing email job", "worker_id", workerID, "user_id", job.UserID, "email", job.Email)

	if err := handler(job); err != nil {
		m.logger.Error("[QUEUE] Job failed", "worker_id", workerID, "error", err)
		msg.Nack(false, true)
		return
	}

	msg.Ack(false)
	m.logger.Info("[QUEUE] Job completed", "worker_id", workerID, "user_id", job.UserID)
}

// Close gracefully closes the channel and connection.
func (m *Manager) Close() {
	if m.channel != nil {
		m.channel.Close()
	}
	if m.conn != nil {
		m.conn.Close()
	}
	m.logger.Info("[QUEUE] RabbitMQ connection closed")
}
