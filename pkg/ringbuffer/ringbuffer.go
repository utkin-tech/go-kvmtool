package ringbuffer

import (
	"errors"
	"io"
	"sync"
)

type RingBuffer[T any] struct {
	buffer   []T
	head     int
	tail     int
	size     int
	cap      int
	mu       sync.RWMutex
	pushFunc func()
}

func NewRingBuffer[T any](capacity int, pushFunc func()) *RingBuffer[T] {
	if capacity <= 0 {
		panic("capacity must be positive")
	}

	return &RingBuffer[T]{
		buffer:   make([]T, capacity),
		head:     0,
		tail:     0,
		size:     0,
		cap:      capacity,
		pushFunc: pushFunc,
	}
}

func (rb *RingBuffer[T]) Push(item T) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buffer[rb.tail] = item
	rb.tail = (rb.tail + 1) % rb.cap

	if rb.size < rb.cap {
		rb.size++
	} else {
		rb.head = (rb.head + 1) % rb.cap
	}

	if rb.pushFunc != nil {
		go rb.pushFunc()
	}
}

func (rb *RingBuffer[T]) Pop() (T, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	var zero T
	if rb.size == 0 {
		return zero, errors.New("buffer is empty")
	}

	item := rb.buffer[rb.head]
	rb.buffer[rb.head] = zero
	rb.head = (rb.head + 1) % rb.cap
	rb.size--

	return item, nil
}

func (rb *RingBuffer[T]) TryPop() (T, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	var zero T
	if rb.size == 0 {
		return zero, false
	}

	item := rb.buffer[rb.head]
	rb.buffer[rb.head] = zero
	rb.head = (rb.head + 1) % rb.cap
	rb.size--

	return item, true
}

func (rb *RingBuffer[T]) Peek() (T, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	var zero T
	if rb.size == 0 {
		return zero, errors.New("buffer is empty")
	}

	return rb.buffer[rb.head], nil
}

func (rb *RingBuffer[T]) TryPeek() (T, bool) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	var zero T
	if rb.size == 0 {
		return zero, false
	}

	return rb.buffer[rb.head], true
}

func (rb *RingBuffer[T]) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}

func (rb *RingBuffer[T]) Capacity() int {
	return rb.cap
}

func (rb *RingBuffer[T]) IsEmpty() bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size == 0
}

func (rb *RingBuffer[T]) IsFull() bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size == rb.cap
}

func (rb *RingBuffer[T]) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	var zero T
	for i := range rb.buffer {
		rb.buffer[i] = zero
	}

	rb.head = 0
	rb.tail = 0
	rb.size = 0
}

var _ io.Writer = (*RingBuffer[byte])(nil)

func (rb *RingBuffer[byte]) Write(p []byte) (n int, err error) {
	for _, b := range p {
		rb.Push(b)
		n++
	}
	return n, nil
}

var _ io.Reader = (*RingBuffer[byte])(nil)

func (rb *RingBuffer[byte]) Read(p []byte) (n int, err error) {
	if rb.IsEmpty() {
		return 0, nil
	}

	for i := range p {
		b, ok := rb.TryPop()
		if !ok {
			if n == 0 {
				return 0, nil
			}
			return n, nil
		}
		p[i] = b
		n++
	}

	return n, nil
}
