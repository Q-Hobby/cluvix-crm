package pagination

import "testing"

// story-24 AC3 — 2 test thuần trên string input của joinedTables, không cần DB.

func TestJoinedTables_NestedJoinInsideSubquery_NotLeaked(t *testing.T) {
	clause := "LEFT JOIN (SELECT c.id FROM customer c JOIN branch b ON b.id = c.branch_id) s ON s.id = payment.customer_id"
	got := joinedTables(clause)
	for _, tbl := range got {
		if tbl == "branch" || tbl == "customer" {
			t.Fatalf("joinedTables không được nạp bảng lồng trong join-subquery, được: %v", got)
		}
	}
	if len(got) != 0 {
		t.Fatalf("joinedTables trên join-subquery không có bảng top-level nào phải trả rỗng, được: %v", got)
	}
}

func TestJoinedTables_JoinFollowedByNewlineTab_Detected(t *testing.T) {
	clause := "LEFT JOIN\n\tcustomer c ON c.id = payment.customer_id"
	got := joinedTables(clause)
	found := false
	for _, tbl := range got {
		if tbl == "customer" {
			found = true
		}
	}
	if !found {
		t.Fatalf("joinedTables phải bắt được bảng sau JOIN cách nhau bằng xuống dòng/tab, được: %v", got)
	}
}

func TestJoinedTables_MultipleTopLevelJoinsAfterSubquery_StillDetected(t *testing.T) {
	clause := "LEFT JOIN (SELECT c.id FROM customer c JOIN branch b ON b.id = c.branch_id) s ON s.id = payment.customer_id " +
		"LEFT JOIN appointment a ON a.customer_id = s.id"
	got := joinedTables(clause)
	found := false
	for _, tbl := range got {
		if tbl == "appointment" {
			found = true
		}
		if tbl == "branch" || tbl == "customer" {
			t.Fatalf("joinedTables không được nạp bảng lồng trong join-subquery, được: %v", got)
		}
	}
	if !found {
		t.Fatalf("joinedTables phải vẫn bắt được JOIN top-level SAU khi bỏ qua join-subquery, được: %v", got)
	}
}
