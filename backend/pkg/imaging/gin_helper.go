package imaging

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"erpCluvix/pkg/storage"

	"github.com/gin-gonic/gin"
)

// HandleFormImage đọc field multipart, xử lý (resize+webp), lưu vào kho PUBLIC và ghi KEY vào *target.
// Không có file ⇒ nil (không phải lỗi). Chỉ trả lỗi khi xử lý/lưu thật sự thất bại.
//
// *target sau hàm này là KEY (đường dẫn tương đối) — trả FE phải ghép URL qua storage.PublicURL.
//
//	if err := imaging.HandleFormImage(c, h.Store, "logoFile", "business_profile/", &item.LogoImage, nil); err != nil {
//	    response.HandleError(c, err); return
//	}
func HandleFormImage(c *gin.Context, st storage.Store, formField, basePath string, target *string, opts *Options) error {
	file, err := c.FormFile(formField)
	if isBodyTooLarge(err) {
		return err // body vượt trần ⇒ caller (response.HandleError) trả 413, KHÔNG coi là "không có file"
	}
	if err != nil || file == nil {
		return nil // no file uploaded — not an error
	}
	if st == nil {
		return nil
	}
	key, err := ProcessAndUpload(c.Request.Context(), st, file, basePath, opts)
	if err != nil {
		return err
	}
	*target = key
	return nil
}

// HandleFormImageRaw như HandleFormImage nhưng trả KEY thay vì ghi vào con trỏ. ("", nil) khi không có file.
func HandleFormImageRaw(c *gin.Context, st storage.Store, formField, basePath string, opts *Options) (string, error) {
	file, err := c.FormFile(formField)
	if isBodyTooLarge(err) {
		return "", err
	}
	if err != nil || file == nil {
		return "", nil
	}
	if st == nil {
		return "", nil
	}
	return ProcessAndUpload(c.Request.Context(), st, file, basePath, opts)
}

// isBodyTooLarge — lỗi đọc body do vượt trần http.MaxBytesReader (middleware.RequestSizeLimit).
// Trước đây mọi lỗi FormFile bị nuốt thành "không có file" ⇒ upload quá cỡ (chunked) âm thầm lưu
// bản ghi KHÔNG có ảnh mà vẫn báo thành công.
func isBodyTooLarge(err error) bool {
	_, ok := errors.AsType[*http.MaxBytesError](err)
	return err != nil && ok
}

// DeleteOldIfChanged xoá ảnh cũ trong kho khi giá trị cũ khác giá trị mới (ảnh bị thay hoặc bị xoá).
// Best-effort — không trả lỗi (chỉ log), gọi SAU KHI DB đã lưu thành công (DB là nguồn sự thật).
//
// Cả 2 giá trị được chuẩn hoá về KEY trước khi so (storage.ToKey): giá trị cũ có thể là URL tuyệt
// đối (dữ liệu trước 2026-10-19) hoặc FE gửi lại nguyên URL nó nhận — so chuỗi thô sẽ thấy "khác"
// và XOÁ NHẦM ảnh đang dùng. URL tuyệt đối không thuộc kho hiện tại (CDN cũ đã đổi, link ngoài)
// ⇒ không xoá được, bỏ qua.
//
// allowedPrefixes (BẮT BUỘC, ≥1): CHỈ xoá key nằm dưới đúng prefix upload của handler gọi
// (vd "branch/", "user-profile/<uid>/avatar/"). Giá trị cũ trong DB có thể là key mà client từng
// nhét qua payload (dữ liệu trước bản vá 2026-10-19: logo thương hiệu, ảnh người khác, thậm chí
// `private/...` — file đính kèm riêng tư khi S3 dùng chung bucket) ⇒ xoá theo nó là xoá file của
// người khác. Ngoài prefix ⇒ KHÔNG xoá, chỉ log cảnh báo (file mồ côi an toàn hơn xoá nhầm).
// Không truyền prefix nào ⇒ không bao giờ xoá.
//
//	oldImage := item.Image // capture TRƯỚC khi bind field mới đè lên
//	... bind + upload + DB.Save ...
//	imaging.DeleteOldIfChanged(h.Store, oldImage, item.Image, "branch/")
func DeleteOldIfChanged(st storage.Store, oldValue, newValue string, allowedPrefixes ...string) {
	if st == nil || oldValue == "" {
		return
	}
	oldKey := storage.ToKey(st, oldValue)
	if oldKey == storage.ToKey(st, newValue) {
		return
	}
	if storage.IsAbsoluteURL(oldKey) || storage.SafeKey(oldKey) != nil {
		return
	}
	if !KeyInPrefixes(oldKey, allowedPrefixes) {
		log.Printf("imaging: bỏ qua xoá ảnh cũ %q — nằm ngoài prefix cho phép %v", oldKey, allowedPrefixes)
		return
	}
	if err := st.Delete(context.Background(), oldKey); err != nil {
		log.Printf("imaging: xoá ảnh cũ %q lỗi (bỏ qua, file mồ côi): %v", oldKey, err)
	}
}

// KeyInPrefixes — key (đã chuẩn hoá, đã qua SafeKey) nằm dưới ít nhất 1 prefix hợp lệ. Prefix rỗng
// hoặc không kết thúc bằng "/" bị bỏ qua: "" khớp mọi key, "branch" khớp cả "branchX/...".
func KeyInPrefixes(key string, prefixes []string) bool {
	for _, p := range prefixes {
		if p == "" || !strings.HasSuffix(p, "/") || storage.SafeKey(p) != nil {
			continue
		}
		if strings.HasPrefix(key, p) && len(key) > len(p) {
			return true
		}
	}
	return false
}
