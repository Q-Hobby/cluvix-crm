package storage

import (
	"context"
	"fmt"
	"mime/multipart"
	"strings"
	"time"

	"erpCluvix/pkg/helpers"
)

// SavedFile — file như nó nằm trong kho sau khi lưu.
type SavedFile struct {
	Key  string // đường dẫn tương đối trong kho — giá trị lưu DB
	MIME string // MIME đã sniff từ magic bytes (không phải header client)
	Size int64
}

// SaveUpload lưu NGUYÊN BẢN một file multipart vào kho s dưới basePath.
//
//   - Kiểm magic bytes theo whitelist allowed (Content-Type client khai không được tin).
//   - Đuôi file suy từ MIME ĐÃ SNIFF, KHÔNG lấy từ tên client: `shell.php` đổi tên `anh.jpg` vẫn
//     không đổi được nội dung, và tên gốc (thường chứa họ tên khách) không lộ vào key.
//   - Content-Type ghi vào kho = MIME đã sniff: object public mà mang Content-Type client khai
//     (`text/html`) là stored XSS trên domain phục vụ file.
func SaveUpload(ctx context.Context, s Store, file *multipart.FileHeader, basePath string, allowed []string) (SavedFile, error) {
	if s == nil {
		return SavedFile{}, fmt.Errorf("storage chưa được cấu hình")
	}
	if file == nil {
		return SavedFile{}, fmt.Errorf("không có file để tải lên")
	}
	mtype, err := detectAllowed(file, allowed)
	if err != nil {
		return SavedFile{}, err
	}

	key := NewKey(basePath, mtype.Extension())
	src, err := file.Open()
	if err != nil {
		return SavedFile{}, fmt.Errorf("open file: %w", err)
	}
	defer src.Close()

	contentType := BaseMIME(mtype.String())
	if err := s.Save(ctx, src, key, contentType); err != nil {
		return SavedFile{}, err
	}
	return SavedFile{Key: key, MIME: mtype.String(), Size: file.Size}, nil
}

// NewKey sinh key ngẫu nhiên `<basePath><random>-<unix><ext>`. ext gồm dấu chấm (".pdf") hoặc rỗng.
func NewKey(basePath, ext string) string {
	if basePath != "" && !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}
	return fmt.Sprintf("%s%s-%d%s", strings.TrimLeft(basePath, "/"), helpers.RandomString(16), time.Now().Unix(), strings.ToLower(ext))
}
