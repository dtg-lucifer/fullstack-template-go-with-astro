// Package events provides a typed, in-process domain event bus.
// Services emit events; listeners react to them. Neither side knows about the other,
// which keeps business logic decoupled from side-effects like queuing or broadcasting.
package events

import "sync"

// ── Bus ───────────────────────────────────────────────────────────────────────

// Bus is a goroutine-safe in-process pub/sub bus.
// Listeners are called synchronously in the goroutine that calls Emit.
// If a listener needs to do slow work it should spawn its own goroutine.
type Bus struct {
	mu        sync.RWMutex
	listeners map[string][]func(any)
}

// New creates a ready-to-use Bus.
func New() *Bus {
	return &Bus{listeners: make(map[string][]func(any))}
}

func (b *Bus) on(name string, fn func(any)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners[name] = append(b.listeners[name], fn)
}

func (b *Bus) emit(name string, payload any) {
	b.mu.RLock()
	fns := b.listeners[name]
	b.mu.RUnlock()
	for _, fn := range fns {
		fn(payload)
	}
}
