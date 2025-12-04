package event

import "sync"

type ObservableState[T any] struct {
	value       T
	subscribers []chan T
	mu          sync.RWMutex
}

func NewObservableState[T any](initial T) *ObservableState[T] {
	return &ObservableState[T]{
		value:       initial,
		subscribers: make([]chan T, 0),
	}
}

func (os *ObservableState[T]) Get() T {
	os.mu.RLock()
	defer os.mu.RUnlock()
	return os.value
}

func (os *ObservableState[T]) Set(value T) {
	os.mu.Lock()
	os.value = value
	subscribers := os.subscribers
	os.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- value:
		default:
			// Subscriber can't process value, drop value
		}
	}
}

func (os *ObservableState[T]) Subscribe() chan T {
	os.mu.Lock()
	defer os.mu.Unlock()

	ch := make(chan T, 10)
	os.subscribers = append(os.subscribers, ch)

	ch <- os.value

	return ch
}
