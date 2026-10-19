package export

import "testing"

// Khoá lại rằng file export dịch vụ LUÔN có dòng field key và luôn khớp độ dài với dòng nhãn.
// Bỏ FieldKeys đi thì round-trip (export → sửa → import) hỏng mà build vẫn xanh.
func TestServiceExportConfigCarriesFieldKeys(t *testing.T) {
	cfg := serviceExportConfig(nil)
	if len(cfg.FieldKeys) == 0 {
		t.Fatal("Config export dịch vụ không có FieldKeys — file sẽ mất dòng field key")
	}
	if len(cfg.FieldKeys) != len(cfg.Headers) {
		t.Fatalf("FieldKeys (%d) lệch Headers (%d)", len(cfg.FieldKeys), len(cfg.Headers))
	}
	if len(cfg.Headers) != len(serviceCols) {
		t.Fatalf("Headers (%d) lệch serviceCols (%d)", len(cfg.Headers), len(serviceCols))
	}
	for i, col := range serviceCols {
		if cfg.Headers[i] != col.header || cfg.FieldKeys[i] != col.key {
			t.Errorf("cột %d lệch: header=%q key=%q, muốn %q/%q", i, cfg.Headers[i], cfg.FieldKeys[i], col.header, col.key)
		}
	}
}

// Cột tiền nullable phải giữ NULL, không ép về 0 — nếu không, chỉ cần export rồi import lại là
// mọi giá/hoa hồng đang NULL biến thành 0 thật trong DB.
func TestServiceNullableMoneyColsKeepNull(t *testing.T) {
	empty := map[string]interface{}{}
	for _, key := range []string{
		"min_price", "max_price",
		"commission_telesale", "commission_telesale_new",
	} {
		found := false
		for _, col := range serviceCols {
			if col.key != key {
				continue
			}
			found = true
			if got := col.extract(empty); got != nil {
				t.Errorf("cột %q với giá trị NULL export ra %v, muốn nil", key, got)
			}
		}
		if !found {
			t.Errorf("không tìm thấy cột %q trong serviceCols", key)
		}
	}
}
