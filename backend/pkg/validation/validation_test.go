package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pure unit tests cho validator helpers — no DB. Validators with DB
// (Unique, Exists) cần integration test.

func TestErrors_Required(t *testing.T) {
	cases := []struct {
		name  string
		value any
		fail  bool
	}{
		{"nil value", nil, true},
		{"empty string", "", true},
		{"zero int", 0, true},
		{"zero float", 0.0, true},
		{"non-empty string", "foo", false},
		{"non-zero int", 42, false},
		{"non-zero float", 1.5, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := New()
			v.Required(tc.value, "Field")
			assert.Equal(t, tc.fail, v.HasErrors())
		})
	}
}

func TestErrors_RequiredIf(t *testing.T) {
	t.Run("condition false → skip", func(t *testing.T) {
		v := New()
		v.RequiredIf(false, nil, "Field")
		assert.False(t, v.HasErrors())
	})
	t.Run("condition true + empty → fail", func(t *testing.T) {
		v := New()
		v.RequiredIf(true, "", "Field")
		assert.True(t, v.HasErrors())
	})
	t.Run("condition true + present → pass", func(t *testing.T) {
		v := New()
		v.RequiredIf(true, "value", "Field")
		assert.False(t, v.HasErrors())
	})
}

func TestErrors_Min(t *testing.T) {
	v := New()
	v.Min(5, 10, "Amount")
	require.True(t, v.HasErrors())
	assert.Contains(t, v.Error(), "Amount phải >= 10")

	v2 := New()
	v2.Min(15, 10, "Amount")
	assert.False(t, v2.HasErrors())
}

func TestErrors_Max(t *testing.T) {
	v := New()
	v.Max(20, 10, "Amount")
	require.True(t, v.HasErrors())
	assert.Contains(t, v.Error(), "Amount phải <= 10")

	v2 := New()
	v2.Max(5, 10, "Amount")
	assert.False(t, v2.HasErrors())
}

func TestErrors_Range(t *testing.T) {
	v := New()
	v.Range(5, 10, 20, "Percent")
	assert.True(t, v.HasErrors())

	v2 := New()
	v2.Range(15, 10, 20, "Percent")
	assert.False(t, v2.HasErrors())

	v3 := New()
	v3.Range(25, 10, 20, "Percent")
	assert.True(t, v3.HasErrors())
}

func TestErrors_MinLen_MaxLen(t *testing.T) {
	// MinLen
	v := New()
	v.MinLen("ab", 3, "Code")
	assert.True(t, v.HasErrors())

	v2 := New()
	v2.MinLen("abc", 3, "Code")
	assert.False(t, v2.HasErrors())

	// MinLen empty string → skip (Required handles empty)
	v3 := New()
	v3.MinLen("", 3, "Code")
	assert.False(t, v3.HasErrors())

	// MaxLen
	v4 := New()
	v4.MaxLen("abcdef", 3, "Code")
	assert.True(t, v4.HasErrors())

	// Unicode: "ăn" = 2 runes, not 3 bytes
	v5 := New()
	v5.MaxLen("ăn", 2, "Tên")
	assert.False(t, v5.HasErrors())
}

func TestErrors_In(t *testing.T) {
	v := New()
	v.In(3, []int{1, 2, 4}, "Status")
	assert.True(t, v.HasErrors())

	v2 := New()
	v2.In(2, []int{1, 2, 4}, "Status")
	assert.False(t, v2.HasErrors())

	// Non-slice "allowed" → silent skip
	v3 := New()
	v3.In(2, "not-a-slice", "Status")
	assert.False(t, v3.HasErrors())
}

func TestErrors_Match(t *testing.T) {
	v := New()
	v.Match("abc123", `^[a-z]+$`, "Code phải toàn chữ")
	assert.True(t, v.HasErrors())

	v2 := New()
	v2.Match("abc", `^[a-z]+$`, "Code phải toàn chữ")
	assert.False(t, v2.HasErrors())
}

func TestErrors_MultipleAccumulate(t *testing.T) {
	v := New()
	v.Required(nil, "F1")
	v.Min(5, 10, "F2")
	v.MaxLen("x", 0, "F3")

	require.True(t, v.HasErrors())
	msg := v.Error()
	assert.Contains(t, msg, "F1")
	assert.Contains(t, msg, "F2")
	assert.Contains(t, msg, "F3")
}
