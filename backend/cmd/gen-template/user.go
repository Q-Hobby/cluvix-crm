package main

import (
	"log"
	"os"
	"path/filepath"

	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/pkg/xlsxtemplate"

	"github.com/xuri/excelize/v2"
)

// userSheetName — sheet dữ liệu PHẢI là sheet đầu tiên: importer đọc f.GetSheetName(0).
const userSheetName = "Mau_import"

// userFieldNotes — ghi chú cho từng cột, khoá theo TÊN CỘT DB (dòng 2 của file).
// Có test bắt buộc mọi cột của export đều có mặt ở đây: thêm cột vào export mà quên ghi chú thì
// template im lặng ra một cột không ai biết điền gì.
var userFieldNotes = map[string]string{
	"id": "Để trống = tạo nhân viên mới. Có id = cập nhật đúng nhân viên đó. Chỉ sửa được nhân viên thuộc phạm vi bạn thấy trong danh sách.",
	"id_old": "Id từ hệ thống cũ (tuỳ chọn). GIÁ TRỊ DUY NHẤT toàn hệ thống — copy một dòng cũ để tạo " +
		"nhân viên mới thì phải xoá cả ô này, không thì cả file bị từ chối vì trùng id cũ.",
	"code":     "Mã chấm công.",
	"fullname": "Họ tên nhân viên. Hệ thống tự chuẩn hoá viết hoa đầu mỗi từ.",
	"nickname": "Tên gọi khác / bí danh.",
	"username": "Tên đăng nhập. BẮT BUỘC khi tạo mới và không được trùng tài khoản đã có.",
	"password": "CHỈ dùng khi tạo mới. Để trống = hệ thống tự sinh mật khẩu và hiện ra sau khi import xong. " +
		"Với dòng cập nhật, cột này bị bỏ qua — đổi mật khẩu bằng chức năng Đặt lại mật khẩu trên danh sách. " +
		"File export luôn để trống cột này vì mật khẩu đã mã hoá, không đọc ngược được.",
	"gender":         "1 = Nam, 2 = Nữ, 3 = Khác.",
	"status":         "10 = Hoạt động, 9 = Chờ duyệt, 1 = Đã xoá. Để trống khi tạo mới = 10.",
	"phone":          "Số điện thoại. Nếu số này đang thuộc nhân viên khác thì dòng đó bỏ qua cột SĐT (không báo lỗi, các cột khác vẫn cập nhật).",
	"email":          "Địa chỉ email.",
	"role":           "Tên vai trò trong phân quyền (vd: admin, manager). Mỗi nhân viên 1 vai trò. Không cấp được vai trò cao hơn quyền của chính bạn.",
	"departmentList": "Danh sách id phòng ban, dạng JSON. VD: [1,2]. Để trống = giữ nguyên.",
	"branchId":       "Danh sách id chi nhánh, dạng JSON. VD: [1,3]. Chỉ được dùng chi nhánh bạn có quyền.",
}

// userColWidths — bề rộng riêng cho vài cột dài; còn lại dùng mặc định.
var userColWidths = map[string]float64{
	"id": 8, "id_old": 10, "fullname": 26, "nickname": 20, "username": 20,
	"email": 26, "role": 20, "phone": 16,
	"departmentList": 20, "branchId": 20,
}

// userTemplateSpec — file mẫu import nhân sự. Cột và thứ tự cột lấy THẲNG từ export
// (UserExportHeaders/UserExportFieldKeys) nên template và file export không thể lệch nhau.
func userTemplateSpec() xlsxtemplate.Spec {
	return xlsxtemplate.Spec{
		SheetName:  userSheetName,
		GuideTitle: "HƯỚNG DẪN NHẬP FILE IMPORT NHÂN SỰ",
		Headers:    exportHandlers.UserExportHeaders(),
		Keys:       exportHandlers.UserExportFieldKeys(),
		Notes:      userFieldNotes,
		ColWidths:  userColWidths,
		Rules: []string{
			"  - id trống = tạo nhân viên mới, có id = cập nhật nhân viên đó.",
			"  - Copy một dòng có sẵn để tạo nhân viên mới: nhớ xoá CẢ ô id VÀ ô 'id cũ' (id cũ là giá trị duy nhất).",
			"  - Ô để trống = KHÔNG đụng tới giá trị đang lưu. Muốn xoá hẳn một giá trị thì sửa trực tiếp trên form nhân sự.",
			"  - Cột nào không muốn đụng tới thì xoá hẳn cả cột đó khỏi file — hệ thống chỉ ghi những cột có mặt.",
			"  - Tạo mới bắt buộc có Tài khoản và List thương hiệu; thiếu sẽ báo lỗi kèm số dòng.",
			"  - Danh sách id (phòng ban / chi nhánh / thương hiệu / khoa EMR) nhập dạng JSON: [1,2]. Không nhập tên.",
			"  - Không sửa được tài khoản quản trị gốc (id = 1) qua file import.",
			"  - Không để 2 cột cùng trỏ về một trường — hệ thống sẽ từ chối file.",
			"  - Tối đa 5000 dòng dữ liệu mỗi file. Nhiều hơn thì chia nhỏ file ra.",
			"  - Toàn bộ file chạy trong 1 giao dịch: 1 dòng lỗi là rollback tất cả, không có dòng nào được ghi.",
		},
	}
}

func buildUserTemplate() (*excelize.File, error) {
	return xlsxtemplate.Build(userTemplateSpec())
}

func genUserTemplate() {
	f, err := buildUserTemplate()
	if err != nil {
		log.Fatalf("build user template: %v", err)
	}
	defer func() { _ = f.Close() }()

	outPath := filepath.Join("..", "frontend", "src", "assets", "download", "mau_import_nhan_su.xlsx")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}
	if err := f.SaveAs(outPath); err != nil {
		log.Fatalf("save: %v", err)
	}
	log.Printf("✅ Đã tạo: %s", outPath)
}
