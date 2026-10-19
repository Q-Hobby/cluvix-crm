package logger

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Compile-time check: MemoryLogger và zapLogger đều satisfy Logger.
var (
	_ Logger = (*MemoryLogger)(nil)
	_ Logger = (*zapLogger)(nil)
	_ Logger = (*memoryChild)(nil)
)

func TestMemoryLogger_CapturesEntries(t *testing.T) {
	m := NewMemoryLogger()
	m.Info("hello", zap.String("k", "v"))
	m.Warn("careful")
	m.Error("boom", zap.Int("code", 500))

	entries := m.Entries()
	require.Len(t, entries, 3)
	assert.Equal(t, "info", entries[0].Level)
	assert.Equal(t, "hello", entries[0].Msg)
	assert.Equal(t, "warn", entries[1].Level)
	assert.Equal(t, "error", entries[2].Level)
	assert.Equal(t, 500, int(entries[2].Fields[0].Integer))
}

func TestMemoryLogger_WithInheritsFields(t *testing.T) {
	m := NewMemoryLogger()
	scoped := m.With(zap.String("request_id", "abc"))
	scoped.Info("ping")

	entries := m.Entries()
	require.Len(t, entries, 1)
	require.Len(t, entries[0].Fields, 1)
	assert.Equal(t, "request_id", entries[0].Fields[0].Key)
	assert.Equal(t, "abc", entries[0].Fields[0].String)
}

func TestMemoryLogger_WithNested(t *testing.T) {
	m := NewMemoryLogger()
	a := m.With(zap.String("layer", "service"))
	b := a.With(zap.String("op", "create"))
	b.Warn("retry")

	entries := m.Entries()
	require.Len(t, entries, 1)
	require.Len(t, entries[0].Fields, 2)
	assert.Equal(t, "layer", entries[0].Fields[0].Key)
	assert.Equal(t, "op", entries[0].Fields[1].Key)
}

func TestMemoryLogger_HasMessage(t *testing.T) {
	m := NewMemoryLogger()
	m.Info("one")
	m.Error("two")
	assert.True(t, m.HasMessage("one"))
	assert.True(t, m.HasMessage("two"))
	assert.False(t, m.HasMessage("missing"))
}

func TestMemoryLogger_Reset(t *testing.T) {
	m := NewMemoryLogger()
	m.Info("a")
	m.Reset()
	m.Info("b")
	entries := m.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "b", entries[0].Msg)
}

func TestMemoryLogger_ConcurrentSafe(t *testing.T) {
	m := NewMemoryLogger()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.Info("msg", zap.Int("i", i))
		}(i)
	}
	wg.Wait()
	assert.Len(t, m.Entries(), 50)
}

func TestDefault_NilSafe(t *testing.T) {
	// Khi L chưa Init() (test env), Default() trả wrapper no-op — không panic.
	saved := L
	L = nil
	defer func() { L = saved }()

	log := Default()
	require.NotNil(t, log)
	log.Info("should not panic")
	log.With(zap.String("k", "v")).Error("nope")
}
