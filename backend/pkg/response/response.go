package response

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"erpCluvix/pkg/errs"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type ValidationError struct {
	Field      string `json:"field"`
	Message    string `json:"message"`
	FieldLabel string `json:"field_label"`
}

func Data(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func DataWithMessage(c *gin.Context, data interface{}, message string) {
	c.JSON(http.StatusOK, gin.H{"data": data, "message": message})
}

func CreatedDataWithMessage(c *gin.Context, data interface{}, message string) {
	c.JSON(http.StatusCreated, gin.H{"data": data, "message": message})
}

// Accepted — 202: yêu cầu đã NHẬN và đang chạy NỀN, chưa xong.
//
// Khác 200/201 ở đúng một điều mà client cần biết: không có kết quả cuối ở đây, phải hỏi lại trạng
// thái. Dùng 200 cho một việc chạy nền là nói dối về thời điểm hoàn thành.
func Accepted(c *gin.Context, data interface{}) {
	c.JSON(http.StatusAccepted, gin.H{"data": data})
}

// Created — 201 với message i18n "created", không cần truyền message
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, gin.H{"data": data, "message": i18n.T(c, "created")})
}

// Updated — 200 với message i18n "updated", không cần truyền message
func Updated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"data": data, "message": i18n.T(c, "updated")})
}

// CreatedWithWarning — như Created nhưng kèm "warning": cảnh báo mềm (lưu thành công,
// KHÔNG chặn) để FE hiển thị toast/inline vàng. warning rỗng → không thêm field.
func CreatedWithWarning(c *gin.Context, data interface{}, warning string) {
	payload := gin.H{"data": data, "message": i18n.T(c, "created")}
	if warning != "" {
		payload["warning"] = warning
	}
	c.JSON(http.StatusCreated, payload)
}

// UpdatedWithWarning — như Updated nhưng kèm "warning" (cảnh báo mềm, KHÔNG chặn).
func UpdatedWithWarning(c *gin.Context, data interface{}, warning string) {
	payload := gin.H{"data": data, "message": i18n.T(c, "updated")}
	if warning != "" {
		payload["warning"] = warning
	}
	c.JSON(http.StatusOK, payload)
}

func Deleted(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": i18n.T(c, "deleted")})
}

func Validate(c *gin.Context, message string) {
	if isBindError(message) {
		// Bake path vào Message để Telegram hook (chỉ đọc Entry.Message) thấy được path gây lỗi.
		logger.L.Error(fmt.Sprintf("BindError @ %s %s\n%s",
			c.Request.Method, c.Request.URL.String(), message))
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": parseUnmarshalFieldError(c, message)})
		return
	}
	c.JSON(http.StatusUnprocessableEntity, gin.H{"message": message})
}

// ImportErrorGroup — 1 nhóm lỗi cùng loại cho FE render dialog chi tiết (thay vì toast 1 dòng).
// Dùng cho các flow bulk-import cần báo TOÀN BỘ vấn đề trong 1 lần thay vì dừng ở lỗi đầu tiên.
type ImportErrorGroup struct {
	Type  string   `json:"type"`  // khoá nhận dạng (vd "branch", "quantity")
	Label string   `json:"label"` // nhãn hiển thị theo locale hiện tại
	Items []string `json:"items"` // từng dòng lỗi cụ thể, đã format sẵn (kèm số dòng nếu có)
}

// ValidateWithDetails — 422 kèm "details" (nhóm lỗi) bên cạnh "message" tóm tắt.
// FE: skip toast tự động (SKIP_ERROR_NOTIFICATION context token) cho request này, tự đọc
// err.error.details để mở dialog chi tiết; message vẫn giữ để dùng làm tiêu đề/fallback.
func ValidateWithDetails(c *gin.Context, message string, details []ImportErrorGroup) {
	c.JSON(http.StatusUnprocessableEntity, gin.H{"message": message, "details": details})
}

// ValidateWithErrors — 422 kèm "errors" tự do (mảng struct riêng của caller, vd
// [{path, token}]/[{path, code}]) khi hình dạng không khớp ValidationError/ImportErrorGroup.
// message vẫn giữ để FE dùng làm tiêu đề/fallback toast.
func ValidateWithErrors(c *gin.Context, message string, errors interface{}) {
	c.JSON(http.StatusUnprocessableEntity, gin.H{"message": message, "errors": errors})
}

// ValidateErr translates validator errors to user locale then responds 422
func ValidateErr(c *gin.Context, err error) {
	if IsPayloadTooLarge(err) {
		PayloadTooLarge(c)
		return
	}
	c.JSON(http.StatusUnprocessableEntity, gin.H{"message": TranslateValidationErrors(c, err)})
}

func isBindError(msg string) bool {
	return strings.Contains(msg, "cannot unmarshal") ||
		strings.Contains(msg, "invalid character") ||
		strings.Contains(msg, "unexpected end of JSON")
}

// parseUnmarshalFieldError trích field name từ lỗi "cannot unmarshal X into Go struct field A.B.field_name of type T"
// và trả message dạng "{field_label} không hợp lệ"
func parseUnmarshalFieldError(c *gin.Context, msg string) string {
	// Pattern: "cannot unmarshal array into Go struct field BankAccount.BaseModel.company_id of type uint"
	if strings.Contains(msg, "cannot unmarshal") && strings.Contains(msg, "Go struct field") {
		// Lấy phần sau "Go struct field "
		parts := strings.SplitN(msg, "Go struct field ", 2)
		if len(parts) == 2 {
			// Lấy tên field: "BankAccount.BaseModel.company_id" → "company_id"
			fieldPart := strings.SplitN(parts[1], " of type", 2)[0]
			segs := strings.Split(fieldPart, ".")
			field := segs[len(segs)-1]
			label := i18n.FieldLabel(c, field)
			return label + " " + i18n.T(c, "not_valid")
		}
	}
	return i18n.T(c, "server_error")
}

func ValidationErrors(c *gin.Context, errors []ValidationError) {
	c.JSON(http.StatusUnprocessableEntity, errors)
}

func Conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, gin.H{"message": message})
}

// ValidateWithCode / ConflictWithCode — 422/409 kèm `code` máy đọc được bên cạnh `message` cho người đọc.
//
// Vì sao cần: có những lỗi mà FE phải RẼ NHÁNH chứ không chỉ hiện toast — ví dụ liên thông đơn thuốc
// quốc gia, mỗi lý do chặn dẫn tới một hành động khác nhau (gán chi nhánh vào giấy phép / bật giấy
// phép / bảo bác sĩ khai mã / không làm gì cả). Bắt FE so khớp CHUỖI đã dịch là hỏng ngay khi đổi
// locale hoặc sửa câu chữ.
//
// `code` là định danh ổn định, KHÔNG dịch, KHÔNG đổi khi sửa câu thông báo.
func ValidateWithCode(c *gin.Context, message, code string) {
	c.JSON(http.StatusUnprocessableEntity, gin.H{"message": message, "code": code})
}

func ConflictWithCode(c *gin.Context, message, code string) {
	c.JSON(http.StatusConflict, gin.H{"message": message, "code": code})
}

// HandleError checks for ErrVersionConflict (409) before falling back to Validate (422).
// LocalizedError (with args) → i18n.TF. Sentinel errors → i18n.TryT. MySQL codes → ServerError. Plain strings → as-is.
func HandleError(c *gin.Context, err error) {
	if IsPayloadTooLarge(err) {
		PayloadTooLarge(c)
		return
	}
	// Tag-validation fail (binding:"required,..." trên DTO) → translate per-tag.
	// Đặt trước LocalizedError check vì validator.ValidationErrors KHÔNG match
	// errors.As cho *LocalizedError, nhưng có Error() format dài/khó đọc nếu
	// rơi xuống Validate raw fallback.
	if ve, ok := errors.AsType[validator.ValidationErrors](err); ok {
		Validate(c, TranslateValidationErrors(c, ve))
		return
	}
	if le, ok := errors.AsType[*errs.LocalizedError](err); ok {
		Validate(c, i18n.TF(c, le.Key, le.Args...))
		return
	}
	key := err.Error()
	if msg := i18n.TryT(c, key); msg != "" {
		if errors.Is(err, errs.ErrVersionConflict) {
			Conflict(c, msg)
			return
		}
		Validate(c, msg)
		return
	}
	// MySQL error codes (1451 FK, 1452 ref missing, 1062 duplicate, etc.) → ServerError handles them
	if isMySQLError(key, "1451") || isMySQLError(key, "1452") || isMySQLError(key, "1062") ||
		isMySQLError(key, "1364") || isMySQLError(key, "1048") || isMySQLError(key, "1406") ||
		strings.Contains(key, "Duplicate entry") {
		ServerError(c, err)
		return
	}
	Validate(c, key)
}

// SanitizeErrorMessage — như HandleError nhưng TRẢ VỀ string thay vì ghi response ngay, dùng khi
// cần gộp message vào 1 payload lớn hơn (vd nhiều lỗi 1 lần trong `ImportErrorGroup.Items`).
// KHÔNG BAO GIỜ echo raw Go/MySQL error text — lỗi lạ không lường trước → fallback "server_error"
// (raw message có thể chứa tên bảng/constraint nội bộ, rò rỉ schema cho client).
func SanitizeErrorMessage(c *gin.Context, err error) string {
	if ve, ok := errors.AsType[validator.ValidationErrors](err); ok {
		return TranslateValidationErrors(c, ve)
	}
	if le, ok := errors.AsType[*errs.LocalizedError](err); ok {
		return i18n.TF(c, le.Key, le.Args...)
	}
	key := err.Error()
	if msg := i18n.TryT(c, key); msg != "" {
		return msg
	}
	switch {
	case isMySQLError(key, "1062") || strings.Contains(key, "Duplicate entry"):
		return parseDuplicateMessage(c, key)
	case isMySQLError(key, "1451"):
		return i18n.T(c, "fk_constraint")
	case isMySQLError(key, "1452"):
		return i18n.T(c, "fk_ref_missing")
	case isMySQLError(key, "1364"):
		return parseFieldMessage(c, key, "Field '", "required")
	case isMySQLError(key, "1048"):
		return parseFieldMessage(c, key, "Column '", "required")
	case isMySQLError(key, "1406"):
		return parseFieldMessage(c, key, "column '", "data_too_long")
	default:
		return i18n.T(c, "server_error")
	}
}

// IsPayloadTooLarge — err sinh ra do body vượt trần http.MaxBytesReader (middleware.RequestSizeLimit
// bọc MỌI request, kể cả multipart chunked không có Content-Length). Lỗi này nổi lên ở chỗ ĐỌC body
// (ParseMultipartForm/FormFile/ShouldBind) chứ không ở middleware, nên mọi đường báo lỗi chung phải
// nhận ra nó để trả 413 thay vì 500/422 sai nghĩa.
func IsPayloadTooLarge(err error) bool {
	if err == nil {
		return false
	}
	_, ok := errors.AsType[*http.MaxBytesError](err)
	return ok
}

// PayloadTooLarge — 413 + i18n payload_too_large.
func PayloadTooLarge(c *gin.Context) {
	c.JSON(http.StatusRequestEntityTooLarge, gin.H{"message": i18n.T(c, "payload_too_large")})
}

// RejectMultipartTooLarge — parse multipart SỚM ở handler upload để vượt trần được báo đúng 413.
// Lý do: c.PostForm/c.FormFile của gin NUỐT lỗi parse — body quá cỡ biến thành "thiếu field"
// (422 sai nghĩa) hoặc "không có file" (lưu bản ghi thiếu ảnh). true ⇒ đã trả 413, caller return.
// Request không phải multipart ⇒ false (không làm gì).
func RejectMultipartTooLarge(c *gin.Context) bool {
	if !strings.HasPrefix(c.ContentType(), "multipart/") {
		return false
	}
	if _, err := c.MultipartForm(); IsPayloadTooLarge(err) {
		PayloadTooLarge(c)
		return true
	}
	return false
}

func NotFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"message": i18n.T(c, "not_found")})
}

func Forbidden(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"message": i18n.T(c, "forbidden")})
}

// Gone — 410: tài nguyên từng hợp lệ nhưng không còn dùng được (vd URL ký đã hết hạn). message đã qua i18n.
func Gone(c *gin.Context, message string) {
	c.JSON(http.StatusGone, gin.H{"message": message})
}

func ServerError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if IsPayloadTooLarge(err) {
		PayloadTooLarge(c)
		return
	}
	msg := err.Error()

	// MySQL 1062 Duplicate entry → 422
	if isMySQLError(msg, "1062") || strings.Contains(msg, "Duplicate entry") {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": parseDuplicateMessage(c, msg)})
		return
	}

	// MySQL 1451 FK constraint on DELETE (parent row has children) → 422
	if isMySQLError(msg, "1451") {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": i18n.T(c, "fk_constraint")})
		return
	}
	// MySQL 1452 FK constraint on INSERT/UPDATE (referenced row missing) → 422.
	// Extract FK column from constraint name (vd: `fk-customer_care-status_care` → status_care)
	// để FE biết field nào invalid thay vì generic message.
	if isMySQLError(msg, "1452") {
		logger.L.Warn("FK1452: "+msg, zap.String("path", c.Request.Method+" "+c.Request.URL.String()))
		field := extractFKField(msg)
		payload := gin.H{"message": i18n.T(c, "fk_ref_missing")}
		if field != "" {
			payload["field"] = field
			payload["message"] = i18n.T(c, "fk_ref_missing") + " (" + field + ")"
		}
		c.JSON(http.StatusUnprocessableEntity, payload)
		return
	}

	// MySQL 1364 Field doesn't have a default value → 422
	if isMySQLError(msg, "1364") {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": parseFieldMessage(c, msg, "Field '", "required")})
		return
	}

	// MySQL 1048 Column cannot be null → 422
	if isMySQLError(msg, "1048") {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": parseFieldMessage(c, msg, "Column '", "required")})
		return
	}

	// MySQL 1406 Data too long → 422
	if isMySQLError(msg, "1406") {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": parseFieldMessage(c, msg, "column '", "data_too_long")})
		return
	}

	logger.L.Error(fmt.Sprintf("ServerError @ %s %s\n%s",
		c.Request.Method, c.Request.URL.String(), msg))
	c.JSON(http.StatusInternalServerError, gin.H{"message": i18n.T(c, "server_error")})
}

// ServerErrorReadOnly — biến thể read-only của ServerError, dùng cho lỗi phát sinh từ đường ĐỌC
// (SELECT/list — `pkg/pagination.failList`/`FailQuery`). KHÔNG rẽ nhánh 422 theo mã lỗi MySQL
// (1062/1451/1452/1364/1048/1406) — bộ rẽ nhánh đó thiết kế cho INSERT/UPDATE để hiển thị lỗi
// validate cho user (vd tên cột trùng); áp cho lỗi SELECT là "đúng do may" (các mã đó gần như
// không phát sinh từ SELECT thuần) chứ không do thiết kế, và có thể echo mảnh SQL ra client qua
// message field. Luôn trả 500 sạch (`i18n.T(c, "server_error")`), kèm log giống nhánh mặc định
// của ServerError.
func ServerErrorReadOnly(c *gin.Context, err error) {
	if err == nil {
		return
	}
	logger.L.Error(fmt.Sprintf("ServerErrorReadOnly @ %s %s\n%s",
		c.Request.Method, c.Request.URL.String(), err.Error()))
	c.JSON(http.StatusInternalServerError, gin.H{"message": i18n.T(c, "server_error")})
}

// isMySQLError checks for MySQL error code pattern "Error 1048 (23000):" or "(1048)"
func isMySQLError(msg, code string) bool {
	return strings.Contains(msg, "Error "+code) || strings.Contains(msg, "("+code+")")
}

// extractFKField parses MySQL 1452 error to identify which column has the
// invalid FK reference. Format:
//
//	Cannot add or update a child row: a foreign key constraint fails
//	(`schema`.`customer_care`, CONSTRAINT `fk-customer_care-status_care`
//	 FOREIGN KEY (`status_care`) REFERENCES `customer_care_status` (`id`)...)
//
// Returns the column name (vd "status_care") or empty if can't parse.
func extractFKField(msg string) string {
	idx := strings.Index(msg, "FOREIGN KEY (`")
	if idx == -1 {
		return ""
	}
	start := idx + len("FOREIGN KEY (`")
	end := strings.Index(msg[start:], "`")
	if end == -1 {
		return ""
	}
	return msg[start : start+end]
}

func parseDuplicateMessage(c *gin.Context, msg string) string {
	start := strings.Index(msg, "Duplicate entry '")
	if start == -1 {
		return i18n.TF(c, "unique", i18n.T(c, "no_data"))
	}
	start += len("Duplicate entry '")
	end := strings.Index(msg[start:], "'")
	if end == -1 {
		return i18n.TF(c, "unique", i18n.T(c, "no_data"))
	}
	value := msg[start : start+end]

	keyStart := strings.Index(msg, "for key '")
	if keyStart != -1 {
		keyStart += len("for key '")
		keyEnd := strings.Index(msg[keyStart:], "'")
		if keyEnd != -1 {
			key := msg[keyStart : keyStart+keyEnd]
			if dot := strings.LastIndex(key, "."); dot != -1 {
				key = key[dot+1:]
			}
			label := i18n.FieldLabel(c, key)
			return label + " " + value + " " + i18n.T(c, "already_exists")
		}
	}

	return value + " " + i18n.T(c, "already_exists")
}

// parseFieldMessage extracts field name from MySQL error and returns i18n message
func parseFieldMessage(c *gin.Context, msg, prefix, msgKey string) string {
	start := strings.Index(msg, prefix)
	if start == -1 {
		return i18n.T(c, msgKey)
	}
	start += len(prefix)
	end := strings.Index(msg[start:], "'")
	if end == -1 {
		return i18n.T(c, msgKey)
	}
	field := msg[start : start+end]
	label := i18n.FieldLabel(c, field)
	return i18n.TF(c, msgKey, label)
}
