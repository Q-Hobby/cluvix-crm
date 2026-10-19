package testhelpers

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// PNGBytes — ảnh PNG w×h hợp lệ (gradient) cho test upload ảnh qua pipeline imaging.
func PNGBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), uint8(x + y), 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("PNGBytes: %v", err)
	}
	return buf.Bytes()
}

// MultipartBody dựng body multipart: các field text + (tuỳ chọn) 1 file. Trả body + Content-Type.
func MultipartBody(t *testing.T, fields map[string]string, fileField, fileName string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("MultipartBody: %v", err)
		}
	}
	if fileField != "" {
		fw, err := mw.CreateFormFile(fileField, fileName)
		if err != nil {
			t.Fatalf("MultipartBody: %v", err)
		}
		_, _ = fw.Write(content)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("MultipartBody: %v", err)
	}
	return &buf, mw.FormDataContentType()
}

// ListStoreFiles — mọi file (key tương đối, sort) đang nằm dưới thư mục gốc của kho storage.Local.
// Bỏ qua file ẩn (file tạm ghi dở của Local.Save).
func ListStoreFiles(t *testing.T, root string) []string {
	t.Helper()
	out := []string{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasPrefix(info.Name(), ".") {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ListStoreFiles: %v", err)
	}
	sort.Strings(out)
	return out
}
