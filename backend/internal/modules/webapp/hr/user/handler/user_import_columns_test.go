package handler

import (
	"testing"

	exportHandlers "erpCluvix/internal/modules/webapp/export"
)

// Hợp đồng round-trip: mọi cột file export ghi ra PHẢI được importer nhận lại. Lệch một key thì cột
// đó im lặng biến mất khi user export → sửa → import lại, không lỗi không cảnh báo.
func TestUserExportFieldKeysAreImportable(t *testing.T) {
	for _, k := range exportHandlers.UserExportFieldKeys() {
		if !userImportValidCols[k] {
			t.Errorf("cột export %q không nằm trong allowlist importer (userHeaderToCol)", k)
		}
	}
}

// Nhãn dòng 1 của file export cũng phải map được — user xoá dòng field key (hoặc dùng file export
// đời cũ 1 dòng header) thì import vẫn phải chạy.
func TestUserExportHeadersMapToColumns(t *testing.T) {
	headers := exportHandlers.UserExportHeaders()
	keys := exportHandlers.UserExportFieldKeys()
	for i, h := range headers {
		col, ok := userHeaderToCol[h]
		if !ok {
			t.Errorf("nhãn export %q không có trong userHeaderToCol", h)
			continue
		}
		if col != keys[i] {
			t.Errorf("nhãn %q map về %q, phải là %q", h, col, keys[i])
		}
	}
}

func TestResolveUserImportColumns(t *testing.T) {
	headers := exportHandlers.UserExportHeaders()
	keys := exportHandlers.UserExportFieldKeys()

	t.Run("file export 2 dòng header — đọc theo dòng field key, dữ liệu từ dòng 3", func(t *testing.T) {
		rows := [][]string{headers, keys, make([]string, len(keys))}
		colIndex, dataStart, dup := resolveUserImportColumns(rows)
		if dup != "" {
			t.Fatalf("báo cột lặp %q trên file hợp lệ", dup)
		}
		if dataStart != 2 {
			t.Errorf("dataStart = %d, muốn 2", dataStart)
		}
		if len(colIndex) != len(keys) {
			t.Errorf("nhận %d cột, muốn %d", len(colIndex), len(keys))
		}
		for i, k := range keys {
			if colIndex[i] != k {
				t.Errorf("cột %d = %q, muốn %q", i, colIndex[i], k)
			}
		}
	})

	t.Run("file cũ 1 dòng nhãn — vẫn chạy, dữ liệu từ dòng 2", func(t *testing.T) {
		rows := [][]string{headers, make([]string, len(headers))}
		colIndex, dataStart, dup := resolveUserImportColumns(rows)
		if dup != "" {
			t.Fatalf("báo cột lặp %q trên file hợp lệ", dup)
		}
		if dataStart != 1 {
			t.Errorf("dataStart = %d, muốn 1", dataStart)
		}
		if len(colIndex) != len(headers) {
			t.Errorf("nhận %d cột, muốn %d", len(colIndex), len(headers))
		}
	})

	t.Run("chỉ có dòng field key — nhận, dữ liệu từ dòng 2", func(t *testing.T) {
		rows := [][]string{keys, make([]string, len(keys))}
		colIndex, dataStart, _ := resolveUserImportColumns(rows)
		if dataStart != 1 {
			t.Errorf("dataStart = %d, muốn 1", dataStart)
		}
		if len(colIndex) != len(keys) {
			t.Errorf("nhận %d cột, muốn %d", len(colIndex), len(keys))
		}
	})

	t.Run("file chỉ vài cột — chỉ nhận đúng những cột có mặt", func(t *testing.T) {
		rows := [][]string{
			{"id", "Nhân viên", "Cột lạ"},
			{"id", "fullname", "khong_ton_tai"},
			{"1", "Nguyen Van A", "x"},
		}
		colIndex, dataStart, dup := resolveUserImportColumns(rows)
		if dup != "" {
			t.Fatalf("báo cột lặp %q", dup)
		}
		if dataStart != 2 {
			t.Errorf("dataStart = %d, muốn 2", dataStart)
		}
		if len(colIndex) != 2 || colIndex[0] != "id" || colIndex[1] != "fullname" {
			t.Errorf("colIndex = %v, muốn {0:id, 1:fullname}", colIndex)
		}
	})

	t.Run("2 cột cùng trỏ 1 field — phải báo lỗi thay vì ghi bừa", func(t *testing.T) {
		rows := [][]string{
			{"Tài khoản", "Nhân viên", "Tài khoản"},
			{"1", "Nguyen Van A", "2"},
		}
		_, _, dup := resolveUserImportColumns(rows)
		if dup != "username" {
			t.Errorf("dup = %q, muốn \"username\"", dup)
		}
	})

	t.Run("không nhận diện được cột nào", func(t *testing.T) {
		rows := [][]string{{"aaa", "bbb"}, {"1", "2"}}
		colIndex, _, _ := resolveUserImportColumns(rows)
		if len(colIndex) != 0 {
			t.Errorf("colIndex = %v, muốn rỗng", colIndex)
		}
	})
}

// Ô export bắt đầu bằng = + - @ bị chèn dấu nháy chống công thức Excel; import phải bóc lại, nếu
// không mỗi vòng export → import lại chồng thêm một dấu vào chính dữ liệu của khách.
func TestUserRowDataUnsanitizes(t *testing.T) {
	colIndex := map[int]string{0: "id", 1: "nickname", 2: "code", 3: "fullname"}
	// Ô thứ 4 có KHOẢNG TRẮNG trước dấu nháy: Unsanitize chỉ bóc khi ký tự đầu tiên là nháy đơn nên
	// TrimSpace bắt buộc chạy TRƯỚC. Làm ngược thứ tự thì dấu nháy chui thẳng vào DB.
	data := userRowData([]string{" 7 ", "'-Anh", "", "  '=Tong  "}, colIndex)
	if data["id"] != "7" {
		t.Errorf("id = %v, muốn \"7\"", data["id"])
	}
	if data["nickname"] != "-Anh" {
		t.Errorf("nickname = %v, muốn \"-Anh\"", data["nickname"])
	}
	if data["fullname"] != "=Tong" {
		t.Errorf("fullname = %v, muốn \"=Tong\" (khoảng trắng dẫn đầu không được che mất dấu nháy)", data["fullname"])
	}
	if _, ok := data["code"]; ok {
		t.Errorf("ô rỗng không được đưa vào data (sẽ ghi đè giá trị đang lưu)")
	}
}

// Nháy đơn vì lý do khác (không phải sanitize) phải giữ nguyên — Unsanitize chỉ bóc khi ký tự ngay
// sau đúng là ký tự công thức.
func TestUserRowDataKeepsUnrelatedQuote(t *testing.T) {
	data := userRowData([]string{"'Anh"}, map[int]string{0: "nickname"})
	if data["nickname"] != "'Anh" {
		t.Errorf("nickname = %v, muốn giữ nguyên \"'Anh\"", data["nickname"])
	}
}

// Trần số dòng phải khớp với con số ghi trong sheet hướng dẫn của file mẫu (cmd/gen-template/user.go)
// — lệch nhau thì user làm đúng hướng dẫn vẫn bị từ chối file.
func TestMaxUserImportRowsMatchesTemplateGuide(t *testing.T) {
	if maxUserImportRows != 5000 {
		t.Errorf("maxUserImportRows = %d; sheet Huong_dan của file mẫu đang ghi 5000 — sửa cả hai", maxUserImportRows)
	}
}

// File export luôn chở ĐỦ cột, nên mảng id (branchId) của mỗi nhân viên quay lại y nguyên dù người
// import không có quyền trên hết các id đó. So sánh phải bỏ qua khác biệt khoảng trắng
// (MySQL trả "[5, 3, 2]", người gõ tay ra "[5,3,2]") — nếu không, re-import file KHÔNG SỬA GÌ vẫn bị
// coi là đổi scope và bị chặn ngay dòng đầu.
func TestJSONIDsChanged(t *testing.T) {
	cases := []struct {
		name        string
		file, cur   string
		wantChanged bool
	}{
		{"y nguyên, khác mỗi space (chính là ca export→import)", "[5, 3, 2, 1, 8, 9, 10]", "[5, 3, 2, 1, 8, 9, 10]", false},
		{"file bỏ space, DB có space", "[5,3,2,1,8,9,10]", "[5, 3, 2, 1, 8, 9, 10]", false},
		{"thêm 1 id", "[5,3,2,11]", "[5, 3, 2]", true},
		{"bớt 1 id", "[5,3]", "[5, 3, 2]", true},
		{"đảo thứ tự = ĐỔI (chặt tay: nghi ngờ thì validate)", "[3,5,2]", "[5, 3, 2]", true},
		{"DB đang NULL/rỗng, file gán mới", "[2]", "", true},
		{"mảng rỗng vs rỗng", "[]", "[]", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jsonIDsChanged(tc.file, tc.cur); got != tc.wantChanged {
				t.Errorf("jsonIDsChanged(%q, %q) = %v, muốn %v", tc.file, tc.cur, got, tc.wantChanged)
			}
		})
	}
}

// Giá trị ĐỔI thì vẫn phải validate đầy đủ — bản vá không được nới lỏng chiều này.
func TestJSONIDsAllowedStillBlocksOutOfScope(t *testing.T) {
	allowed := []uint{1, 2, 3}
	if jsonIDsAllowed([]byte("[1,2,99]"), allowed) {
		t.Error("id 99 ngoài phạm vi mà vẫn cho qua")
	}
	if !jsonIDsAllowed([]byte("[1,3]"), allowed) {
		t.Error("id trong phạm vi lại bị chặn")
	}
}

// Re-import một file export CHƯA SỬA GÌ không được sinh ra UPDATE nào.
// Hai lý do: (1) bump updated_at/updated_by của người không bị sửa là sai dấu vết audit;
// (2) đo thật trên DB dev, mỗi UPDATE `user` một dòng tốn 3–19 GIÂY (bảng có 351 FK trỏ vào),
// 28 dòng ≈ 90s — đúng triệu chứng "import mãi không xong".
func TestSameAsCurrent(t *testing.T) {
	cur := map[string]string{
		"username": "0906904882", "status": "10", "id_old": "",
		"branchId":       "[5, 3, 2, 1, 8, 9, 10]",
		"departmentList": "[9]", "fullname": "Đào Văn Admin", "email": "",
	}
	same := []struct{ col, val string }{
		{"username", "0906904882"},
		{"status", "10"},
		// Story-58: ca mảng id chuyển từ cột companyId (đã bỏ khỏi import) sang branchId — cùng
		// nhánh chuẩn hoá khoảng trắng.
		{"branchId", "[5, 3, 2, 1, 8, 9, 10]"}, // y nguyên từ file export
		{"branchId", "[5,3,2,1,8,9,10]"},       // người gõ tay bỏ space → vẫn là không đổi
		{"departmentList", "[9]"},
		{"fullname", "Đào Văn Admin"},
	}
	for _, c := range same {
		if !sameAsCurrent(c.col, c.val, cur) {
			t.Errorf("sameAsCurrent(%q, %q) = false, muốn true → sẽ sinh UPDATE thừa", c.col, c.val)
		}
	}
	diff := []struct{ col, val string }{
		{"username", "0906904883"},
		{"status", "9"},
		{"branchId", "[5,3,2]"},
		{"branchId", "[3,5,2,1,8,9,10]"}, // đảo thứ tự = đổi
		{"fullname", "Đào Văn Admin 2"},
		{"email", "a@b.c"}, // đang rỗng → gán mới là đổi
	}
	for _, c := range diff {
		if sameAsCurrent(c.col, c.val, cur) {
			t.Errorf("sameAsCurrent(%q, %q) = true, muốn false → thay đổi thật sẽ bị bỏ qua", c.col, c.val)
		}
	}
	// Không có bản ghi cũ (user chưa có hồ sơ) → luôn coi là đổi, không được nuốt mất dữ liệu.
	if sameAsCurrent("code", "NV01", nil) {
		t.Error("current=nil phải trả false")
	}
}

// Story-58 (D23): cột "List thương hiệu"/companyId bỏ khỏi import — file cũ còn cột này thì cột bị
// bỏ qua như cột lạ (không map, không lỗi cột lặp), các cột khác vẫn đọc đúng.
func TestResolveUserImportColumns_S58_BoQuaCotThuongHieu(t *testing.T) {
	if _, ok := userHeaderToCol["List thương hiệu"]; ok {
		t.Fatal(`nhãn "List thương hiệu" vẫn được map`)
	}
	if userImportValidCols["companyId"] {
		t.Fatal("companyId vẫn nằm trong allowlist import")
	}
	rows := [][]string{
		{"Tài khoản", "List thương hiệu", "List chi nhánh"},
		{"username", "companyId", "branchId"},
		{"abcde", "[1]", "[2]"},
	}
	colIndex, dataStart, dup := resolveUserImportColumns(rows)
	if dup != "" || dataStart != 2 {
		t.Fatalf("dup=%q dataStart=%d", dup, dataStart)
	}
	if _, ok := colIndex[1]; ok || colIndex[0] != "username" || colIndex[2] != "branchId" {
		t.Errorf("colIndex = %v — muốn {0:username, 2:branchId}", colIndex)
	}
}
