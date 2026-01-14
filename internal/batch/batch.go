package batch

import (
	"sync"
	"time"

	"websocket-demo/internal/types"
)

// MessageBatch handles batching of messages for performance optimization
type MessageBatch struct {
	Messages   []types.Message
	MaxSize    int
	FlushAfter time.Duration
	Timer      *time.Timer
	Mutex      sync.Mutex
	FlushFunc  func([]types.Message)
	done       chan struct{}
}

// NewMessageBatch creates a new message batch
func NewMessageBatch(maxSize int, flushAfter time.Duration, flushFunc func([]types.Message)) *MessageBatch {
	b := &MessageBatch{
		Messages:   make([]types.Message, 0, maxSize),
		MaxSize:    maxSize,
		FlushAfter: flushAfter,
		FlushFunc:  flushFunc,
		done:       make(chan struct{}),
	}
	b.Timer = time.NewTimer(flushAfter)
	go b.startTimer()
	return b
}

// Add adds a message to the batch
func (b *MessageBatch) Add(msg types.Message) {
	b.Mutex.Lock()
	defer b.Mutex.Unlock()

	b.Messages = append(b.Messages, msg)

	// Flush if batch is full
	if len(b.Messages) >= b.MaxSize {
		b.flush()
	} else {
		// Reset timer to debounce
		b.Timer.Reset(b.FlushAfter)
	}
}

// flush flushes the current batch
func (b *MessageBatch) flush() {
	if len(b.Messages) == 0 {
		return
	}

	// Copy messages to avoid race conditions
	messages := make([]types.Message, len(b.Messages))
	copy(messages, b.Messages)

	// Clear batch
	b.Messages = b.Messages[:0]

	// Call flush function in goroutine to avoid blocking
	go b.FlushFunc(messages)
}

// startTimer starts the flush timer
func (b *MessageBatch) startTimer() {
	for {
		select {
		case <-b.Timer.C:
			b.Mutex.Lock()
			b.flush()
			b.Mutex.Unlock()
		case <-b.done:
			return
		}
	}
}

// Stop stops the batch processor
func (b *MessageBatch) Stop() {
	b.Mutex.Lock()
	defer b.Mutex.Unlock()

	if b.Timer != nil {
		b.Timer.Stop()
	}

	// Signal goroutine to exit
	close(b.done)

	// Flush remaining messages
	if len(b.Messages) > 0 {
		b.flush()
	}
}

// Size returns the current batch size
func (b *MessageBatch) Size() int {
	b.Mutex.Lock()
	defer b.Mutex.Unlock()
	return len(b.Messages)
}
