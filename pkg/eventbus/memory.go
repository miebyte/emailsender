package eventbus

import (
	"context"
	"sync"

	"github.com/miebyte/emailsender/domain/event"
)

type memoryEventBus struct {
	handlerFns map[string][]event.Handler
	mu         sync.RWMutex
}

func NewMemoryEventBus() event.EventBus {
	return &memoryEventBus{
		handlerFns: make(map[string][]event.Handler),
	}
}

func (b *memoryEventBus) Publish(ctx context.Context, event event.Event) error {
	b.mu.RLock()
	handlers := b.handlerFns[event.EventName()]
	b.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (b *memoryEventBus) Subscribe(eventName string, handler event.Handler) {
	b.mu.Lock()
	b.handlerFns[eventName] = append(b.handlerFns[eventName], handler)
	b.mu.Unlock()
}
