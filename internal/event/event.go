package event

import (
	"context"

	"example.com/v2/internal/api"
	"github.com/rs/zerolog"
)

type OrderCreated struct {
	Order api.OrderResponse
}
type EventWorker interface {
	StartEventWorker(ctx context.Context)
}

type eventWorker struct {
	eventQueue chan OrderCreated
	logger     zerolog.Logger
}

func NewEventWorker(eventQueue chan OrderCreated, logger zerolog.Logger) EventWorker {
	return &eventWorker{
		eventQueue: eventQueue,
		logger:     logger,
	}
}

func (e *eventWorker) StartEventWorker(ctx context.Context) {
	logger := e.logger.With().Str("worker", "EventWorker").Logger()
	go func() {
		for {
			select {
			case e := <-e.eventQueue:
				// Simulate notification/email
				// Use structured logging or other logic here
				logger.Info().Msgf("Processing event: OrderCreated: %+v\n", e.Order)
			case <-ctx.Done():
				logger.Info().Msgf("Shutting down event worker")
				return
			}
		}
	}()
}
