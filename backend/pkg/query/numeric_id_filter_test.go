package query

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

// Giá trị trong các test dưới đây KHÔNG phải bịa: chúng là đúng tập đã đo trên bench 545.547 dòng
// của 1 bảng con khách hàng thật (2026-09-08, bảng gốc đã xoá ở story-13 crm-standalone) — mỗi giá
// trị từng trả về 5.000 bản ghi của khách 888002. Bảng dùng ở test dưới đây (`order_service_used`)
// chỉ là ví dụ trung tính cho generic SQL builder.
func TestApplyFilter_CotIDSo_GiaTriRac_TraVeRong(t *testing.T) {
	racs := []string{"888002abc", " 888002", "888002.9", "abc", "-1", "1e3", "0x10", "888002 OR 1=1"}
	for _, rac := range racs {
		t.Run(rac, func(t *testing.T) {
			db := newDryRunDB(t)
			c := newTestContext("filter[customer_id]=" + strings.ReplaceAll(rac, " ", "%20"))

			sql, _ := buildSQL(t, db, "order_service_used", func(q *gorm.DB) *gorm.DB {
				return ApplyFilter(c, q, "customer_id", "order_service_used.customer_id")
			})

			if !strings.Contains(sql, "1 = 0") {
				t.Fatalf("giá trị rác %q phải cho ra điều kiện chặn (1 = 0), got: %s", rac, sql)
			}
			if strings.Contains(sql, "order_service_used.customer_id = ?") {
				t.Fatalf("giá trị rác %q KHÔNG được đi vào so sánh cột (MySQL sẽ ép kiểu), got: %s", rac, sql)
			}
		})
	}
}

func TestApplyFilter_CotIDSo_GiaTriHopLe_GiuNguyen(t *testing.T) {
	db := newDryRunDB(t)
	c := newTestContext("filter[customer_id]=888002")

	sql, vars := buildSQL(t, db, "order_service_used", func(q *gorm.DB) *gorm.DB {
		return ApplyFilter(c, q, "customer_id", "order_service_used.customer_id")
	})

	if !strings.Contains(sql, "order_service_used.customer_id = ?") {
		t.Fatalf("giá trị hợp lệ phải giữ nguyên so sánh cột, got: %s", sql)
	}
	if strings.Contains(sql, "1 = 0") {
		t.Fatalf("giá trị hợp lệ KHÔNG được bị chặn, got: %s", sql)
	}
	if len(vars) != 1 || vars[0] != "888002" {
		t.Fatalf("bind args mong đợi [888002], got: %v", vars)
	}
}

// Cột *_id kiểu varchar (template_id, page_id, client_id…) KHÔNG nằm trong allowlist ⇒ phải giữ
// NGUYÊN hành vi cũ. Đây là lý do dùng allowlist thay vì quy tắc "tên kết thúc _id".
func TestApplyFilter_CotIDVarchar_KhongBiChan(t *testing.T) {
	for _, col := range []string{"template_id", "page_id", "client_id", "msg_id", "icd_id"} {
		t.Run(col, func(t *testing.T) {
			db := newDryRunDB(t)
			c := newTestContext("filter[" + col + "]=abc123")

			sql, vars := buildSQL(t, db, "zalo_zns_log", func(q *gorm.DB) *gorm.DB {
				return ApplyFilter(c, q, col, col)
			})

			if strings.Contains(sql, "1 = 0") {
				t.Fatalf("cột varchar %q KHÔNG được bị chặn — giá trị chữ là hợp lệ, got: %s", col, sql)
			}
			if len(vars) != 1 || vars[0] != "abc123" {
				t.Fatalf("cột varchar %q phải giữ nguyên giá trị, got: %v", col, vars)
			}
		})
	}
}

// service_type_id CỐ Ý ngoài allowlist: int ở `service`, json ở `treatment_course_type`.
func TestApplyFilter_ServiceTypeID_CoYNgoaiAllowlist(t *testing.T) {
	db := newDryRunDB(t)
	c := newTestContext("filter[service_type_id]=abc")

	sql, _ := buildSQL(t, db, "treatment_course_type", func(q *gorm.DB) *gorm.DB {
		return ApplyFilter(c, q, "service_type_id", "service_type_id")
	})

	if strings.Contains(sql, "1 = 0") {
		t.Fatalf("service_type_id phải giữ hành vi cũ (có biến thể JSON), got: %s", sql)
	}
}

// neq đi hướng ngược: "khác một id không hợp lệ" đúng với mọi dòng ⇒ bỏ mệnh đề, KHÔNG trả rỗng.
func TestApplyFilter_CotIDSo_Neq_GiaTriRac_BoMenhDe(t *testing.T) {
	db := newDryRunDB(t)
	c := newTestContext("filter[customer_id][neq]=888002abc")

	sql, _ := buildSQL(t, db, "order_service_used", func(q *gorm.DB) *gorm.DB {
		return ApplyFilter(c, q, "customer_id", "order_service_used.customer_id")
	})

	if strings.Contains(sql, "1 = 0") {
		t.Fatalf("neq với giá trị rác phải khớp MỌI dòng, không được trả rỗng, got: %s", sql)
	}
	if strings.Contains(sql, "customer_id != ?") {
		t.Fatalf("neq với giá trị rác không được đẩy giá trị vào so sánh, got: %s", sql)
	}
}

// IN: lọc bỏ phần tử rác, giữ phần tử hợp lệ; rác HẾT ⇒ trả rỗng.
func TestApplyFilter_CotIDSo_In_LocPhanTuRac(t *testing.T) {
	db := newDryRunDB(t)
	c := newTestContext("filter[customer_id][in][0]=101&filter[customer_id][in][1]=102abc&filter[customer_id][in][2]=103")

	sql, vars := buildSQL(t, db, "order_service_used", func(q *gorm.DB) *gorm.DB {
		return ApplyFilter(c, q, "customer_id", "order_service_used.customer_id")
	})

	if strings.Contains(sql, "1 = 0") {
		t.Fatalf("còn phần tử hợp lệ thì không được trả rỗng, got: %s", sql)
	}
	// GORM trải slice của IN thành từng bind arg riêng.
	if len(vars) != 2 || vars[0] != "101" || vars[1] != "103" {
		t.Fatalf("phải giữ đúng [101 103] và LOẠI '102abc', got: %v", vars)
	}
	if !strings.Contains(sql, "IN (?,?)") {
		t.Fatalf("mong đợi IN 2 phần tử, got: %s", sql)
	}
}

func TestApplyFilter_CotIDSo_In_RacHet_TraVeRong(t *testing.T) {
	db := newDryRunDB(t)
	c := newTestContext("filter[customer_id][in][0]=abc&filter[customer_id][in][1]=1x")

	sql, _ := buildSQL(t, db, "order_service_used", func(q *gorm.DB) *gorm.DB {
		return ApplyFilter(c, q, "customer_id", "order_service_used.customer_id")
	})

	if !strings.Contains(sql, "1 = 0") {
		t.Fatalf("rác hết phải trả rỗng, got: %s", sql)
	}
}

// Cột có alias/tên bảng đứng trước — phải nhận diện theo đoạn sau dấu chấm CUỐI.
func TestIsNumericIDColumn_NhanDienQuaAlias(t *testing.T) {
	cases := map[string]bool{
		"customer_id":              true,
		"appointment.customer_id":  true,
		"osu.customer_id":          true,
		"template_id":              false,
		"zalo_zns_log.template_id": false,
		"service_type_id":          false,
		"fullname":                 false,
	}
	for col, want := range cases {
		if got := isNumericIDColumn(col); got != want {
			t.Errorf("isNumericIDColumn(%q) = %v, want %v", col, got, want)
		}
	}
}

// gt/gte/lt/lte cũng ép kiểu như eq.
func TestApplyFilter_CotIDSo_SoSanh_GiaTriRac_TraVeRong(t *testing.T) {
	for _, op := range []string{"gt", "gte", "lt", "lte"} {
		t.Run(op, func(t *testing.T) {
			db := newDryRunDB(t)
			c := newTestContext("filter[customer_id][" + op + "]=5abc")

			sql, _ := buildSQL(t, db, "order_service_used", func(q *gorm.DB) *gorm.DB {
				return ApplyFilter(c, q, "customer_id", "order_service_used.customer_id")
			})

			if !strings.Contains(sql, "1 = 0") {
				t.Fatalf("%s với giá trị rác phải trả rỗng, got: %s", op, sql)
			}
		})
	}
}
