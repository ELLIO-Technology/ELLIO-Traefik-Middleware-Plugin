package logs

import (
	"sync"
)

// RingBuffer is a circular buffer for storing events
type RingBuffer struct {
	buffer   []*BlockEvent
	capacity int
	head     int
	tail     int
	size     int
	mu       sync.Mutex
}

// NewRingBuffer creates a new ring buffer
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		buffer:   make([]*BlockEvent, capacity),
		capacity: capacity,
	}
}

// Add adds an event to the buffer
func (rb *RingBuffer) Add(event *BlockEvent) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size >= rb.capacity {
		// Buffer is full, overwrite oldest
		rb.buffer[rb.tail] = event
		rb.tail = (rb.tail + 1) % rb.capacity
		rb.head = (rb.head + 1) % rb.capacity
		return true
	}

	rb.buffer[rb.tail] = event
	rb.tail = (rb.tail + 1) % rb.capacity
	rb.size++
	return true
}

// Drain removes up to n events from the buffer
func (rb *RingBuffer) Drain(n int) []*BlockEvent {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return nil
	}

	count := minInt(n, rb.size)
	events := make([]*BlockEvent, count)

	for i := 0; i < count; i++ {
		events[i] = rb.buffer[rb.head]
		rb.buffer[rb.head] = nil // Clear reference
		rb.head = (rb.head + 1) % rb.capacity
		rb.size--
	}

	return events
}

// DrainAll removes all events from the buffer
func (rb *RingBuffer) DrainAll() []*BlockEvent {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return nil
	}

	events := make([]*BlockEvent, rb.size)

	for i := 0; i < len(events); i++ {
		events[i] = rb.buffer[rb.head]
		rb.buffer[rb.head] = nil
		rb.head = (rb.head + 1) % rb.capacity
	}

	rb.size = 0
	return events
}

// Size returns the current number of events in the buffer
func (rb *RingBuffer) Size() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.size
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
