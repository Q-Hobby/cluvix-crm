package imaging

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	"erpCluvix/pkg/storage"
)

// UploadResult mô tả file NHƯ NÓ NẰM TRONG KHO sau khi xử lý — không phải file client gửi lên.
//
// Vì sao cần: các luồng đính kèm lưu `mime`/`size` vào DB. Trước đây chúng lấy từ
// `header.Header.Get("Content-Type")` và `header.Size`, tức thông tin của FILE GỐC. Khi ảnh được
// convert sang WebP thì hai giá trị đó sai, FE hiển thị lệch (rủi ro R4 trong architecture.md).
type UploadResult struct {
	Key      string // đường dẫn tương đối trong kho — giá trị lưu DB (KHÔNG phải URL)
	MIME     string
	Size     int64
	Filename string // tên cuối cùng trong kho, không phải tên client gửi
}

// SaveUploadSmart — CỔNG VÀO DUY NHẤT cho các luồng đính kèm hỗn hợp (ảnh lẫn PDF/video/audio/office).
//
// Phân nhánh theo MAGIC BYTES, không theo đuôi file và không theo Content-Type client gửi:
//
//	HEIC/HEIF              → từ chối tường minh
//	MIME thuộc ảnh         → pipeline (resize + WebP + guard bomb)
//	còn lại                → lưu nguyên bản (whitelist AttachmentMIMETypes, đuôi theo MIME đã sniff)
//
// Dùng cho: crm/attachment (kho PRIVATE).
func SaveUploadSmart(ctx context.Context, st storage.Store, file *multipart.FileHeader,
	basePath string, opts *Options) (UploadResult, error) {

	if file == nil {
		return UploadResult{}, fmt.Errorf("không có file để tải lên")
	}

	// Cố ý KHÔNG kiểm st ở đây: mọi guard (HEIC, bomb, định dạng) phải chạy TRƯỚC khi chạm hạ
	// tầng — giống ProcessAndUpload. Nhờ vậy guard chứng minh được bằng test không cần kho, và
	// file xấu không bao giờ đi tới bước lưu. Kiểm st nằm ở saveAsRaw/UploadProcessed.
	detected := storage.DetectMIMEType(file)

	// HEIC phải chặn TRƯỚC nhánh raw. Bẫy: image/heic KHÔNG nằm trong ImageMIMETypes (nên không
	// vào nhánh ảnh) và cũng KHÔNG nằm trong AttachmentMIMETypes — nếu để rơi xuống
	// SaveUploadedFile, user nhận thông báo chung "loại file không được phép" thay vì hướng dẫn
	// "đổi sang JPG". Kiểm cả đuôi file để bắt cả trường hợp mimetype không nhận diện được.
	if isHEICMIME(detected) || heicExts[strings.ToLower(filepath.Ext(file.Filename))] {
		return UploadResult{}, heicError()
	}

	if isImageMIME(detected) {
		return saveAsImage(ctx, st, file, basePath, opts)
	}
	return saveAsRaw(ctx, st, file, basePath)
}

// saveAsImage — nhánh ảnh.
//
// Dùng ProcessFile + UploadProcessed thay vì ProcessAndUpload vì cần độ dài byte của KẾT QUẢ để
// điền UploadResult.Size (ProcessAndUpload chỉ trả URL).
// Bỏ qua storage.ValidateFileType không làm yếu bảo mật: caller chỉ vào nhánh này sau khi
// isImageMIME(detected) đã kiểm chính magic bytes đó; ProcessFile còn tự guard ext + bomb + HEIC.
func saveAsImage(ctx context.Context, st storage.Store, file *multipart.FileHeader,
	basePath string, opts *Options) (UploadResult, error) {

	result, err := ProcessFile(ctx, file, opts)
	if err != nil {
		return UploadResult{}, err
	}
	key, err := UploadProcessed(ctx, st, result, basePath)
	if err != nil {
		return UploadResult{}, err
	}
	// MIME/Size lấy từ KẾT QUẢ, không phải từ file gốc — đây là điểm chính của UploadResult.
	return UploadResult{
		Key:      key,
		MIME:     result.ContentType,
		Size:     int64(len(result.Data)),
		Filename: result.Filename,
	}, nil
}

// saveAsRaw — nhánh không phải ảnh: lưu nguyên bản. storage.SaveUpload tự kiểm whitelist
// AttachmentMIMETypes theo magic bytes (file lạ vẫn bị từ chối) và đặt đuôi theo MIME đã sniff.
func saveAsRaw(ctx context.Context, st storage.Store, file *multipart.FileHeader, basePath string) (UploadResult, error) {
	if st == nil {
		return UploadResult{}, fmt.Errorf("storage chưa được cấu hình")
	}
	saved, err := storage.SaveUpload(ctx, st, file, basePath, storage.AttachmentMIMETypes)
	if err != nil {
		return UploadResult{}, err
	}
	return UploadResult{
		Key:      saved.Key,
		MIME:     saved.MIME,
		Size:     saved.Size,
		Filename: filenameFromURL(saved.Key),
	}, nil
}

// filenameFromURL — suy tên file từ URL đã dựng.
//
// Cắt query/fragment trước khi lấy Base: hiện `s3c.URL()` không kèm tham số, nhưng nếu sau này
// dùng signed URL (`?X-Amz-...`) thì `filepath.Base` trần sẽ nuốt cả chuỗi ký + trả tên rác, và
// sai đó KHÔNG lộ ra lỗi — nó chỉ nằm im trong cột metadata.
func filenameFromURL(url string) string {
	if i := strings.IndexAny(url, "?#"); i >= 0 {
		url = url[:i]
	}
	return filepath.Base(url)
}

// IsImageUpload — file này có phải ẢNH (theo magic bytes) không.
//
// Dùng khi caller cần rẽ nhánh TRƯỚC lúc gọi SaveUploadSmart, ví dụ áp trần dung lượng khác nhau:
// ảnh đằng nào cũng bị nén xuống nên trần thấp là hợp lý, còn PDF/tài liệu giữ nguyên bản nên
// trần thấp sẽ CHẶN NHẦM dữ liệu thật.
func IsImageUpload(file *multipart.FileHeader) bool {
	if file == nil {
		return false
	}
	return isImageMIME(storage.DetectMIMEType(file))
}

func isImageMIME(mime string) bool {
	for _, allowed := range storage.ImageMIMETypes {
		if mime == allowed {
			return true
		}
	}
	return false
}
