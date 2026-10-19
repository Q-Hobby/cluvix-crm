package imaging

// Test nền cho pkg/imaging — KHOÁ HÀNH VI ĐANG CÓ (characterization test).
//
// Mục đích: story-02/03/04 của epic image-pipeline-optimization sẽ đổi hành vi lõi của package.
// Trước đó package không có test nào, nên không có lưới an toàn để biết cái gì vỡ.
//
// QUAN TRỌNG: một số test ở đây khoá ĐÚNG NHỮNG BUG mà epic đi sửa (ảnh 4000×3000 nhưng ≤200KB
// được passthrough nguyên bản; .webp quá khổ passthrough). Chúng được đánh dấu
// `// SẼ ĐỔI Ở STORY-03` — story sau phải CẬP NHẬT kỳ vọng, không phải xoá test.

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math/rand"
	"mime/multipart"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gabriel-vasile/mimetype"

	"erpCluvix/pkg/errs"
	"erpCluvix/pkg/i18n"
)

// ── Helpers dựng ảnh test ────────────────────────────────────
//
// Không commit file ảnh nhị phân vào repo — mọi ảnh test được sinh trong code.

// newTestFileHeader bọc []byte thành *multipart.FileHeader hợp lệ (có .Size đúng và .Open() chạy).
// Đây là cách duy nhất lấy được FileHeader thật: dựng multipart body rồi đọc lại bằng ReadForm.
func newTestFileHeader(t *testing.T, filename string, data []byte) *multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	form, err := multipart.NewReader(&body, w.Boundary()).ReadForm(64 << 20)
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	t.Cleanup(func() { _ = form.RemoveAll() })

	fhs := form.File["file"]
	if len(fhs) != 1 {
		t.Fatalf("expected 1 file header, got %d", len(fhs))
	}
	if int(fhs[0].Size) != len(data) {
		t.Fatalf("FileHeader.Size = %d, muốn %d", fhs[0].Size, len(data))
	}
	return fhs[0]
}

// newTestImageFile sinh ảnh theo format/kích thước rồi bọc thành FileHeader.
// noisy = true → nội dung nhiễu ngẫu nhiên (nén kém → file to);
// noisy = false → gradient mượt (nén rất tốt → file nhỏ).
func newTestImageFile(t *testing.T, format string, w, h int, noisy bool) *multipart.FileHeader {
	t.Helper()
	data := encodeTestImage(t, format, w, h, noisy)
	return newTestFileHeader(t, "anh-test."+format, data)
}

// makeImage dựng ảnh NRGBA. Nhiễu dùng PRNG cố định seed để test tất định.
func makeImage(w, h int, noisy bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if noisy {
		rnd := rand.New(rand.NewSource(42))
		buf := img.Pix
		for i := 0; i < len(buf); i += 4 {
			buf[i] = uint8(rnd.Intn(256))
			buf[i+1] = uint8(rnd.Intn(256))
			buf[i+2] = uint8(rnd.Intn(256))
			buf[i+3] = 255
		}
		return img
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8((x * 255) / maxInt(w, 1)),
				G: uint8((y * 255) / maxInt(h, 1)),
				B: 128,
				A: 255,
			})
		}
	}
	return img
}

func encodeTestImage(t *testing.T, format string, w, h int, noisy bool) []byte {
	t.Helper()
	img := makeImage(w, h, noisy)

	var buf bytes.Buffer
	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 75}); err != nil {
			t.Fatalf("encode jpeg: %v", err)
		}
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			t.Fatalf("encode png: %v", err)
		}
	case "gif":
		if err := gif.Encode(&buf, img, nil); err != nil {
			t.Fatalf("encode gif: %v", err)
		}
	case "webp":
		data, err := encodeWebP(img, 80)
		if err != nil {
			t.Fatalf("encode webp: %v", err)
		}
		buf.Write(data)
	default:
		t.Fatalf("format không hỗ trợ trong test: %s", format)
	}
	return buf.Bytes()
}

// jpegUnderKB sinh JPEG có kích thước ≤ limitKB bằng cách hạ dần quality.
// Dùng cho test cần "ảnh pixel lớn nhưng file nhỏ" — đúng tình huống ảnh điện thoại nén tốt.
func jpegUnderKB(t *testing.T, w, h, limitKB int) []byte {
	t.Helper()
	img := makeImage(w, h, false)
	for _, q := range []int{75, 60, 45, 30, 20, 10} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
			t.Fatalf("encode jpeg q=%d: %v", q, err)
		}
		if buf.Len() <= limitKB*1024 {
			return buf.Bytes()
		}
	}
	t.Fatalf("không sinh được JPEG %dx%d dưới %dKB", w, h, limitKB)
	return nil
}

// animatedGIF dựng GIF nhiều frame để kiểm tra animation không bị mất.
func animatedGIF(t *testing.T, w, h, frames int) []byte {
	t.Helper()
	g := &gif.GIF{}
	pal := color.Palette{color.Black, color.White, color.RGBA{R: 255, A: 255}}
	for i := 0; i < frames; i++ {
		fr := image.NewPaletted(image.Rect(0, 0, w, h), pal)
		for p := range fr.Pix {
			fr.Pix[p] = uint8((p + i) % len(pal))
		}
		g.Image = append(g.Image, fr)
		g.Delay = append(g.Delay, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatalf("encode animated gif: %v", err)
	}
	return buf.Bytes()
}

// jpegWithOrientation nhét segment APP1/Exif chứa tag Orientation vào ngay sau SOI.
// goexif chỉ ĐỌC exif, không ghi — nên phải dựng tay TIFF tối thiểu (1 IFD entry).
func jpegWithOrientation(t *testing.T, w, h, orientation int) []byte {
	t.Helper()

	base := encodeTestImage(t, "jpeg", w, h, false)
	if len(base) < 2 || base[0] != 0xFF || base[1] != 0xD8 {
		t.Fatalf("JPEG sinh ra không bắt đầu bằng SOI")
	}

	// TIFF little-endian, IFD0 tại offset 8, đúng 1 entry: Orientation (0x0112), type SHORT, count 1.
	tiff := []byte{
		'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00,
		0x01, 0x00,
		0x12, 0x01,
		0x03, 0x00,
		0x01, 0x00, 0x00, 0x00,
		byte(orientation), 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segLen := len(payload) + 2 // độ dài kể cả 2 byte chính nó

	out := make([]byte, 0, len(base)+segLen+2)
	out = append(out, base[:2]...) // SOI
	out = append(out, 0xFF, 0xE1, byte(segLen>>8), byte(segLen))
	out = append(out, payload...)
	out = append(out, base[2:]...)
	return out
}

func decodeBounds(t *testing.T, data []byte) image.Rectangle {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode kết quả: %v", err)
	}
	return img.Bounds()
}

// ── AC9: applyDefaults ───────────────────────────────────────

func TestApplyDefaults_FillsExpectedValues(t *testing.T) {
	opts := &Options{}
	applyDefaults(opts)

	if opts.TargetKB != 160 {
		t.Errorf("TargetKB = %d, muốn 160", opts.TargetKB)
	}
	if opts.QMin != 60 {
		t.Errorf("QMin = %d, muốn 60", opts.QMin)
	}
	if opts.QMax != 92 {
		t.Errorf("QMax = %d, muốn 92", opts.QMax)
	}
	if opts.SkipBelowKB != 200 {
		t.Errorf("SkipBelowKB = %d, muốn 200", opts.SkipBelowKB)
	}
}

// SkipBelowKB = -1 nghĩa là "không bao giờ skip theo dung lượng" (dùng ở avatar khách hàng).
// applyDefaults chỉ điền khi == 0, nên -1 phải sống sót.
func TestApplyDefaults_KeepsNegativeSkipBelowKB(t *testing.T) {
	opts := &Options{SkipBelowKB: -1}
	applyDefaults(opts)

	if opts.SkipBelowKB != -1 {
		t.Errorf("SkipBelowKB = %d, muốn giữ nguyên -1", opts.SkipBelowKB)
	}
}

// ── AC10 + AC11: validate đầu vào ────────────────────────────

func TestProcessFile_RejectsDisallowedExtension(t *testing.T) {
	for _, name := range []string{"anh.svg", "malware.exe", "khong-co-duoi"} {
		t.Run(name, func(t *testing.T) {
			fh := newTestFileHeader(t, name, []byte("noi dung bat ky"))
			if _, err := ProcessFile(t.Context(), fh, nil); err == nil {
				t.Fatalf("muốn lỗi cho %q, nhận nil", name)
			}
		})
	}
}

func TestProcessFile_RejectsOversizeFile(t *testing.T) {
	// Kiểm tra size chạy TRƯỚC khi decode, nên nội dung không cần là ảnh hợp lệ.
	oversize := make([]byte, MaxUploadBytes+1)
	fh := newTestFileHeader(t, "qua-lon.jpg", oversize)

	_, err := ProcessFile(t.Context(), fh, nil)
	if err == nil {
		t.Fatal("muốn lỗi file quá lớn, nhận nil")
	}
	if !strings.Contains(err.Error(), "quá lớn") {
		t.Errorf("thông báo lỗi = %q, muốn nói về dung lượng", err.Error())
	}
}

// ── AC3: encodeWebPTargetSize ────────────────────────────────

func TestEncodeWebPTargetSize_RespectsTarget(t *testing.T) {
	img := makeImage(800, 600, true) // nhiễu → nén kém, buộc binary search phải hạ quality

	// Target phải KHẢ THI thì mới có quyền đòi "≤ target+10". Sàn khả thi = kích thước tại qMin.
	// Đo sàn rồi đặt target trên sàn một chút — nếu không, test đang kiểm một điều bất khả thi
	// (xem TestEncodeWebPTargetSize_UnreachableTarget_FallsBackToMidQuality).
	floor, err := encodeWebP(img, 60)
	if err != nil {
		t.Fatalf("encodeWebP(qMin): %v", err)
	}
	targetKB := len(floor)/1024 + 30

	data, err := encodeWebPTargetSize(img, targetKB, 60, 92)
	if err != nil {
		t.Fatalf("encodeWebPTargetSize: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("trả buffer rỗng")
	}
	if gotKB := len(data) / 1024; gotKB > targetKB+10 {
		t.Errorf("kích thước = %dKB, muốn ≤ %dKB (target %d + tolerance 10)", gotKB, targetKB+10, targetKB)
	}
	// Phải tận dụng ngân sách, không hạ quality quá tay xuống dưới sàn qMin.
	if len(data) < len(floor) {
		t.Errorf("kích thước = %d byte, nhỏ hơn cả mức tại qMin (%d) — quality đã ra ngoài [qMin,qMax]",
			len(data), len(floor))
	}
}

// Target rất lớn → binary search phải hội tụ về qMax (chất lượng cao nhất trong khoảng).
// Đây là cách quan sát gián tiếp "quality luôn nằm trong [qMin,qMax]".
func TestEncodeWebPTargetSize_ConvergesToQMaxWhenTargetHuge(t *testing.T) {
	img := makeImage(400, 300, false)

	got, err := encodeWebPTargetSize(img, 100_000, 60, 92)
	if err != nil {
		t.Fatalf("encodeWebPTargetSize: %v", err)
	}
	want, err := encodeWebP(img, 92)
	if err != nil {
		t.Fatalf("encodeWebP(92): %v", err)
	}
	if len(got) != len(want) {
		t.Errorf("kích thước = %d, muốn = kích thước tại qMax (%d)", len(got), len(want))
	}
}

// D8 ĐÃ VÁ Ở STORY-03. Nhánh không quality nào đạt target (`bestBuf == nil`).
//
// Trước: fallback về quality GIỮA KHOẢNG `(qMin+qMax)/2`. Đo thật với ảnh nhiễu 800×600:
// qMin(60) = 263KB, mid(76) = 296KB → lúc đang cố ép nhỏ nhất có thể lại trả file TO HƠN ~12%.
// Sau: fallback về qMin = kết quả nhỏ nhất có thể, tức gần target nhất.
func TestEncodeWebPTargetSize_UnreachableTarget_FallsBackToQMin(t *testing.T) {
	img := makeImage(800, 600, true)

	data, err := encodeWebPTargetSize(img, 1, 60, 92)
	if err != nil {
		t.Fatalf("encodeWebPTargetSize: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("target bất khả thi → vẫn phải trả dữ liệu, nhận buffer rỗng")
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("kết quả không decode được: %v", err)
	}

	atQMin, err := encodeWebP(img, 60)
	if err != nil {
		t.Fatalf("encodeWebP(qMin): %v", err)
	}
	atMid, err := encodeWebP(img, (60+92)/2)
	if err != nil {
		t.Fatalf("encodeWebP(mid): %v", err)
	}

	if len(data) != len(atQMin) {
		t.Errorf("kích thước = %d, muốn khớp qMin (%d) — fallback phải chọn mức NHỎ NHẤT", len(data), len(atQMin))
	}
	if len(atMid) > len(atQMin) && len(data) >= len(atMid) {
		t.Errorf("kích thước = %d ≥ mức giữa khoảng (%d) — nợ D8 đã tái phát", len(data), len(atMid))
	}
}

// ── Bảng quyết định passthrough (story-03: pixel-first) ──────

// ĐÃ ĐỔI Ở STORY-03 (trước đây test này khoá hành vi SAI: passthrough nguyên bản).
// Ảnh điện thoại 4000×3000 nén tốt (≤200KB) giờ PHẢI bị resize về hộp — đây là mục tiêu M1
// của epic. Trước story-03, FE phải tải và decode 12 triệu pixel để hiện avatar 40px.
func TestProcessFile_LargeDimensionSmallFile_IsResized(t *testing.T) {
	data := jpegUnderKB(t, 4000, 3000, 200)
	fh := newTestFileHeader(t, "anh-dien-thoai.jpg", data)

	res, err := ProcessFile(t.Context(), fh, nil)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if res.ContentType != "image/webp" {
		t.Errorf("ContentType = %q, muốn image/webp (phải bị xử lý dù file nhẹ)", res.ContentType)
	}
	b := decodeBounds(t, res.Data)
	if b.Dx() != 1280 {
		t.Errorf("chiều rộng = %d, muốn 1280 (hộp mặc định)", b.Dx())
	}
	if b.Dy() != 960 {
		t.Errorf("chiều cao = %d, muốn 960 (giữ tỷ lệ 4:3)", b.Dy())
	}
}

// Ảnh nhỏ + nhẹ + đúng định dạng → vẫn giữ nguyên. Story-03 KHÔNG được biến mọi ảnh thành WebP:
// re-encode ảnh đã tối ưu là mất chất vô ích.
func TestProcessFile_SmallLightImage_StillPassesThrough(t *testing.T) {
	data := jpegUnderKB(t, 800, 600, 200)
	fh := newTestFileHeader(t, "nho-nhe.jpg", data)

	res, err := ProcessFile(t.Context(), fh, nil)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if res.ContentType != "image/jpeg" {
		t.Errorf("ContentType = %q, muốn image/jpeg (vừa hộp + nhẹ → giữ nguyên)", res.ContentType)
	}
	if !bytes.Equal(res.Data, data) {
		t.Error("dữ liệu bị đổi, muốn giữ nguyên bản")
	}
}

// AC10 story-03 — LƯỚI CHẶN RỦI RO R2 (hồi quy 20 call site).
// Logo PNG nhỏ có nền trong suốt phải giữ nguyên: không re-encode, không mất kênh alpha.
func TestProcessFile_SmallTransparentLogo_PassesThroughIntact(t *testing.T) {
	logo := image.NewNRGBA(image.Rect(0, 0, 300, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 300; x++ {
			// Nửa trái đục, nửa phải TRONG SUỐT hoàn toàn.
			a := uint8(255)
			if x >= 150 {
				a = 0
			}
			logo.SetNRGBA(x, y, color.NRGBA{R: 200, G: 30, B: 60, A: a})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, logo); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	fh := newTestFileHeader(t, "logo.png", buf.Bytes())

	res, err := ProcessFile(t.Context(), fh, nil)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if res.ContentType != "image/png" {
		t.Fatalf("ContentType = %q, muốn image/png (logo nhỏ phải passthrough)", res.ContentType)
	}

	out, _, err := image.Decode(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("decode kết quả: %v", err)
	}
	if _, _, _, a := out.At(250, 100).RGBA(); a != 0 {
		t.Errorf("alpha tại điểm trong suốt = %d, muốn 0 (nền trong suốt bị mất)", a)
	}
	if _, _, _, a := out.At(50, 100).RGBA(); a == 0 {
		t.Error("alpha tại điểm đục = 0, ảnh bị hỏng")
	}
}

// NHÁNH GIAO NHAU: .gif + ModeFill — hai luật trong bảng quyết định xung đột.
// Giữ animation THẮNG cam kết kích thước, đúng thứ tự dòng trong bảng AC1 story-03.
// Hệ quả thấy được: upload avatar bằng ảnh GIF thì ảnh KHÔNG được crop về 500×500.
// Đây là hành vi có sẵn từ trước story-03 (nhánh .gif vốn đã passthrough), không phải hồi quy mới.
func TestProcessFile_GIFWithModeFill_KeepsAnimationNotDimensions(t *testing.T) {
	data := animatedGIF(t, 300, 200, 4)
	fh := newTestFileHeader(t, "avatar.gif", data)

	res, err := ProcessFile(t.Context(), fh, OptFill500())
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if res.ContentType != "image/gif" {
		t.Errorf("ContentType = %q, muốn image/gif (giữ animation thắng cam kết kích thước)", res.ContentType)
	}
	if b := decodeBounds(t, res.Data); b.Dx() != 300 || b.Dy() != 200 {
		t.Errorf("kích thước = %dx%d, muốn giữ 300x200 (KHÔNG crop về 500x500)", b.Dx(), b.Dy())
	}
}

// Ảnh trong hộp nhưng NẶNG (> SkipBelowKB) → bị xử lý, ra WebP.
// Hành vi này ĐÚNG và story-03 giữ nguyên.
func TestProcessFile_InBoxHeavyFile_IsProcessed(t *testing.T) {
	fh := newTestImageFile(t, "png", 1000, 800, true) // PNG nhiễu ≈ 2.4MB

	res, err := ProcessFile(t.Context(), fh, nil)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if res.ContentType != "image/webp" {
		t.Errorf("ContentType = %q, muốn image/webp", res.ContentType)
	}
	if !strings.HasSuffix(res.Filename, ".webp") {
		t.Errorf("Filename = %q, muốn đuôi .webp", res.Filename)
	}
	if len(res.Data) >= int(fh.Size) {
		t.Errorf("kết quả %d byte, muốn nhỏ hơn bản gốc %d byte", len(res.Data), fh.Size)
	}
}

// ── AC5: passthrough theo định dạng ──────────────────────────

// ĐÃ ĐỔI Ở STORY-03. Trước đây `.webp` passthrough VÔ ĐIỀU KIỆN, nên file .webp 6000px lọt
// nguyên. Nay .webp vượt hộp phải được decode + resize + re-encode.
func TestProcessFile_WebPOversize_IsResized(t *testing.T) {
	data := encodeTestImage(t, "webp", 3000, 2000, false)
	fh := newTestFileHeader(t, "anh-lon.webp", data)

	res, err := ProcessFile(t.Context(), fh, nil)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if b := decodeBounds(t, res.Data); b.Dx() > 1280 || b.Dy() > 1280 {
		t.Errorf("kích thước = %dx%d, muốn cạnh dài ≤ 1280", b.Dx(), b.Dy())
	}
}

// .webp ĐÃ vừa hộp → vẫn passthrough, tránh re-encode mất chất vô ích.
func TestProcessFile_WebPInBox_PassesThrough(t *testing.T) {
	data := encodeTestImage(t, "webp", 800, 600, false)
	fh := newTestFileHeader(t, "vua-hop.webp", data)

	res, err := ProcessFile(t.Context(), fh, nil)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if !bytes.Equal(res.Data, data) {
		t.Error("dữ liệu bị đổi, muốn passthrough nguyên bản")
	}
}

// ModeFill KHÔNG có đường giữ nguyên — kể cả file rất nhẹ.
// Trước story-03, `OptFill500()` để SkipBelowKB=0 → nhận default 200 → ảnh 3000px/150KB
// "passthrough" và KHÔNG hề là 500×500: cam kết kích thước bị phá âm thầm.
func TestProcessFile_ModeFill_LightFileStillCropped(t *testing.T) {
	data := jpegUnderKB(t, 3000, 2000, 200)
	fh := newTestFileHeader(t, "nhe.jpg", data)

	res, err := ProcessFile(t.Context(), fh, OptFill500())
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if b := decodeBounds(t, res.Data); b.Dx() != 500 || b.Dy() != 500 {
		t.Errorf("kích thước = %dx%d, muốn đúng 500x500 dù file nhẹ", b.Dx(), b.Dy())
	}
}

// GIF luôn passthrough để giữ animation — story-03 GIỮ NGUYÊN hành vi này.
// Bẫy: nếu ai đó bỏ passthrough cho .gif, image.Decode chỉ lấy frame đầu → mất animation
// im lặng mà build vẫn xanh. Test này là lưới bắt đúng lỗi đó.
func TestProcessFile_AnimatedGIF_KeepsAllFrames(t *testing.T) {
	const frames = 5
	data := animatedGIF(t, 300, 200, frames)
	fh := newTestFileHeader(t, "dong.gif", data)

	res, err := ProcessFile(t.Context(), fh, nil)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if res.ContentType != "image/gif" {
		t.Errorf("ContentType = %q, muốn image/gif", res.ContentType)
	}

	g, err := gif.DecodeAll(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("DecodeAll kết quả: %v", err)
	}
	if len(g.Image) != frames {
		t.Errorf("số frame = %d, muốn %d (animation phải còn nguyên)", len(g.Image), frames)
	}
}

// ModeRaw luôn passthrough, không decode.
func TestProcessFile_ModeRaw_PassesThrough(t *testing.T) {
	fh := newTestImageFile(t, "png", 1000, 800, true)

	res, err := ProcessFile(t.Context(), fh, OptRaw())
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if res.ContentType != "image/png" {
		t.Errorf("ContentType = %q, muốn image/png", res.ContentType)
	}
	if int64(len(res.Data)) != fh.Size {
		t.Errorf("kích thước = %d, muốn nguyên bản %d", len(res.Data), fh.Size)
	}
}

// ── Story-02: guard decompression bomb + HEIC ────────────────

// pngBomb dựng PNG mà HEADER khai báo kích thước khổng lồ nhưng FILE rất nhỏ.
// Đây đúng hình dạng của decompression bomb: image.DecodeConfig đọc được w×h từ IHDR,
// nhưng image.Decode sẽ cố cấp phát w*h*4 byte.
//
// Cách dựng: encode PNG hợp lệ nhỏ rồi GHI ĐÈ trường Width/Height trong chunk IHDR và tính lại
// CRC của chunk đó. IHDR luôn là chunk đầu, bắt đầu ở offset 8 (sau 8 byte PNG signature):
//
//	[0:8]   signature
//	[8:12]  length = 13
//	[12:16] type   = "IHDR"
//	[16:20] width  (big-endian)
//	[20:24] height (big-endian)
//	[24:29] bitDepth, colorType, compression, filter, interlace
//	[29:33] CRC của (type + data)
func pngBomb(t *testing.T, width, height uint32) []byte {
	t.Helper()

	data := encodeTestImage(t, "png", 8, 8, false)
	if len(data) < 33 {
		t.Fatalf("PNG sinh ra quá ngắn: %d byte", len(data))
	}
	if string(data[12:16]) != "IHDR" {
		t.Fatalf("chunk đầu không phải IHDR: %q", data[12:16])
	}

	out := make([]byte, len(data))
	copy(out, data)
	binary.BigEndian.PutUint32(out[16:20], width)
	binary.BigEndian.PutUint32(out[20:24], height)
	binary.BigEndian.PutUint32(out[29:33], crc32.ChecksumIEEE(out[12:29]))
	return out
}

// AC9 — bomb bị từ chối, và RAM không nhảy vọt (chứng minh guard chạy TRƯỚC khi decode).
func TestProcessFile_RejectsPNGBomb(t *testing.T) {
	data := pngBomb(t, 20000, 20000) // 400 MP ≈ 1.6GB nếu decode thật
	fh := newTestFileHeader(t, "bomb.png", data)

	if int(fh.Size) > 100*1024 {
		t.Fatalf("file bomb phải nhỏ, nhận %d byte — test không đúng hình dạng bomb", fh.Size)
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	_, err := ProcessFile(t.Context(), fh, nil)

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	if err == nil {
		t.Fatal("muốn từ chối ảnh bomb, nhận nil")
	}
	if !errors.Is(err, ErrImageDimensionExceeded) {
		t.Errorf("err = %v, muốn ErrImageDimensionExceeded", err)
	}

	// Guard chạy trước decode → không cấp phát pixel buffer. Ngưỡng 50MB để bắt lỗi "quên guard"
	// (khi đó sẽ tăng hàng trăm MB), không phải phép đo chính xác.
	if after.HeapAlloc > before.HeapAlloc {
		if grew := (after.HeapAlloc - before.HeapAlloc) / (1024 * 1024); grew > 50 {
			t.Errorf("HeapAlloc tăng %dMB, muốn < 50MB (guard phải chặn TRƯỚC khi decode)", grew)
		}
	}
}

// AC4 — vượt ngưỡng theo TỪNG CẠNH, dù tổng pixel vẫn dưới 50MP.
func TestProcessFile_RejectsOverlongSide(t *testing.T) {
	// 25000 × 100 = 2.5 MP (dưới MaxImagePixels) nhưng cạnh > MaxImageDimension.
	data := pngBomb(t, 25000, 100)
	fh := newTestFileHeader(t, "dai.png", data)

	_, err := ProcessFile(t.Context(), fh, nil)
	if !errors.Is(err, ErrImageDimensionExceeded) {
		t.Errorf("err = %v, muốn ErrImageDimensionExceeded (cạnh dài vượt ngưỡng)", err)
	}
}

// AC10 — không chặn nhầm ảnh thật. 4000×3000 = 12 MP, dưới ngưỡng.
func TestProcessFile_AcceptsRealisticLargePhoto(t *testing.T) {
	data := jpegUnderKB(t, 4000, 3000, 200)
	fh := newTestFileHeader(t, "that.jpg", data)

	if _, err := ProcessFile(t.Context(), fh, nil); err != nil {
		t.Fatalf("ảnh 4000x3000 hợp lệ bị từ chối: %v", err)
	}
}

// AC11 + AC12 — guard phủ CẢ các nhánh passthrough (gif / ModeRaw / webp / file nhỏ).
// Trước story-02, 3 nhánh này `return` sớm nên bomb đi thẳng lên S3.
func TestProcessFile_GuardCoversPassthroughBranches(t *testing.T) {
	bomb := pngBomb(t, 20000, 20000)

	tests := []struct {
		name     string
		filename string
		opts     *Options
	}{
		{"ModeRaw", "bomb.png", OptRaw()},
		{"file nhỏ (dưới SkipBelowKB)", "bomb.png", nil},
		{"ModeFill", "bomb.png", OptFill500()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fh := newTestFileHeader(t, tc.filename, bomb)
			_, err := ProcessFile(t.Context(), fh, tc.opts)
			if !errors.Is(err, ErrImageDimensionExceeded) {
				t.Errorf("err = %v, muốn ErrImageDimensionExceeded — nhánh này đang bỏ qua guard", err)
			}
		})
	}
}

// Nhánh .gif: GIF khai báo kích thước lớn phải bị chặn dù .gif luôn passthrough.
func TestProcessFile_GuardCoversGIFBranch(t *testing.T) {
	// GIF header: "GIF89a" + width(2, little-endian) + height(2, little-endian) + ...
	data := animatedGIF(t, 100, 100, 2)
	if string(data[:3]) != "GIF" {
		t.Fatalf("không phải GIF: %q", data[:3])
	}
	binary.LittleEndian.PutUint16(data[6:8], 30000)  // width
	binary.LittleEndian.PutUint16(data[8:10], 30000) // height

	fh := newTestFileHeader(t, "bomb.gif", data)
	_, err := ProcessFile(t.Context(), fh, nil)
	if !errors.Is(err, ErrImageDimensionExceeded) {
		t.Errorf("err = %v, muốn ErrImageDimensionExceeded — nhánh .gif đang bỏ qua guard", err)
	}
}

// AC5 — ProcessReader cũng phải qua guard.
func TestProcessReader_RejectsBomb(t *testing.T) {
	data := pngBomb(t, 20000, 20000)

	_, err := ProcessReader(t.Context(), bytes.NewReader(data), "bomb.png", nil)
	if !errors.Is(err, ErrImageDimensionExceeded) {
		t.Errorf("err = %v, muốn ErrImageDimensionExceeded", err)
	}
}

// AC13 — HEIC theo ĐUÔI FILE: phải ra lỗi riêng, KHÔNG phải lỗi ext chung.
func TestProcessFile_RejectsHEICByExtension(t *testing.T) {
	for _, name := range []string{"anh.heic", "anh.HEIF", "anh.heif"} {
		t.Run(name, func(t *testing.T) {
			fh := newTestFileHeader(t, name, []byte("noi dung bat ky"))

			_, err := ProcessFile(t.Context(), fh, nil)
			if !errors.Is(err, ErrHEICNotSupported) {
				t.Fatalf("err = %v, muốn ErrHEICNotSupported", err)
			}
			if strings.Contains(err.Error(), "Chỉ chấp nhận ảnh") {
				t.Error("rơi vào lỗi ext chung — phải là thông báo HEIC riêng")
			}
		})
	}
}

// HEIC đổi tên thành .jpg: đuôi qua được allowlist nên phải bắt bằng MAGIC BYTES.
// Nếu quên bước này, DecodeConfig trả "unknown format" — lẫn với file hỏng, sai thông điệp.
func TestProcessFile_RejectsHEICByMagicBytes(t *testing.T) {
	// ftyp box của HEIC: [size][ftyp][major_brand "heic"]...
	heic := []byte{
		0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'h', 'e', 'i', 'c',
		0x00, 0x00, 0x00, 0x00, 'h', 'e', 'i', 'c', 'm', 'i', 'f', '1',
	}
	if !isHEICMIME(mimetype.Detect(heic).String()) {
		t.Skipf("thư viện mimetype không nhận diện mẫu HEIC này (%s) — mẫu test cần cập nhật",
			mimetype.Detect(heic).String())
	}

	fh := newTestFileHeader(t, "gia-dang.jpg", heic)

	_, err := ProcessFile(t.Context(), fh, nil)
	if !errors.Is(err, ErrHEICNotSupported) {
		t.Errorf("err = %v, muốn ErrHEICNotSupported (bắt qua magic bytes)", err)
	}
}

// ĐƯỜNG PRODUCTION THẬT — ProcessAndUpload, không phải ProcessFile.
//
// QA story-02 phát hiện: `ProcessAndUpload` chạy `storage.ValidateFileType(file, ImageMIMETypes)`
// TRƯỚC `ProcessFile`. `image/heic` không nằm trong ImageMIMETypes → nếu không chặn HEIC sớm,
// user nhận thông báo CHUNG "loại file không được phép" thay vì hướng dẫn "đổi sang JPG",
// dù test ở tầng ProcessFile vẫn xanh. Đây là lớp bug "test đúng, production sai".
//
// s3c = nil là hợp lệ ở đây: mọi guard phải bắn TRƯỚC khi chạm S3, nên không cần S3 thật.
func TestProcessAndUpload_GuardsRunBeforeS3AndBeforeMIMEWhitelist(t *testing.T) {
	t.Run("HEIC theo đuôi file", func(t *testing.T) {
		fh := newTestFileHeader(t, "anh.heic", []byte("noi dung bat ky"))

		_, err := ProcessAndUpload(t.Context(), nil, fh, "test/", nil)
		if !errors.Is(err, ErrHEICNotSupported) {
			t.Fatalf("err = %v, muốn ErrHEICNotSupported", err)
		}
		if strings.Contains(err.Error(), "không được phép tải lên") {
			t.Error("rơi vào thông báo chung của ValidateFileType — guard HEIC chạy quá muộn")
		}
	})

	t.Run("HEIC đổi tên thành .jpg", func(t *testing.T) {
		heic := []byte{
			0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'h', 'e', 'i', 'c',
			0x00, 0x00, 0x00, 0x00, 'h', 'e', 'i', 'c', 'm', 'i', 'f', '1',
		}
		fh := newTestFileHeader(t, "gia-dang.jpg", heic)

		_, err := ProcessAndUpload(t.Context(), nil, fh, "test/", nil)
		if !errors.Is(err, ErrHEICNotSupported) {
			t.Errorf("err = %v, muốn ErrHEICNotSupported", err)
		}
	})

	t.Run("ảnh bomb", func(t *testing.T) {
		fh := newTestFileHeader(t, "bomb.png", pngBomb(t, 20000, 20000))

		_, err := ProcessAndUpload(t.Context(), nil, fh, "test/", nil)
		if !errors.Is(err, ErrImageDimensionExceeded) {
			t.Errorf("err = %v, muốn ErrImageDimensionExceeded", err)
		}
	})

	t.Run("ảnh hợp lệ đi tới bước S3 rồi mới lỗi vì s3c nil", func(t *testing.T) {
		// Chốt rằng guard KHÔNG chặn nhầm ảnh thật: phải đi qua hết guard, chỉ dừng ở S3.
		fh := newTestFileHeader(t, "that.png", encodeTestImage(t, "png", 900, 700, true))

		_, err := ProcessAndUpload(t.Context(), nil, fh, "test/", nil)
		if err == nil {
			t.Fatal("muốn lỗi S3 chưa cấu hình")
		}
		if errors.Is(err, ErrHEICNotSupported) || errors.Is(err, ErrImageDimensionExceeded) {
			t.Errorf("ảnh hợp lệ bị guard chặn nhầm: %v", err)
		}
	})
}

// Thông điệp lỗi phải đi qua i18n với ĐỦ tham số — không hardcode chuỗi trong package.
// Kiểm cả 4 ngôn ngữ để không sót file nào (Hard Rule i18n, CLAUDE.md).
func TestGuardErrors_AreLocalizedWithArgs(t *testing.T) {
	err := guardConfig(image.Config{Width: 20000, Height: 20000})

	var le *errs.LocalizedError
	if !errors.As(err, &le) {
		t.Fatalf("err = %T, muốn *errs.LocalizedError để HandleError dịch được", err)
	}
	if le.Key != "image_dimension_exceeded" {
		t.Errorf("Key = %q, muốn image_dimension_exceeded", le.Key)
	}
	if len(le.Args) != 4 {
		t.Fatalf("Args = %v, muốn đúng 4 tham số (w, h, maxMP, maxDim)", le.Args)
	}

	for _, locale := range []string{i18n.Vi, i18n.En} {
		t.Run(locale, func(t *testing.T) {
			for _, key := range []string{"image_dimension_exceeded", "image_heic_not_supported"} {
				if msg := i18n.TL(locale, key); msg == key {
					t.Errorf("key %q thiếu bản dịch cho locale %q", key, locale)
				}
			}
			msg := i18n.TLF(locale, "image_dimension_exceeded", le.Args...)
			if strings.Contains(msg, "%!") {
				t.Errorf("format sai (số tham số không khớp): %q", msg)
			}
		})
	}
}

// ── Story-04: hàng đợi huỷ được theo context ─────────────────

// AC6 — context đã huỷ → trả ctx.Err() ngay, KHÔNG treo.
// Trước story-04, `processSem <- struct{}{}` block vô hạn: client ngắt kết nối vẫn giữ chỗ.
func TestProcessFile_CancelledContext_ReturnsImmediately(t *testing.T) {
	fh := newTestImageFile(t, "png", 1000, 800, true) // đủ nặng để chắc chắn phải vào hàng đợi

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	done := make(chan error, 1)
	go func() { _, err := ProcessFile(ctx, fh, nil); done <- err }()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, muốn context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ProcessFile treo khi context đã huỷ — semaphore không nhận huỷ")
	}
}

func TestProcessReader_CancelledContext_ReturnsImmediately(t *testing.T) {
	data := encodeTestImage(t, "png", 800, 600, true)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	done := make(chan error, 1)
	go func() { _, err := ProcessReader(ctx, bytes.NewReader(data), "x.png", nil); done <- err }()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, muốn context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ProcessReader treo khi context đã huỷ")
	}
}

// AC7 — KHÔNG RÒ RỈ SLOT.
//
// Bẫy: nếu `defer releaseSlot()` được đăng ký TRƯỚC lời gọi acquire, nhánh ctx.Done() sẽ nhả một
// slot chưa hề chiếm → semaphore phình ra, mất tác dụng giới hạn đồng thời. Build vẫn xanh.
//
// Cách bắt: chiếm sạch 4 slot, gọi ProcessFile với ctx đã huỷ nhiều lần, rồi kiểm sức chứa còn
// nguyên 4 — nếu bị rò, số slot chiếm được sẽ > 4.
func TestAcquireSlot_CancelledDoesNotLeakSlot(t *testing.T) {
	capacity := cap(processSem)
	if capacity != 4 {
		t.Fatalf("sức chứa processSem = %d, muốn 4 (AC2: không được tăng)", capacity)
	}

	// Chiếm sạch hàng đợi.
	for i := 0; i < capacity; i++ {
		if err := acquireSlot(t.Context()); err != nil {
			t.Fatalf("acquireSlot lần %d: %v", i, err)
		}
	}

	// Mọi lời gọi với ctx đã huỷ phải trả lỗi ngay và KHÔNG nhả slot của người khác.
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	for i := 0; i < 10; i++ {
		if err := acquireSlot(cancelled); !errors.Is(err, context.Canceled) {
			t.Fatalf("acquireSlot lần %d với ctx huỷ: err = %v, muốn context.Canceled", i, err)
		}
	}

	// Hàng đợi vẫn phải ĐẦY (không ai được thêm slot nhờ nhánh huỷ).
	if got := len(processSem); got != capacity {
		t.Errorf("số slot đang chiếm = %d, muốn %d — nhánh ctx.Done() đã nhả slot chưa chiếm", got, capacity)
	}

	// Trả lại slot; sau đó hàng đợi phải dùng lại được bình thường.
	for i := 0; i < capacity; i++ {
		releaseSlot()
	}
	if got := len(processSem); got != 0 {
		t.Fatalf("sau khi trả hết, còn %d slot bị giữ", got)
	}
	if err := acquireSlot(t.Context()); err != nil {
		t.Fatalf("hàng đợi không dùng lại được sau khi huỷ: %v", err)
	}
	releaseSlot()
}

// ctx nil không được panic (call site cũ/quên truyền).
func TestProcessFile_NilContext_DoesNotPanic(t *testing.T) {
	fh := newTestImageFile(t, "png", 400, 300, true)

	//nolint:staticcheck // cố ý truyền nil để kiểm guard orBackground
	if _, err := ProcessFile(nil, fh, nil); err != nil {
		t.Fatalf("ProcessFile với ctx nil: %v", err)
	}
}

// ── ProcessReader: đường vào công khai THỨ HAI ───────────────
//
// Story-03 đổi bảng quyết định passthrough và story-04 đổi signature + semaphore của hàm này.
// Không có baseline thì hai story đó sửa mà không lưới nào bắt (QA story-01 phát hiện, 2026-08-13).

// ProcessReader KHÔNG có bất kỳ nhánh passthrough nào — luôn decode → resize → WebP.
// Khác hẳn ProcessFile (có 4 nhánh passthrough). Story-03 phải giữ khác biệt này có chủ đích
// vì ProcessReader không có `file.Size` để áp SkipBelowKB.
func TestProcessReader_AlwaysProcesses(t *testing.T) {
	data := jpegUnderKB(t, 400, 300, 200) // nhỏ và nhẹ — ProcessFile sẽ passthrough

	res, err := ProcessReader(t.Context(), bytes.NewReader(data), "nho.jpg", nil)
	if err != nil {
		t.Fatalf("ProcessReader: %v", err)
	}
	if res.ContentType != "image/webp" {
		t.Errorf("ContentType = %q, muốn image/webp (ProcessReader luôn xử lý)", res.ContentType)
	}
	if !strings.HasSuffix(res.Filename, ".webp") {
		t.Errorf("Filename = %q, muốn đuôi .webp", res.Filename)
	}
}

// KHOÁ BẪY ĐÃ BIẾT (QA story-01 phát hiện):
// `processReader` chỉ phân biệt ModeFill vs default — ModeRaw RƠI VÀO `default:` và bị xử lý như
// ModeFit. Nghĩa là ProcessReader(r, name, OptRaw()) vẫn RE-ENCODE sang WebP, trái hẳn ý nghĩa
// "Raw = giữ nguyên 100%" mà OptRaw cam kết ở ProcessFile.
// Đây là bẫy im lặng: gọi đúng preset mà vẫn mất bản gốc. → deferred-work.md D9.
func TestProcessReader_ModeRawIsIgnored_KnownTrap(t *testing.T) {
	data := encodeTestImage(t, "png", 400, 300, true)

	res, err := ProcessReader(t.Context(), bytes.NewReader(data), "raw.png", OptRaw())
	if err != nil {
		t.Fatalf("ProcessReader: %v", err)
	}
	if res.ContentType != "image/webp" {
		t.Fatalf("ContentType = %q — hành vi đã đổi, xem lại D9 trong deferred-work.md", res.ContentType)
	}
	t.Log("BẪY ĐÃ BIẾT (D9): ModeRaw bị bỏ qua trong ProcessReader, ảnh vẫn bị re-encode sang WebP")
}

func TestProcessReader_ModeFillExactDimensions(t *testing.T) {
	data := encodeTestImage(t, "png", 1200, 900, true)

	res, err := ProcessReader(t.Context(), bytes.NewReader(data), "fill.png", OptFill500())
	if err != nil {
		t.Fatalf("ProcessReader: %v", err)
	}
	if b := decodeBounds(t, res.Data); b.Dx() != 500 || b.Dy() != 500 {
		t.Errorf("kích thước = %dx%d, muốn đúng 500x500", b.Dx(), b.Dy())
	}
}

// ── Preset: khoá giá trị để không trôi âm thầm ───────────────
//
// Story-06 thêm OptMedical() và story-03 đụng bảng quyết định — preset trôi giá trị là lỗi
// không ai thấy cho tới lúc ảnh production sai kích thước.

func TestPresets_HaveExpectedValues(t *testing.T) {
	tests := []struct {
		name string
		got  *Options
		want Options
	}{
		{"OptFill500", OptFill500(), Options{Mode: ModeFill, MaxWidth: 500, MaxHeight: 500, TargetKB: 100, Sharpen: true}},
		{"OptFill200", OptFill200(), Options{Mode: ModeFill, MaxWidth: 200, MaxHeight: 200, TargetKB: 50, Sharpen: true}},
		{"OptRaw", OptRaw(), Options{Mode: ModeRaw}},
		{"OptHighQuality", OptHighQuality(), Options{Mode: ModeFit, MaxWidth: 1920, MaxHeight: 1080, TargetKB: 300, QMin: 70, QMax: 95, Sharpen: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if *tc.got != tc.want {
				t.Errorf("preset = %+v, muốn %+v", *tc.got, tc.want)
			}
		})
	}
}

// ── AC6: ModeFill ────────────────────────────────────────────

func TestProcessFile_ModeFill_ExactDimensions(t *testing.T) {
	fh := newTestImageFile(t, "png", 1200, 900, true) // nhiễu → vượt SkipBelowKB, chắc chắn bị xử lý

	res, err := ProcessFile(t.Context(), fh, OptFill500())
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if b := decodeBounds(t, res.Data); b.Dx() != 500 || b.Dy() != 500 {
		t.Errorf("kích thước = %dx%d, muốn đúng 500x500", b.Dx(), b.Dy())
	}
}

// ── AC7: EXIF orientation ────────────────────────────────────

func TestDecodeWithOrientation_RotatesBySwapping(t *testing.T) {
	// Orientation 5-8 xoay 90°/270° → chiều rộng và chiều cao HOÁN ĐỔI.
	for _, ori := range []int{5, 6, 7, 8} {
		t.Run("orientation_"+string(rune('0'+ori)), func(t *testing.T) {
			data := jpegWithOrientation(t, 400, 200, ori)

			img, err := decodeWithOrientation(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("decodeWithOrientation: %v", err)
			}
			b := img.Bounds()
			if b.Dx() != 200 || b.Dy() != 400 {
				t.Errorf("kích thước = %dx%d, muốn 200x400 (hoán đổi sau khi xoay)", b.Dx(), b.Dy())
			}
		})
	}
}

func TestDecodeWithOrientation_KeepsDimensionsForFlipAndRotate180(t *testing.T) {
	// Orientation 1-4 (giữ nguyên / lật / xoay 180°) → kích thước KHÔNG đổi.
	for _, ori := range []int{1, 2, 3, 4} {
		t.Run("orientation_"+string(rune('0'+ori)), func(t *testing.T) {
			data := jpegWithOrientation(t, 400, 200, ori)

			img, err := decodeWithOrientation(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("decodeWithOrientation: %v", err)
			}
			b := img.Bounds()
			if b.Dx() != 400 || b.Dy() != 200 {
				t.Errorf("kích thước = %dx%d, muốn giữ 400x200", b.Dx(), b.Dy())
			}
		})
	}
}

func TestDecodeWithOrientation_NoExifStillDecodes(t *testing.T) {
	// Ảnh không có EXIF: exif.Decode fail, nhưng reader phải được Seek(0) để Decode vẫn chạy.
	data := encodeTestImage(t, "png", 120, 80, false)

	img, err := decodeWithOrientation(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decodeWithOrientation không EXIF: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 120 || b.Dy() != 80 {
		t.Errorf("kích thước = %dx%d, muốn 120x80", b.Dx(), b.Dy())
	}
}

// ── AC8: resizeFitBox không upscale ──────────────────────────

func TestResizeFitBox_DoesNotUpscale(t *testing.T) {
	img := makeImage(300, 200, false)
	opts := &Options{Mode: ModeFit, MaxWidth: 1280, MaxHeight: 1280}

	out := resizeFitBox(img, opts)

	if b := out.Bounds(); b.Dx() != 300 || b.Dy() != 200 {
		t.Errorf("kích thước = %dx%d, muốn giữ 300x200 (không upscale)", b.Dx(), b.Dy())
	}
}

func TestResizeFitBox_ShrinksToBoxKeepingRatio(t *testing.T) {
	img := makeImage(4000, 2000, false)
	opts := &Options{Mode: ModeFit, MaxWidth: 1280, MaxHeight: 1280}

	out := resizeFitBox(img, opts)

	b := out.Bounds()
	if b.Dx() != 1280 {
		t.Errorf("chiều rộng = %d, muốn 1280", b.Dx())
	}
	if b.Dy() != 640 {
		t.Errorf("chiều cao = %d, muốn 640 (giữ tỷ lệ 2:1)", b.Dy())
	}
}

// ── Phụ: tên file sinh ra ────────────────────────────────────

func TestGenFilename_UsesRequestedExtension(t *testing.T) {
	name := genFilename("Ảnh Của Tôi.JPG", ".webp")

	if filepath.Ext(name) != ".webp" {
		t.Errorf("đuôi = %q, muốn .webp", filepath.Ext(name))
	}
	if strings.ContainsAny(name, " /\\") {
		t.Errorf("tên file %q chứa ký tự không an toàn", name)
	}
	// genFilename có time.Now().Unix() + RandomString → chỉ kiểm hình dạng, không so khớp tuyệt đối.
	if !strings.Contains(name, "-") {
		t.Errorf("tên file %q không theo dạng <alias>-<random>-<unix>.<ext>", name)
	}
}

// ── Neo cho story-02: RAM khi gặp ảnh dị thường ──────────────

// Story-02 sẽ thêm guard pixel (MaxImagePixels / MaxImageDimension) và bật ngưỡng RAM trong test
// riêng. Ở đây chỉ ghi nhận mốc đo hiện tại để so sánh — KHÔNG assert ngưỡng, vì hành vi hiện tại
// là decode toàn ảnh (đúng cái story-02 đi sửa).
func TestProcessFile_MemoryFootprintBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("bỏ qua ở chế độ -short")
	}

	fh := newTestImageFile(t, "png", 2000, 1500, true)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	if _, err := ProcessFile(t.Context(), fh, nil); err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	t.Logf("HeapAlloc trước=%dKB sau=%dKB", before.HeapAlloc/1024, after.HeapAlloc/1024)
}

// LƯỚI CHO RỦI RO R2 — trước 2026-08-14 `architecture.md` chỉ GIẢ ĐỊNH "WebP lossy có hỗ trợ alpha
// nên nền trong suốt được giữ". Test `TestProcessFile_SmallTransparentLogo_PassesThroughIntact`
// KHÔNG kiểm được điều đó: logo nhỏ đi nhánh passthrough nên không hề bị re-encode.
//
// Test này ép ảnh ĐỦ TO để buộc đi nhánh lossy re-encode, rồi kiểm alpha còn nguyên.
func TestProcessFile_LargeTransparentImage_KeepsAlphaThroughLossyReencode(t *testing.T) {
	const w, h = 2000, 1200
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := uint8(255)
			if x >= w/2 {
				a = 0 // nửa phải trong suốt hoàn toàn
			}
			m.SetNRGBA(x, y, color.NRGBA{R: uint8(x % 251), G: uint8(y % 241), B: 90, A: a})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	fh := newTestFileHeader(t, "logo-lon.png", buf.Bytes())

	res, err := ProcessFile(t.Context(), fh, nil)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}
	if res.ContentType != "image/webp" {
		t.Fatalf("ContentType = %q — ảnh đi nhánh passthrough, test không kiểm được lossy re-encode", res.ContentType)
	}

	out, _, err := image.Decode(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("decode kết quả: %v", err)
	}
	b := out.Bounds()
	if _, _, _, a := out.At(b.Min.X+b.Dx()*3/4, b.Min.Y+b.Dy()/2).RGBA(); a != 0 {
		t.Errorf("alpha vùng trong suốt = %d, muốn 0 — MẤT NỀN TRONG SUỐT qua lossy re-encode", a)
	}
	if _, _, _, a := out.At(b.Min.X+b.Dx()/4, b.Min.Y+b.Dy()/2).RGBA(); a == 0 {
		t.Error("alpha vùng đục = 0 — ảnh bị hỏng")
	}
}
