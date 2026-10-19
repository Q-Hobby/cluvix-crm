package export

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"erpCluvix/pkg/export"
	"erpCluvix/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var DownloadURL string

// userCol — 1 cột export nhân sự. Gộp 3 thứ vào MỘT nguồn sự thật (cùng lý do với serviceCol):
//
//   - key    : tên cột DB, ghi ở dòng header thứ 2 của file. Importer (hr/user/handler.ImportUsers)
//     nhận cột theo đúng dòng này — lệch key = cột im lặng bị bỏ qua khi import ngược.
//   - header : nhãn tiếng Việt, dòng header thứ 1. Importer vẫn fallback theo nhãn cho file cũ.
//   - uiKey  : `value` trong FE `displayCols` của UserList — dùng để lọc `?columns=` khi user chọn
//     "chỉ export cột đang hiển thị". Rỗng = cột không có trên bảng (id cũ, mật khẩu,
//     khoa EMR) nên chỉ có mặt ở chế độ export toàn bộ.
type userCol struct {
	key     string
	header  string
	uiKey   string
	extract func(row map[string]interface{}) interface{}
}

func userPlainCol(key, header, uiKey, field string) userCol {
	return userCol{key, header, uiKey, func(r map[string]interface{}) interface{} { return r[field] }}
}

func userJSONCol(key, header, uiKey, field string) userCol {
	return userCol{key, header, uiKey, func(r map[string]interface{}) interface{} { return bytesToString(r[field]) }}
}

// userCols — tập cột export nhân sự = ĐÚNG tập cột import được (cộng `id`), theo đúng thứ tự file
// mẫu `mau_import_nhan_su.xlsx` (sinh từ chính slice này qua `cmd/gen-template -only=user`).
//
// Cột chỉ để xem trên bảng (avatar, người tạo/ngày tạo, người cập nhật/ngày cập nhật) CỐ Ý không có
// ở đây: importer không nhận chúng, đưa vào file sẽ phá lời hứa "export → sửa → import lại".
var userCols = []userCol{
	userPlainCol("id", "id", "id", "id"),
	userPlainCol("id_old", "id cũ", "", "id_old"),
	userPlainCol("code", "Mã chấm công", "code", "code"),
	userPlainCol("fullname", "Nhân viên", "name", "fullname"),
	userPlainCol("nickname", "Bí danh", "nickname", "nickname"),
	userPlainCol("username", "Tài khoản", "username", "username"),
	// Mật khẩu LUÔN xuất ô trống — hash không đảo ngược được, và file export lọt ra ngoài thì cột
	// này là toàn bộ danh sách tài khoản. Ô trống lúc import = tạo mới thì tự sinh, cập nhật thì
	// không đụng vào mật khẩu đang có.
	{"password", "Mật khẩu", "", func(map[string]interface{}) interface{} { return nil }},
	userPlainCol("gender", "Giới tính", "genderUser", "gender"),
	userPlainCol("status", "Trạng thái", "userStatus", "status"),
	userPlainCol("phone", "Số điện thoại", "phone", "phone"),
	userPlainCol("email", "Email", "email", "email"),
	userPlainCol("role", "Vai trò", "description", "role_name"),
	userJSONCol("departmentList", "List phòng ban", "department", "departmentList"),
	userJSONCol("branchId", "List chi nhánh", "branch", "branchId"),
	// Story-38 (D23): bỏ cột "Thương hiệu" khỏi export — 1 thương hiệu/DB không cần cột này.
	// Importer (hr/user ImportUsers) đọc cột theo tên động (resolveUserImportColumns), companyId
	// mặc định `[]` khi cột vắng mặt ⇒ file export CŨ còn cột này vẫn import được bình thường.
}

// UserExportHeaders / UserExportFieldKeys — nhãn và tên cột DB của file export nhân sự, đúng thứ tự
// cột. Generator template (`cmd/gen-template`) và test đối chiếu allowlist importer dùng 2 hàm này
// để template, file export và importer không thể lệch nhau mà build vẫn xanh.
func UserExportHeaders() []string {
	headers := make([]string, len(userCols))
	for i, col := range userCols {
		headers[i] = col.header
	}
	return headers
}

func UserExportFieldKeys() []string {
	keys := make([]string, len(userCols))
	for i, col := range userCols {
		keys[i] = col.key
	}
	return keys
}

// userDisplayOnlyUIKeys — các `value` trong FE `displayCols` CỐ Ý không có cột export tương ứng:
// ảnh đại diện (URL, vô nghĩa trong Excel) và 4 cột dấu vết audit (importer không nhận, đưa vào file
// sẽ phá lời hứa "export → sửa → import lại").
//
// Khai tường minh ở đây để `filterUserExportCols` phân biệt được "cột cố ý bỏ" với "cột lệch tên".
// FE là git repo RIÊNG nên không test nào của 2 bên bắt được việc ai đó đổi `value: 'description'`
// thành `'role'` — không có danh sách này thì cột im lặng biến mất khỏi file export mà build, tsc và
// test đều xanh. Có nó thì key lạ được log ra, drift thành thứ nhìn thấy được.
var userDisplayOnlyUIKeys = map[string]bool{
	"avatar": true, "created_by": true, "created_at": true, "updated_by": true, "updated_at": true,
}

// filterUserExportCols lọc cột theo `?columns=` (FE gửi `value` của displayCols khi user chọn "chỉ
// export cột đang hiển thị"). coreKeys LUÔN được giữ bất kể user chọn gì — `id` là cột duy nhất cho
// phép import ngược nhận ra "cập nhật user nào", thiếu nó thì mọi dòng thành tạo mới.
//
// Trả kèm `unknown` = key FE gửi lên mà không khớp cột export nào và cũng không nằm trong danh sách
// display-only ở trên. Caller log ra để phát hiện lệch hợp đồng FE↔BE.
func filterUserExportCols(colParam string, coreKeys ...string) (cols []userCol, unknown []string) {
	if strings.TrimSpace(colParam) == "" {
		return userCols, nil
	}
	exportable := make(map[string]bool, len(userCols))
	for _, col := range userCols {
		if col.uiKey != "" {
			exportable[col.uiKey] = true
		}
	}
	selected := map[string]bool{}
	for _, k := range strings.Split(colParam, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		selected[k] = true
		if !exportable[k] && !userDisplayOnlyUIKeys[k] {
			unknown = append(unknown, k)
		}
	}
	for _, k := range coreKeys {
		selected[k] = true
	}
	filtered := make([]userCol, 0, len(userCols))
	for _, col := range userCols {
		if col.uiKey != "" && selected[col.uiKey] {
			filtered = append(filtered, col)
		}
	}
	if len(filtered) == 0 {
		return userCols, unknown
	}
	return filtered, unknown
}

// ExportUsers exports user list to Excel/CSV.
// File có 2 dòng header (nhãn + tên cột DB) để user sửa xong import ngược lại qua
// POST /hr/user/import-users mà không phải đổi cấu trúc file.
func ExportUsers(c *gin.Context, q *gorm.DB) {
	cols, unknownCols := filterUserExportCols(c.Query("columns"), "id")
	if len(unknownCols) > 0 {
		// stderr chứ không logger.Warn/Info (global zap pin ErrorLevel, sẽ nuốt câm) và cũng không
		// logger.Error (bắn Telegram alert — đây là lệch hợp đồng cần sửa code, không phải sự cố
		// vận hành cần đánh thức ai).
		fmt.Fprintf(os.Stderr, "[export/user] ?columns= có key không khớp cột nào: %s "+
			"— FE displayCols và userCols.uiKey có thể đã lệch nhau\n", strings.Join(unknownCols, ","))
	}

	headers := make([]string, len(cols))
	keys := make([]string, len(cols))
	for i, col := range cols {
		headers[i] = col.header
		keys[i] = col.key
	}

	url, err := doExport(c, q, export.Config{
		FileName:    "Nhan_su",
		Headers:     headers,
		FieldKeys:   keys,
		ExportDir:   "./storage/export/",
		DownloadURL: DownloadURL,
	}, func(row map[string]interface{}) []interface{} {
		data := make([]interface{}, len(cols))
		for i, col := range cols {
			data[i] = col.extract(row)
		}
		return data
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, url)
}

// bytesToString converts JSON []byte to string, normalising all legacy null forms
// (null, "null", "\"null\"", ...) to nil so they export as empty cells.
func bytesToString(v interface{}) interface{} {
	b, ok := v.([]byte)
	if !ok {
		return v
	}
	s := strings.TrimSpace(string(b))
	// Unwrap JSON string encoding until we reach the real value or a null-like sentinel.
	for strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) && len(s) > 1 {
		var inner string
		if err := json.Unmarshal([]byte(s), &inner); err != nil {
			break
		}
		s = strings.TrimSpace(inner)
	}
	if s == "" || s == "null" || s == "[]" {
		return nil
	}
	return string(b)
}
