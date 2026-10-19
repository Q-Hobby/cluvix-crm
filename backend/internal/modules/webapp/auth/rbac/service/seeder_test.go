package service

import "testing"

// Unit test guard logic prune (thuần, không DB) — cover AC5 (safety-guard) + set-diff.
func TestPlanPrune(t *testing.T) {
	existing := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} // 10 quyền

	// Bình thường: desired a..h → prune i,j (2/10 = 20% < 30%) → OK.
	tp, err := planPrune(existing, []string{"a", "b", "c", "d", "e", "f", "g", "h"})
	if err != nil {
		t.Fatalf("bình thường không được lỗi: %v", err)
	}
	if len(tp) != 2 {
		t.Fatalf("kỳ vọng prune 2, được %d (%v)", len(tp), tp)
	}

	// AC5: desired rỗng → error (chống artifact rỗng).
	if _, err := planPrune(existing, nil); err == nil {
		t.Fatal("desired rỗng phải trả lỗi ABORT")
	}

	// AC5: ratio guard — desired chỉ 1 → prune 9/10 = 90% > 30% → error.
	if _, err := planPrune(existing, []string{"a"}); err == nil {
		t.Fatal("prune > 30% phải trả lỗi guard")
	}

	// Không có gì để prune (desired ⊇ existing) → (nil, nil), idempotent.
	tp2, err := planPrune(existing, []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"})
	if err != nil || tp2 != nil {
		t.Fatalf("không prune phải (nil,nil), được (%v,%v)", tp2, err)
	}

	// Biên 30%: existing 10 → maxPrune = ceil(3.0) = 3. Prune đúng 3 (h,i,j) → OK.
	tp3, err := planPrune(existing, []string{"a", "b", "c", "d", "e", "f", "g"})
	if err != nil {
		t.Fatalf("biên 30%% (prune 3/10) phải pass: %v", err)
	}
	if len(tp3) != 3 {
		t.Fatalf("kỳ vọng prune 3, được %d", len(tp3))
	}

	// Vượt biên: prune 4/10 = 40% > 30% → error.
	if _, err := planPrune(existing, []string{"a", "b", "c", "d", "e", "f"}); err == nil {
		t.Fatal("prune 4/10 (40%) phải trả lỗi guard")
	}
}
