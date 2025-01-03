package ncl

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/bacalhau-project/bacalhau/pkg/lib/validate"
)

// subscriber handles message consumption
type subscriber struct {
	nc      *nats.Conn
	config  SubscriberConfig
	encoder *encoder

	subscriptions       []*nats.Subscription
	consecutiveFailures int
	mu                  sync.Mutex
}

// NewSubscriber creates a new subscriber with the given options
func NewSubscriber(nc *nats.Conn, config SubscriberConfig) (Subscriber, error) {
	config.setDefaults()

	enc, err := newEncoder(encoderConfig{
		source:            config.Name,
		messageSerializer: config.MessageSerializer,
		messageRegistry:   config.MessageRegistry,
	})
	if err != nil {
		return nil, err
	}

	s := &subscriber{
		nc:      nc,
		config:  config,
		encoder: enc,
	}

	// Validate the subscriber
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("error validating subscriber: %w", err)
	}

	return s, nil
}

// validate checks if the subscriber is properly configured
func (s *subscriber) validate() error {
	return errors.Join(
		validate.NotNil(s.nc, "NATS connection cannot be nil"),
		s.config.Validate(),
	)
}

func (s *subscriber) Subscribe(ctx context.Context, subjects ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, subject := range subjects {
		sub, err := s.nc.Subscribe(subject, s.handleNatsMessage)
		if err != nil {
			return err
		}
		s.subscriptions = append(s.subscriptions, sub)
	}
	return nil
}

// messageHandler is the callback function for message processing
func (s *subscriber) handleNatsMessage(m *nats.Msg) {
	if err := s.processMessage(m); err != nil {
		log.Error().Err(err).Str("handler", s.config.Name).Msg("failed to process message")

		s.consecutiveFailures += 1
		delay := s.config.Backoff.BackoffDuration(s.consecutiveFailures)
		if nackErr := NackWithDelay(m, err, delay); nackErr != nil {
			log.Debug().Err(nackErr).Msg("failed to nack message")
		}
		return
	}

	s.consecutiveFailures = 0
	if ackErr := Ack(m); ackErr != nil {
		log.Warn().Err(ackErr).Msg("failed to ack message")
	}
}

func (s *subscriber) processMessage(m *nats.Msg) error {
	// TODO: interrupt processing if subscriber is closed
	ctx, cancel := context.WithTimeout(context.Background(), s.config.ProcessingTimeout)
	defer cancel()

	// Deserialize message envelope
	// Apply filter
	if s.config.MessageFilter.ShouldFilter(m.Header) {
		return nil
	}

	// Deserialize payload
	message, err := s.encoder.decode(m.Data)
	if err != nil {
		return fmt.Errorf("failed to deserialize message payload: %w", err)
	}

	// Process with handler
	if s.config.MessageHandler.ShouldProcess(ctx, message) {
		if err = s.config.MessageHandler.HandleMessage(ctx, message); err != nil {
			return fmt.Errorf("failed to handle message: %w", err)
		}
	}

	// Notify successful processing
	s.config.ProcessingNotifier.OnProcessed(ctx, message)

	return nil
}

func (s *subscriber) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	for _, sub := range s.subscriptions {
		if sub.IsValid() {
			err = errors.Join(err, sub.Unsubscribe())
		}
	}
	if err != nil {
		return fmt.Errorf("error closing subscriptions: %w", err)
	}
	return nil
}

// compile-time interface assertions
var _ Subscriber = (*subscriber)(nil)
