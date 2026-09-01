// Package audit records promotion evaluations, decision trails, and system events
// from the Checkout process as structured (key-value) log entries, compatible with
// Go's log/slog library.
package audit

import (
	"context"
	"iter"
	"log/slog"
	"maps"
	"sync"
	"time"
)

// Event is the structured data model for a single audit log entry.
type Event struct {
	Timestamp  time.Time
	Level      slog.Level
	Message    string
	Attributes map[string]any
}

// Clone returns a deep, thread-safe copy of the Event (including its Attributes map).
func (e Event) Clone() Event {
	return Event{
		Timestamp:  e.Timestamp,
		Level:      e.Level,
		Message:    e.Message,
		Attributes: maps.Clone(e.Attributes),
	}
}

// Logger is the small interface that abstracts audit and logging operations.
//
// Go Pattern: Single-Method Interface & log/slog Compatibility.
type Logger interface {
	Log(ctx context.Context, level slog.Level, msg string, args ...any)
}

// Func adapts a plain function to the Logger interface.
type Func func(ctx context.Context, level slog.Level, msg string, args ...any)

// Log invokes the underlying function.
func (f Func) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	f(ctx, level, msg, args...)
}

// NoOp returns a safe no-operation logger — the zero-value Logger.
func NoOp() Logger {
	return Func(func(ctx context.Context, level slog.Level, msg string, args ...any) {})
}

// SlogLogger wraps Go's built-in log/slog.Logger to satisfy the Logger interface.
type SlogLogger struct {
	logger *slog.Logger
}

// NewSlogLogger wraps an slog.Logger. Falls back to slog.Default() if l is nil.
func NewSlogLogger(l *slog.Logger) *SlogLogger {
	if l == nil {
		l = slog.Default()
	}
	return &SlogLogger{logger: l}
}

// Log forwards the structured event to the underlying slog logger.
func (s *SlogLogger) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Log(ctx, level, msg, args...)
}

// InMemoryLogger is a thread-safe in-memory Logger used in tests and audit assertions.
type InMemoryLogger struct {
	mu     sync.RWMutex
	events []Event
}

// NewInMemoryLogger creates a new InMemoryLogger.
func NewInMemoryLogger() *InMemoryLogger {
	return &InMemoryLogger{
		events: make([]Event, 0),
	}
}

// Log appends the event to the in-memory list in a thread-safe manner.
func (m *InMemoryLogger) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	attrs := make(map[string]any)
	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		var val any
		if i+1 < len(args) {
			val = args[i+1]
		}
		attrs[key] = val
	}

	m.events = append(m.events, Event{
		Timestamp:  time.Now(),
		Level:      level,
		Message:    msg,
		Attributes: attrs,
	})
}

// Events returns a deep, thread-safe copy of all recorded events.
func (m *InMemoryLogger) Events() []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	copied := make([]Event, len(m.events))
	for i, e := range m.events {
		copied[i] = e.Clone()
	}
	return copied
}

// EventsSeq returns a Go 1.23+ iter.Seq that ranges over the recorded events.
func (m *InMemoryLogger) EventsSeq() iter.Seq[Event] {
	return func(yield func(Event) bool) {
		for _, e := range m.Events() {
			if !yield(e) {
				return
			}
		}
	}
}

// FindByLevel returns all events at the specified log level.
func (m *InMemoryLogger) FindByLevel(level slog.Level) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matching []Event
	for _, e := range m.events {
		if e.Level == level {
			matching = append(matching, e.Clone())
		}
	}
	return matching
}

// Count returns the total number of recorded events.
func (m *InMemoryLogger) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.events)
}

// ContainsMessage reports whether any recorded event has exactly the given message.
func (m *InMemoryLogger) ContainsMessage(substr string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, e := range m.events {
		if e.Message == substr {
			return true
		}
	}
	return false
}

// Clear removes all recorded events.
func (m *InMemoryLogger) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = m.events[:0]
}
