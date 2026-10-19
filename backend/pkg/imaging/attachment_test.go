package imaging

// Test cho SaveUploadSmart — cổng vào của 4 luồng đính kèm hỗn hợp.
//
// KHÔNG gọi S3 thật: `s3c = nil` cho các case phải bị từ chối TRƯỚC khi chạm S3, còn các case
// đi tới bước upload thì xác nhận qua lỗi "S3 chưa được cấu hình" — đủ để chứng minh nhánh nào
// được chọn mà không cần hạ tầng.

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"mime/multipart"
	"strings"
	"testing"
)

const pdfBody = "%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"

func TestSaveUploadSmart_NilS3(t *testing.T) {
	fh := newTestImageFile(t, "png", 100, 100, false)

	_, err := SaveUploadSmart(t.Context(), nil, fh, "test/", nil)
	if err == nil {
		t.Fatal("muốn lỗi khi kho nil, nhận nil")
	}
	if !strings.Contains(err.Error(), "storage") {
		t.Errorf("err = %v, muốn nói về storage chưa cấu hình", err)
	}
}

// AC9 — HEIC phải bị chặn TƯỜNG MINH, không rơi xuống nhánh raw.
//
// Bẫy: image/heic không nằm trong ImageMIMETypes (nên không vào nhánh ảnh) và cũng KHÔNG nằm
// trong AttachmentMIMETypes. Nếu quên chặn, SaveUploadedFile sẽ từ chối nó — đúng về mặt chặn
// nhưng SAI thông điệp (user không biết phải đổi sang JPG).
func TestSaveUploadSmart_RejectsHEIC(t *testing.T) {
	heic := []byte{
		0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'h', 'e', 'i', 'c',
		0x00, 0x00, 0x00, 0x00, 'h', 'e', 'i', 'c', 'm', 'i', 'f', '1',
	}

	t.Run("theo magic bytes (đổi tên thành .jpg)", func(t *testing.T) {
		fh := newTestFileHeader(t, "gia-dang.jpg", heic)

		_, err := SaveUploadSmart(t.Context(), nil, fh, "test/", nil)
		if !errors.Is(err, ErrHEICNotSupported) {
			t.Errorf("err = %v, muốn ErrHEICNotSupported", err)
		}
	})

	t.Run("theo đuôi file", func(t *testing.T) {
		fh := newTestFileHeader(t, "anh.heic", []byte("noi dung bat ky"))

		_, err := SaveUploadSmart(t.Context(), nil, fh, "test/", nil)
		if !errors.Is(err, ErrHEICNotSupported) {
			t.Errorf("err = %v, muốn ErrHEICNotSupported", err)
		}
	})
}

// AC11 — ảnh bomb bị guard chặn, KHÔNG upload gì.
func TestSaveUploadSmart_RejectsBomb(t *testing.T) {
	fh := newTestFileHeader(t, "bomb.png", pngBomb(t, 20000, 20000))

	_, err := SaveUploadSmart(t.Context(), nil, fh, "test/", nil)
	if !errors.Is(err, ErrImageDimensionExceeded) {
		t.Errorf("err = %v, muốn ErrImageDimensionExceeded", err)
	}
}

// AC7 + AC10 — phân nhánh theo NỘI DUNG THẬT, không theo đuôi file.
//
// PDF đổi tên thành .jpg phải đi nhánh raw. Nếu phân nhánh theo đuôi, nó sẽ vào pipeline ảnh và
// chết ở bước decode với thông báo khó hiểu.
func TestSaveUploadSmart_BranchesByContentNotExtension(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		body     []byte
		wantRaw  bool
	}{
		{"PDF đúng đuôi", "tai-lieu.pdf", []byte(pdfBody), true},
		{"PDF đổi đuôi thành .jpg", "gia-dang.jpg", []byte(pdfBody), true},
		{"PNG đúng đuôi", "anh.png", encodeTestImage(t, "png", 900, 700, true), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fh := newTestFileHeader(t, tc.filename, tc.body)

			// s3c nil → cả 2 nhánh đều dừng ở bước upload. Phân biệt nhánh bằng LOẠI lỗi:
			// nhánh ảnh chết vì "S3 chưa được cấu hình" (từ UploadProcessed), nhánh raw panic-free
			// cũng dừng sớm ở kiểm tra s3c nil đầu hàm. Nên ở đây chỉ khẳng định KHÔNG có lỗi
			// đặc trưng của nhánh sai.
			_, err := SaveUploadSmart(t.Context(), nil, fh, "test/", nil)
			if err == nil {
				t.Fatal("muốn lỗi vì s3c nil")
			}
			if errors.Is(err, ErrHEICNotSupported) || errors.Is(err, ErrImageDimensionExceeded) {
				t.Errorf("bị guard chặn nhầm: %v", err)
			}
			if tc.wantRaw && strings.Contains(err.Error(), "decode image") {
				t.Errorf("file không phải ảnh bị đẩy vào pipeline ảnh: %v", err)
			}
		})
	}
}

// isImageMIME là bản lề phân nhánh — sai ở đây là sai toàn bộ hành vi.
func TestIsImageMIME(t *testing.T) {
	tests := map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"image/gif":       true,
		"image/webp":      true,
		"image/heic":      false, // chặn ở bước riêng, KHÔNG được coi là ảnh xử lý được
		"image/heif":      false,
		"image/svg+xml":   false, // SVG là vector + có thể chứa script
		"application/pdf": false,
		"video/mp4":       false,
		"text/plain":      false,
		"":                false,
	}
	for mime, want := range tests {
		if got := isImageMIME(mime); got != want {
			t.Errorf("isImageMIME(%q) = %v, muốn %v", mime, got, want)
		}
	}
}

// AC1 — preset y tế. Khoá cả 3 điểm lệch có chủ đích so với preset thường.
func TestOptMedical_Values(t *testing.T) {
	opts := OptMedical()

	if opts.Mode != ModeFit {
		t.Errorf("Mode = %v, muốn ModeFit (KHÔNG crop — ảnh lâm sàng không được cắt rìa tổn thương)", opts.Mode)
	}
	if opts.MaxWidth != 2048 || opts.MaxHeight != 2048 {
		t.Errorf("hộp = %dx%d, muốn 2048x2048 (user chốt brief.md Q1)", opts.MaxWidth, opts.MaxHeight)
	}
	if opts.QMin != 88 || opts.QMax != 95 {
		t.Errorf("quality = %d-%d, muốn 88-95", opts.QMin, opts.QMax)
	}
	if opts.TargetKB != 700 {
		t.Errorf("TargetKB = %d, muốn 700", opts.TargetKB)
	}
	if opts.Sharpen {
		t.Error("Sharpen = true — ảnh chẩn đoán KHÔNG được sharpen (tạo cạnh giả, dễ đọc nhầm là ranh giới tổn thương)")
	}
}

// Ảnh y tế lớn phải xuống ≤ 2048 và nhẹ đi đáng kể — phần đo được của PRD AC6.
// (Phần còn lại — chất lượng nhìn bằng mắt — thuộc story-11, không tự động hoá được.)
func TestOptMedical_ResizesLargePhoto(t *testing.T) {
	fh := newTestImageFile(t, "jpg", 4032, 3024, true)

	res, err := ProcessFile(t.Context(), fh, OptMedical())
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if res.ContentType != "image/webp" {
		t.Fatalf("ContentType = %q, muốn image/webp", res.ContentType)
	}

	b := decodeBounds(t, res.Data)
	if b.Dx() > 2048 || b.Dy() > 2048 {
		t.Errorf("kích thước = %dx%d, muốn cạnh dài ≤ 2048", b.Dx(), b.Dy())
	}
	if b.Dx() != 2048 {
		t.Errorf("chiều rộng = %d, muốn đúng 2048 (fit theo cạnh dài)", b.Dx())
	}

	saved := 100 - (len(res.Data) * 100 / int(fh.Size))
	if saved < 70 {
		t.Errorf("chỉ giảm %d%% dung lượng, muốn ≥ 70%% (PRD AC6)", saved)
	}
	t.Logf("gốc %dKB → %dKB (giảm %d%%)", fh.Size/1024, len(res.Data)/1024, saved)
}

// ── Preset line art (chữ ký điện tử) ─────────────────────────

func TestOptLineArt_Values(t *testing.T) {
	opts := OptLineArt()

	if !opts.Lossless {
		t.Error("Lossless = false — lossy sinh ringing quanh nét mảnh, làm chữ ký rỗ/đứt ở MỌI quality")
	}
	if opts.Sharpen {
		t.Error("Sharpen = true — không sharpen line art")
	}
	if opts.Mode != ModeFit {
		t.Errorf("Mode = %v, muốn ModeFit (không crop chữ ký)", opts.Mode)
	}
	if opts.SkipBelowKB != -1 {
		t.Errorf("SkipBelowKB = %d, muốn -1 (luôn chuẩn hoá về WebP lossless)", opts.SkipBelowKB)
	}
}

// signatureLikePNG dựng ảnh giống chữ ký vẽ tay: nền TRONG SUỐT + vài nét mảnh 2px.
func signatureLikePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	ink := color.NRGBA{R: 20, G: 30, B: 90, A: 255}
	for x := 0; x < w; x++ {
		y := h/2 + int(float64(h)/4*math.Sin(float64(x)/float64(w)*6*math.Pi))
		for dy := 0; dy < 2; dy++ {
			if y+dy >= 0 && y+dy < h {
				m.SetNRGBA(x, y+dy, ink)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// Chữ ký điện tử: lossless phải giữ ĐÚNG pixel nét và nền trong suốt.
// Đây là bằng chứng pháp lý — mất nét là mất giá trị chứng minh.
func TestOptLineArt_PreservesStrokesAndAlpha(t *testing.T) {
	fh := newTestFileHeader(t, "signature.png", signatureLikePNG(t, 600, 200))

	res, err := ProcessFile(t.Context(), fh, OptLineArt())
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if res.ContentType != "image/webp" {
		t.Fatalf("ContentType = %q, muốn image/webp", res.ContentType)
	}

	out, _, err := image.Decode(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("decode kết quả: %v", err)
	}
	if b := out.Bounds(); b.Dx() != 600 || b.Dy() != 200 {
		t.Errorf("kích thước = %dx%d, muốn giữ nguyên 600x200 (không upscale/crop)", b.Dx(), b.Dy())
	}

	src, _, err := image.Decode(bytes.NewReader(signatureLikePNG(t, 600, 200)))
	if err != nil {
		t.Fatalf("decode gốc: %v", err)
	}
	// Lossless ⇒ pixel-exact. Chỉ cần 1 pixel lệch là đã mất tính "bằng chứng".
	diff := 0
	for y := 0; y < 200; y++ {
		for x := 0; x < 600; x++ {
			if src.At(x, y) != out.At(x, y) {
				diff++
			}
		}
	}
	if diff != 0 {
		t.Errorf("%d pixel lệch so với bản gốc — lossless phải pixel-exact", diff)
	}
}

// Lossless trên LINE ART phải nhỏ hơn PNG gốc (nếu không thì chọn sai công cụ).
func TestOptLineArt_SmallerThanSourcePNG(t *testing.T) {
	src := signatureLikePNG(t, 600, 200)
	fh := newTestFileHeader(t, "signature.png", src)

	res, err := ProcessFile(t.Context(), fh, OptLineArt())
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	t.Logf("PNG gốc %d byte → WebP lossless %d byte", len(src), len(res.Data))
	if len(res.Data) >= len(src) {
		t.Errorf("WebP lossless %d byte ≥ PNG gốc %d byte", len(res.Data), len(src))
	}
}

// D19 — preset line art được chọn theo `mode` do client gửi. Nếu ảnh CHỤP bị gửi nhầm vào mode
// electronic, lossless sẽ cho file to gấp nhiều lần. `encodeWebPPreferLossless` encode cả hai rồi
// chọn bản nhỏ hơn, nên không cần tin client gửi đúng mode.
func TestOptLineArt_PhotoMisroutedFallsBackToLossy(t *testing.T) {
	// Ảnh nhiễu = nội dung nhiếp ảnh (worst case cho lossless).
	fh := newTestImageFile(t, "png", 900, 600, true)

	res, err := ProcessFile(t.Context(), fh, OptLineArt())
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	// Đối chứng: nếu ép lossless thuần thì to bao nhiêu.
	src, _, err := image.Decode(bytes.NewReader(encodeTestImage(t, "png", 900, 600, true)))
	if err != nil {
		t.Fatalf("decode gốc: %v", err)
	}
	losslessOnly, err := encodeWebPLossless(src)
	if err != nil {
		t.Fatalf("encodeWebPLossless: %v", err)
	}

	t.Logf("ảnh chụp gửi nhầm vào mode line art: lossless %dKB → chọn %dKB",
		len(losslessOnly)/1024, len(res.Data)/1024)
	if len(res.Data) >= len(losslessOnly) {
		t.Errorf("kết quả %d byte ≥ lossless thuần %d byte — không rơi về lossy khi lossless tệ hơn",
			len(res.Data), len(losslessOnly))
	}
}

// Line art vẫn phải THẮNG bằng lossless — cam kết pixel-exact giữ nguyên ở nơi nó có ý nghĩa.
func TestOptLineArt_StillLosslessForRealLineArt(t *testing.T) {
	src := signatureLikePNG(t, 600, 200)
	fh := newTestFileHeader(t, "signature.png", src)

	res, err := ProcessFile(t.Context(), fh, OptLineArt())
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	orig, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		t.Fatalf("decode gốc: %v", err)
	}
	out, _, err := image.Decode(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("decode kết quả: %v", err)
	}
	for y := 0; y < 200; y++ {
		for x := 0; x < 600; x++ {
			if orig.At(x, y) != out.At(x, y) {
				t.Fatalf("pixel (%d,%d) lệch — line art phải chọn lossless, không rơi về lossy", x, y)
			}
		}
	}
}

// IsImageUpload là bản lề rẽ trần dung lượng ở emr/cls_result — sai ở đây thì PDF kết quả CLS
// bị áp trần ảnh (20MB) và bị chặn oan.
func TestIsImageUpload(t *testing.T) {
	tests := []struct {
		name string
		fh   *multipart.FileHeader
		want bool
	}{
		{"PNG", newTestFileHeader(t, "a.png", encodeTestImage(t, "png", 40, 40, false)), true},
		{"JPEG", newTestFileHeader(t, "a.jpg", encodeTestImage(t, "jpeg", 40, 40, false)), true},
		{"PDF", newTestFileHeader(t, "a.pdf", []byte(pdfBody)), false},
		{"PDF đổi đuôi .jpg", newTestFileHeader(t, "a.jpg", []byte(pdfBody)), false},
		{"nil", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsImageUpload(tc.fh); got != tc.want {
				t.Errorf("IsImageUpload = %v, muốn %v", got, tc.want)
			}
		})
	}
}

func TestFilenameFromURL(t *testing.T) {
	tests := map[string]string{
		"https://cdn.x/develop/crm/a-b-1.pdf":                  "a-b-1.pdf",
		"https://cdn.x/develop/crm/a-b-1.pdf?X-Amz-Sig=abc123": "a-b-1.pdf",
		"https://cdn.x/develop/crm/a-b-1.pdf#page=2":           "a-b-1.pdf",
	}
	for url, want := range tests {
		if got := filenameFromURL(url); got != want {
			t.Errorf("filenameFromURL(%q) = %q, muốn %q", url, got, want)
		}
	}
}
