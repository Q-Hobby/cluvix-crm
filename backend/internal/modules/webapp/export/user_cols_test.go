package export

import (
	"strings"
	"testing"
)

// Mọi cột phải khai đủ key + header; uiKey rỗng là hợp lệ (cột không có trên bảng UserList).
func TestUserColsWellFormed(t *testing.T) {
	seenKey := map[string]bool{}
	seenUI := map[string]bool{}
	for i, col := range userCols {
		if col.key == "" || col.header == "" || col.extract == nil {
			t.Errorf("cột %d thiếu key/header/extract: %+v", i, col.key)
		}
		if seenKey[col.key] {
			t.Errorf("key %q khai trùng — dòng field key của file sẽ có 2 cột cùng tên, importer từ chối file", col.key)
		}
		seenKey[col.key] = true
		if col.uiKey != "" {
			if seenUI[col.uiKey] {
				t.Errorf("uiKey %q khai trùng — lọc ?columns= sẽ ra 2 cột cho 1 ô tích trên FE", col.uiKey)
			}
			seenUI[col.uiKey] = true
		}
	}
	if len(UserExportHeaders()) != len(userCols) || len(UserExportFieldKeys()) != len(userCols) {
		t.Fatal("Headers/FieldKeys phải cùng độ dài với userCols")
	}
}

func keysOf(cols []userCol) string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.key
	}
	return strings.Join(out, ",")
}

func TestFilterUserExportCols(t *testing.T) {
	t.Run("không có columns → export toàn bộ", func(t *testing.T) {
		gotCols, _ := filterUserExportCols("", "id")
		if got := len(gotCols); got != len(userCols) {
			t.Errorf("%d cột, muốn %d", got, len(userCols))
		}
	})

	t.Run("lọc theo uiKey, giữ đúng thứ tự file mẫu", func(t *testing.T) {
		// FE gửi lộn xộn theo thứ tự user tích; file export vẫn phải theo thứ tự cột chuẩn.
		// Story-38 (D23): cột thương hiệu đã bỏ khỏi export — uiKey cũ FE còn gửi giờ không khớp
		// cột nào, filterUserExportCols bỏ qua (chỉ còn id,code,fullname).
		cols, _ := filterUserExportCols("legacy_brand,name,code", "id")
		got := keysOf(cols)
		if got != "id,code,fullname" {
			t.Errorf("cột = %q, muốn \"id,code,fullname\"", got)
		}
	})

	t.Run("id LUÔN có mặt dù FE không gửi — thiếu id thì import ngược thành tạo mới hàng loạt", func(t *testing.T) {
		cols, _ := filterUserExportCols("name", "id")
		got := keysOf(cols)
		if got != "id,fullname" {
			t.Errorf("cột = %q, muốn \"id,fullname\"", got)
		}
	})

	t.Run("cột chỉ để xem trên bảng (avatar, ngày tạo) không có bản export → bỏ qua", func(t *testing.T) {
		cols, unknown := filterUserExportCols("avatar,created_at,updated_by,name", "id")
		if len(unknown) != 0 {
			t.Errorf("cột display-only bị báo là key lạ: %v", unknown)
		}
		got := keysOf(cols)
		if got != "id,fullname" {
			t.Errorf("cột = %q, muốn \"id,fullname\"", got)
		}
	})

	t.Run("columns toàn cột lạ → rơi về export toàn bộ thay vì ra file trống", func(t *testing.T) {
		gotCols, _ := filterUserExportCols("avatar,created_at")
		if got := len(gotCols); got != len(userCols) {
			t.Errorf("%d cột, muốn %d", got, len(userCols))
		}
	})
}

// Hai tập uiKey phải RỜI NHAU và phủ hết những gì FE có thể gửi: cột export được và cột cố ý chỉ để
// xem. Key rơi ra ngoài cả hai = lệch hợp đồng FE↔BE, phải log chứ không im lặng.
func TestUserUIKeySetsAreDisjoint(t *testing.T) {
	for _, col := range userCols {
		if col.uiKey != "" && userDisplayOnlyUIKeys[col.uiKey] {
			t.Errorf("uiKey %q vừa export được vừa nằm trong danh sách display-only", col.uiKey)
		}
	}
}

func TestFilterUserExportColsReportsUnknownUIKey(t *testing.T) {
	// Mô phỏng FE đổi tên `description` → `role` mà BE chưa biết.
	_, unknown := filterUserExportCols("name,role", "id")
	if len(unknown) != 1 || unknown[0] != "role" {
		t.Errorf("unknown = %v, muốn [\"role\"]", unknown)
	}
	// Không gửi columns thì không có gì để báo.
	if _, u := filterUserExportCols("", "id"); u != nil {
		t.Errorf("unknown = %v, muốn nil", u)
	}
}
