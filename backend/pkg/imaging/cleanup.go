package imaging

import (
	"context"
	"log"

	"erpCluvix/pkg/storage"
)

// CleanupNewOnError dọn file VỪA upload trong request hiện tại khi bước ghi DB phía sau thất bại
// (lỗi SQL, 409 khoá lạc quan, validate muộn…): DB không trỏ tới key đó ⇒ để lại là file mồ côi vĩnh
// viễn trong kho. Ngược chiều với DeleteOldIfChanged (gọi SAU khi DB đã lưu, xoá ảnh CŨ).
//
//   - uploadedKey PHẢI là key mà chính request này vừa nhận từ ProcessAndUpload/HandleFormImageRaw/
//     SaveUploadSmart — KHÔNG BAO GIỜ truyền giá trị đọc từ payload client: payload có thể mang key
//     của một file đang được bản ghi khác dùng, xoá theo nó là xoá nhầm dữ liệu người khác.
//   - "" (request không upload gì) ⇒ no-op.
//   - Trùng ảnh cũ (sau chuẩn hoá về key) ⇒ không xoá: ảnh cũ vẫn đang được DB trỏ tới.
//   - KHÔNG BAO GIỜ đụng oldValue.
//   - allowedPrefixes (BẮT BUỘC, ≥1): đúng basePath handler vừa upload vào — key ngoài prefix ⇒ KHÔNG
//     xoá (phòng thủ chiều sâu cho ràng buộc ở gạch đầu dòng 1). Không truyền ⇒ không bao giờ xoá.
//
// Best-effort — chỉ log, không trả lỗi (caller đang trả lỗi DB cho client).
func CleanupNewOnError(st storage.Store, oldValue, uploadedKey string, allowedPrefixes ...string) {
	if st == nil || uploadedKey == "" {
		return
	}
	if oldValue != "" && storage.ToKey(st, oldValue) == uploadedKey {
		return
	}
	if storage.IsAbsoluteURL(uploadedKey) || storage.SafeKey(uploadedKey) != nil {
		return
	}
	if !KeyInPrefixes(uploadedKey, allowedPrefixes) {
		log.Printf("imaging: bỏ qua dọn file %q — nằm ngoài prefix cho phép %v", uploadedKey, allowedPrefixes)
		return
	}
	if err := st.Delete(context.Background(), uploadedKey); err != nil {
		log.Printf("imaging: dọn file mới %q sau lỗi ghi DB thất bại (file mồ côi): %v", uploadedKey, err)
	}
}

// PendingUploads — khuôn defer cho handler có NHIỀU nhánh return sau khi upload (validate muộn,
// 409, lỗi tx…): mọi file đã Track mà handler chưa Keep() thì bị dọn khi hàm kết thúc.
//
//	pending := imaging.NewPendingUploads(h.Store, "service/")
//	defer pending.Cleanup()
//	key, err := imaging.ProcessAndUpload(...)
//	pending.Track(oldImage, key)
//	... ghi DB; lỗi ⇒ return (file tự được dọn) ...
//	pending.Keep() // NGAY sau khi DB commit thành công
type PendingUploads struct {
	st       storage.Store
	prefixes []string
	items    [][2]string // {oldValue, uploadedKey}
	kept     bool
}

// NewPendingUploads tạo bộ theo dõi file upload chờ DB xác nhận. allowedPrefixes: xem CleanupNewOnError.
func NewPendingUploads(st storage.Store, allowedPrefixes ...string) *PendingUploads {
	return &PendingUploads{st: st, prefixes: allowedPrefixes}
}

// Track ghi nhận 1 file vừa upload (uploadedKey "" ⇒ bỏ qua). Xem CleanupNewOnError về ràng buộc
// của uploadedKey.
func (p *PendingUploads) Track(oldValue, uploadedKey string) {
	if uploadedKey == "" {
		return
	}
	p.items = append(p.items, [2]string{oldValue, uploadedKey})
}

// Keep đánh dấu DB đã lưu thành công — Cleanup thành no-op.
func (p *PendingUploads) Keep() { p.kept = true }

// Cleanup dọn mọi file đã Track nếu chưa Keep(). Gọi bằng defer.
func (p *PendingUploads) Cleanup() {
	if p.kept {
		return
	}
	for _, it := range p.items {
		CleanupNewOnError(p.st, it[0], it[1], p.prefixes...)
	}
	p.items = nil
}
