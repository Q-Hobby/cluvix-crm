package helpers

import (
	"encoding/json"
	"testing"
)

func TestParseJSONFields_InvalidJSONLookalikeStaysString(t *testing.T) {
	bad := "[11, 21, 15, 17,  46, 47, , 14]"
	row := map[string]interface{}{
		"note":  bad,
		"ids":   "[1,2,3]",
		"obj":   `{"a":1}`,
		"plain": "hello",
	}
	ParseJSONFields([]map[string]interface{}{row})

	if s, ok := row["note"].(string); !ok || s != bad {
		t.Fatalf("note phải giữ nguyên string, got %T %v", row["note"], row["note"])
	}
	if _, ok := row["ids"].(json.RawMessage); !ok {
		t.Fatalf("ids hợp lệ phải thành RawMessage, got %T", row["ids"])
	}
	if _, ok := row["obj"].(json.RawMessage); !ok {
		t.Fatalf("obj hợp lệ phải thành RawMessage, got %T", row["obj"])
	}

	// Chốt triệu chứng gốc: response chứa row này phải marshal được (trước đây lỗi → Gin 200 body rỗng).
	b, err := json.Marshal(map[string]interface{}{"items": []interface{}{row}})
	if err != nil || len(b) == 0 {
		t.Fatalf("marshal phải thành công, err=%v", err)
	}
}
