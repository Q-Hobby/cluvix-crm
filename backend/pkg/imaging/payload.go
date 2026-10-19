package imaging

import (
	"errors"

	"erpCluvix/pkg/storage"
)

// ErrImageKeyNotAllowed — payload mang một giá trị field ảnh KHÔNG phải ảnh hiện tại của bản ghi và
// cũng không rỗng. Handler trả 422 với i18n key "image_key_not_allowed".
var ErrImageKeyNotAllowed = errors.New("image_key_not_allowed")

// ResolvePayloadImage — chính sách field ảnh dạng CHUỖI trong payload ở đường SỬA (2026-10-19).
//
// Ảnh MỚI chỉ được đến từ file upload trong chính request (HandleFormImageRaw/ProcessAndUpload).
// Chuỗi client gửi chỉ có 2 nghĩa hợp lệ:
//
//   - "" ⇒ xoá ảnh: trả ("", nil).
//   - trùng ảnh hiện tại (so sau storage.ToKey — FE gửi lại nguyên URL GET đã trả) ⇒ giữ nguyên:
//     trả (ToKey(current), nil).
//
// Giá trị khác ⇒ (ToKey(current), ErrImageKeyNotAllowed). Trước đây handler nhận BẤT KỲ key nào:
// nhân viên gửi key logo/ảnh người khác (hoặc `private/...` khi S3 chung bucket) rồi lần sau gửi
// rỗng ⇒ DeleteOldIfChanged xoá file của người khác.
func ResolvePayloadImage(st storage.Store, current, client string) (string, error) {
	cur := storage.ToKey(st, current)
	if client == "" {
		return "", nil
	}
	if storage.ToKey(st, client) == cur {
		return cur, nil
	}
	return cur, ErrImageKeyNotAllowed
}
