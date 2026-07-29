package support

import (
	"sync"
	"time"
)

type Event struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id,omitempty"`
	UserID         int64  `json:"user_id,omitempty"`
	CreatedAt      int64  `json:"created_at"`
}

type eventBroker struct {
	mu          sync.RWMutex
	nextID      uint64
	subscribers map[uint64]chan Event
}

func newEventBroker() *eventBroker {
	return &eventBroker{subscribers: make(map[uint64]chan Event)}
}

func (b *eventBroker) subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	channel := make(chan Event, 32)
	b.subscribers[id] = channel
	b.mu.Unlock()
	return channel, func() {
		b.mu.Lock()
		if current, exists := b.subscribers[id]; exists {
			delete(b.subscribers, id)
			close(current)
		}
		b.mu.Unlock()
	}
}

func (b *eventBroker) publish(event Event) {
	if event.CreatedAt == 0 {
		event.CreatedAt = time.Now().Unix()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, subscriber := range b.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}
