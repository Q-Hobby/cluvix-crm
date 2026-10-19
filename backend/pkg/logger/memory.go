// MemoryLogger — test-only Logger implementation that captures log entries
// in-memory for assertions. Exported (non-_test.go) để test ở package khác
// import được.
//
// Usage:
//
//	mem := logger.NewMemoryLogger()
//	svc := NewMyService(deps.WithLogger(mem))
//	svc.DoThing()
//	require.Len(t, mem.Entries(), 1)
//	assert.Equal(t, "thing done", mem.Entries()[0].Msg)
//
// Thread-safe: lock cho concurrent goroutine emit log.

package logger

import (
	"sync"

	"go.uber.org/zap"
)

// Entry captures one log call.
type Entry struct {
	Level  string // "info" | "warn" | "error" | "fatal"
	Msg    string
	Fields []zap.Field
}

// MemoryLogger implements Logger by appending to an in-memory slice.
type MemoryLogger struct {
	mu      sync.Mutex
	entries []Entry
	parent  []zap.Field // accumulated via With()
}

// NewMemoryLogger constructs an empty MemoryLogger.
func NewMemoryLogger() *MemoryLogger { return &MemoryLogger{} }

// Entries returns a snapshot copy of captured entries (safe for assertions).
func (m *MemoryLogger) Entries() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Entry, len(m.entries))
	copy(out, m.entries)
	return out
}

// Reset clears captured entries (useful between sub-tests).
func (m *MemoryLogger) Reset() {
	m.mu.Lock()
	m.entries = nil
	m.mu.Unlock()
}

// HasMessage returns true if any entry's Msg equals msg.
func (m *MemoryLogger) HasMessage(msg string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if e.Msg == msg {
			return true
		}
	}
	return false
}

func (m *MemoryLogger) append(level, msg string, fields []zap.Field) {
	m.mu.Lock()
	defer m.mu.Unlock()
	all := make([]zap.Field, 0, len(m.parent)+len(fields))
	all = append(all, m.parent...)
	all = append(all, fields...)
	m.entries = append(m.entries, Entry{Level: level, Msg: msg, Fields: all})
}

func (m *MemoryLogger) Debug(msg string, fields ...zap.Field) { m.append("debug", msg, fields) }
func (m *MemoryLogger) Info(msg string, fields ...zap.Field)  { m.append("info", msg, fields) }
func (m *MemoryLogger) Warn(msg string, fields ...zap.Field)  { m.append("warn", msg, fields) }
func (m *MemoryLogger) Error(msg string, fields ...zap.Field) { m.append("error", msg, fields) }
func (m *MemoryLogger) Fatal(msg string, fields ...zap.Field) { m.append("fatal", msg, fields) }

func (m *MemoryLogger) With(fields ...zap.Field) Logger {
	parent := make([]zap.Field, 0, len(m.parent)+len(fields))
	parent = append(parent, m.parent...)
	parent = append(parent, fields...)
	// Share underlying entries slice via pointer to same MemoryLogger — chỉ tách parent.
	return &memoryChild{root: m, parent: parent}
}

// memoryChild routes appends back vào root MemoryLogger, kế thừa parent fields.
type memoryChild struct {
	root   *MemoryLogger
	parent []zap.Field
}

func (c *memoryChild) emit(level, msg string, fields []zap.Field) {
	c.root.mu.Lock()
	defer c.root.mu.Unlock()
	all := make([]zap.Field, 0, len(c.parent)+len(fields))
	all = append(all, c.parent...)
	all = append(all, fields...)
	c.root.entries = append(c.root.entries, Entry{Level: level, Msg: msg, Fields: all})
}

func (c *memoryChild) Debug(msg string, fields ...zap.Field) { c.emit("debug", msg, fields) }
func (c *memoryChild) Info(msg string, fields ...zap.Field)  { c.emit("info", msg, fields) }
func (c *memoryChild) Warn(msg string, fields ...zap.Field)  { c.emit("warn", msg, fields) }
func (c *memoryChild) Error(msg string, fields ...zap.Field) { c.emit("error", msg, fields) }
func (c *memoryChild) Fatal(msg string, fields ...zap.Field) { c.emit("fatal", msg, fields) }

func (c *memoryChild) With(fields ...zap.Field) Logger {
	merged := make([]zap.Field, 0, len(c.parent)+len(fields))
	merged = append(merged, c.parent...)
	merged = append(merged, fields...)
	return &memoryChild{root: c.root, parent: merged}
}
