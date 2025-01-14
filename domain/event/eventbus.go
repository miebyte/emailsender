package event

import "context"

type Event interface {
	EventName() string
}

type Handler func(ctx context.Context, event Event) error

type EventBus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(eventName string, handler Handler)
}
