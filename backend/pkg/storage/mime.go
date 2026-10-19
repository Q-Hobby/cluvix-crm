package storage

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

// ImageMIMETypes — whitelist cho avatar, banner, ảnh dịch vụ.
var ImageMIMETypes = []string{
	"image/jpeg",
	"image/png",
	"image/gif",
	"image/webp",
}

// AttachmentMIMETypes — whitelist cho file đính kèm phổ biến.
var AttachmentMIMETypes = append(
	append([]string{}, ImageMIMETypes...),
	// Documents
	"application/pdf",
	"application/msword",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document", // .docx
	"application/vnd.ms-excel", // .xls
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",         // .xlsx
	"application/vnd.ms-powerpoint",                                             // .ppt
	"application/vnd.openxmlformats-officedocument.presentationml.presentation", // .pptx
	"text/plain", // .txt
	"text/csv",   // .csv
	// Audio
	"audio/mpeg", // .mp3
	"audio/wav",  // .wav
	"audio/ogg",  // .ogg
	"audio/aac",  // .aac
	"audio/flac", // .flac
	// Video
	"video/mp4",        // .mp4
	"video/quicktime",  // .mov
	"video/x-msvideo",  // .avi
	"video/webm",       // .webm
	"video/x-matroska", // .mkv
	// Archive
	"application/zip",
	"application/x-rar-compressed",
	"application/x-7z-compressed",
)

// DetectMIMEType đọc magic bytes của file và trả về MIME type thực sự.
// Dùng sau ValidateFileType để lấy detected type làm giá trị lưu DB thay vì Content-Type header.
func DetectMIMEType(file *multipart.FileHeader) string {
	f, err := file.Open()
	if err != nil {
		return file.Header.Get("Content-Type")
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := io.ReadFull(f, buf)
	return mimetype.Detect(buf[:n]).String()
}

// detectAllowed sniff magic bytes của file, trả MIME đã nhận diện nếu nằm trong whitelist.
func detectAllowed(file *multipart.FileHeader, allowed []string) (*mimetype.MIME, error) {
	f, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("không thể đọc file")
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := io.ReadFull(f, buf)

	mtype := mimetype.Detect(buf[:n])
	for _, a := range allowed {
		if mtype.Is(a) {
			return mtype, nil
		}
	}
	return nil, fmt.Errorf("loại file '%s' không được phép tải lên", mtype.String())
}

// ValidateFileType kiểm tra magic bytes của file, trả error nếu MIME không nằm trong whitelist.
// Đọc 512 byte đầu (magic bytes) — không phụ thuộc vào Content-Type header do client gửi (client-spoofable).
func ValidateFileType(file *multipart.FileHeader, allowed []string) error {
	_, err := detectAllowed(file, allowed)
	return err
}

// ValidateStream detects the MIME type from the first 512 bytes of r, validates
// against allowed, then returns a reconstituted io.Reader containing the full
// original stream. The original r must not be read again after calling this.
func ValidateStream(r io.Reader, allowed []string) (io.Reader, string, error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, "", fmt.Errorf("không thể đọc file")
	}
	mtype := mimetype.Detect(head[:n])
	for _, a := range allowed {
		if mtype.Is(a) {
			return io.MultiReader(bytes.NewReader(head[:n]), r), mtype.String(), nil
		}
	}
	return nil, "", fmt.Errorf("loại file '%s' không được phép tải lên", mtype.String())
}

// BaseMIME bỏ tham số (`text/plain; charset=utf-8` → `text/plain`), chữ thường.
func BaseMIME(m string) string {
	if mt, _, err := mime.ParseMediaType(m); err == nil {
		return strings.ToLower(mt)
	}
	if i := strings.Index(m, ";"); i >= 0 {
		m = m[:i]
	}
	return strings.ToLower(strings.TrimSpace(m))
}

// IsAllowedMIME — m (bỏ tham số) có nằm trong whitelist không.
func IsAllowedMIME(m string, allowed []string) bool {
	b := BaseMIME(m)
	for _, a := range allowed {
		if b == a {
			return true
		}
	}
	return false
}
