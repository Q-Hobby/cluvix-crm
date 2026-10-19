package imaging

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "image/gif"
	"image/jpeg"
	"image/png"

	"github.com/disintegration/imaging"
	"github.com/gabriel-vasile/mimetype"
	gowebp "github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
	"github.com/rwcarlsen/goexif/exif"
	_ "golang.org/x/image/webp"

	"erpCluvix/pkg/errs"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/storage"
)

// ── Resize modes ─────────────────────────────────────────────

type ResizeMode int

const (
	// ModeFit giữ tỷ lệ gốc, fit vào MaxWidth×MaxHeight, không upscale.
	// Dùng cho: ảnh thường (service, branch, news, zalo...)
	// Yii2 tương đương: handleImageUploadWithRatio
	ModeFit ResizeMode = iota

	// ModeFill crop center đúng MaxWidth×MaxHeight.
	// Dùng cho: avatar, coupon image, icon — cần kích thước chính xác.
	// Yii2 tương đương: handleImageUpload(field, path, 500, 500)
	ModeFill

	// ModeRaw upload gốc, chỉ validate extension + size, không decode/resize/encode.
	// Dùng cho: referral card template, file cần giữ nguyên 100%.
	ModeRaw
)

// ── Concurrency ──────────────────────────────────────────────

var processSem = make(chan struct{}, 4)

// acquireSlot chờ slot xử lý ảnh, nhưng BỎ CUỘC khi context bị huỷ.
//
// Trước 2026-08-13 chỗ này là `processSem <- struct{}{}` block VÔ HẠN: client đã ngắt kết nối vẫn
// giữ chỗ trong hàng đợi 4 slot và chiếm connection gin.
//
// defer releaseSlot() chỉ được đăng ký SAU khi hàm này trả nil — đặt defer trước lời gọi sẽ nhả
// một slot chưa hề chiếm và làm hỏng semaphore (lỗi im lặng).
func acquireSlot(ctx context.Context) error {
	// Kiểm ctx TRƯỚC select là bắt buộc, không phải tối ưu: khi hàng đợi còn chỗ thì CẢ HAI case
	// của select đều sẵn sàng, và Go chọn NGẪU NHIÊN giữa các case sẵn sàng. Không có dòng này,
	// request đã huỷ vẫn có xác suất được nhận việc — và test cũng đỏ/xanh thất thường.
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case processSem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseSlot() { <-processSem }

// orBackground — call site truyền nil ctx sẽ làm select trên ctx.Done() panic.
func orBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// ── Validation ───────────────────────────────────────────────

var allowedExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}

// heicExts — định dạng ảnh iPhone. Không có decoder thuần Go, và trình duyệt desktop KHÔNG hiển
// thị được → phải từ chối TƯỜNG MINH (thông báo riêng), không để rơi vào lỗi ext chung.
var heicExts = map[string]bool{".heic": true, ".heif": true}

const MaxUploadBytes = 10 * 1024 * 1024

// ── Guard decompression bomb ─────────────────────────────────
//
// File nén hợp lệ vài MB có thể giải nén thành hàng GB pixel (PNG 20000×20000 ≈ 1.6GB RAM).
// Nhân với sức chứa processSem → OOM cả process chỉ từ một quyền upload avatar.
// Vì vậy MỌI đường vào phải đọc HEADER trước (image.DecodeConfig — không cấp phát pixel buffer)
// và từ chối trước khi decode thật.
//
// Ngưỡng do user chốt 2026-08-13 (brief.md Q3): rộng rãi để không chặn nhầm ảnh máy ảnh chuyên
// nghiệp, vẫn đủ chặn bomb thực tế (bomb thường 200MP–1GP).
const (
	MaxImagePixels    = 50_000_000 // 50 megapixel
	MaxImageDimension = 20_000     // px, áp cho từng cạnh
)

// Sentinel cho errors.Is. Thông điệp hiển thị đi qua errs.LocalizedError + i18n (xem guardConfig).
var (
	ErrImageDimensionExceeded = errors.New("image_dimension_exceeded")
	ErrHEICNotSupported       = errors.New("image_heic_not_supported")
)

func heicError() error {
	return errs.NewLocalizedErrorWrapping("image_heic_not_supported", ErrHEICNotSupported)
}

func isHEICMIME(mime string) bool {
	return strings.HasPrefix(mime, "image/heic") || strings.HasPrefix(mime, "image/heif")
}

// probeConfig đọc HEADER ảnh (không giải nén pixel) rồi tua reader về đầu cho bước decode thật.
func probeConfig(r io.ReadSeeker) (image.Config, string, error) {
	cfg, format, err := image.DecodeConfig(r)
	if _, seekErr := r.Seek(0, io.SeekStart); seekErr != nil {
		return image.Config{}, "", seekErr
	}
	if err != nil {
		return image.Config{}, "", err
	}
	return cfg, format, nil
}

// guardConfig từ chối ảnh vượt ngưỡng. Thông điệp nêu RÕ giới hạn để user biết đường xử lý
// (ảnh scan độ phân giải rất cao có thể bị chặn — rủi ro R6 trong architecture.md).
func guardConfig(cfg image.Config) error {
	tooManyPixels := int64(cfg.Width)*int64(cfg.Height) > MaxImagePixels
	tooWide := cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension
	if tooManyPixels || tooWide {
		return errs.NewLocalizedErrorWrapping("image_dimension_exceeded", ErrImageDimensionExceeded,
			cfg.Width, cfg.Height, MaxImagePixels/1_000_000, MaxImageDimension)
	}
	return nil
}

// probeAndGuard = probe header + guard, dùng chung cho cả 2 đường vào công khai.
// Trả kèm image.Config để caller quyết định passthrough mà không phải probe lần nữa.
// Reader được tua về đầu trước khi trả về, kể cả khi lỗi.
func probeAndGuard(r io.ReadSeeker) (image.Config, error) {
	cfg, _, err := probeConfig(r)
	if err != nil {
		return image.Config{}, fmt.Errorf("decode image: %w", err)
	}
	if err := guardConfig(cfg); err != nil {
		return image.Config{}, err
	}
	return cfg, nil
}

// guardHEIC từ chối HEIC/HEIF theo ĐUÔI FILE và theo MAGIC BYTES (file đổi tên thành .jpg).
//
// PHẢI gọi được độc lập vì `ProcessAndUpload` chạy storage.ValidateFileType TRƯỚC ProcessFile:
// image/heic không nằm trong storage.ImageMIMETypes nên nếu không chặn sớm, user nhận thông báo
// chung "loại file không được phép" thay vì hướng dẫn "đổi sang JPG".
func guardHEIC(file *multipart.FileHeader) error {
	if heicExts[strings.ToLower(filepath.Ext(file.Filename))] {
		return heicError()
	}

	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer src.Close()

	// image.DecodeConfig trả "unknown format" cho HEIC — không phân biệt được với file hỏng,
	// nên phải nhận diện bằng magic bytes.
	head := make([]byte, 512)
	n, _ := io.ReadFull(src, head)
	if isHEICMIME(mimetype.Detect(head[:n]).String()) {
		return heicError()
	}
	return nil
}

// guardUploadedFile = chặn HEIC + probe header + guard ngưỡng pixel. Dùng cho đường vào ProcessFile.
// Trả image.Config để ProcessFile dùng lại cho bảng quyết định passthrough.
func guardUploadedFile(file *multipart.FileHeader, heicChecked bool) (image.Config, error) {
	if !heicChecked {
		if err := guardHEIC(file); err != nil {
			return image.Config{}, err
		}
	}

	src, err := file.Open()
	if err != nil {
		return image.Config{}, fmt.Errorf("open file: %w", err)
	}
	defer src.Close()

	return probeAndGuard(src)
}

// ── Options ──────────────────────────────────────────────────

type Options struct {
	Mode        ResizeMode // Fit | Fill | Raw (default Fit)
	MaxWidth    int        // Fit: max box / Fill: exact width (default 1280)
	MaxHeight   int        // Fit: max box / Fill: exact height (default 1280)
	TargetKB    int        // target WebP file size KB (default 160)
	QMin        int        // WebP quality min (default 60)
	QMax        int        // WebP quality max (default 92)
	Sharpen     bool       // sharpen after resize (default true)
	Scale       float64    // Fit mode: scale when MaxW/MaxH=0
	Lossless    bool       // WebP lossless — cho LINE ART (chữ ký, sơ đồ nét); KHÔNG dùng cho ảnh chụp
	SkipBelowKB int        // skip processing if file ≤ KB (default 200, -1 = never skip)
}

var defaultOpts = Options{
	Mode: ModeFit, MaxWidth: 1280, MaxHeight: 1280,
	TargetKB: 160, QMin: 60, QMax: 92,
	Sharpen: true, SkipBelowKB: 200,
}

// ── Presets ──────────────────────────────────────────────────

// Preset cho avatar, coupon image — crop center 500×500
func OptFill500() *Options {
	return &Options{Mode: ModeFill, MaxWidth: 500, MaxHeight: 500, TargetKB: 100, Sharpen: true}
}

// OptAvatar — preset cho AVATAR NGƯỜI (nhân viên, khách hàng): crop center vuông
// 500×500, luôn xử lý, không bao giờ giữ nguyên bản.
//
// Vì sao tách khỏi OptFill500 (cùng thông số hình học): OptFill500 còn dùng cho
// coupon image, đổi nó là đổi ngoài phạm vi avatar. Vì sao KHÔNG dùng default
// (`nil`): default là ModeFit 1280 — ảnh giữ tỉ lệ gốc, mọi chỗ hiển thị avatar
// đều là khung tròn/vuông (web `p-avatar`, mobile `BoxFit.cover` trong
// `BoxShape.circle`) nên client phải crop lại và tải thừa pixel; tệ hơn, ảnh
// điện thoại (≤1024px, ~150-250KB) rơi đúng ranh giới `SkipBelowKB=200` ⇒ cùng
// một luồng upload mà ảnh này giữ JPEG gốc, ảnh kia thành WebP, tuỳ nội dung.
// ModeFill loại bỏ cả hai vấn đề: `shouldPassthrough` luôn trả false cho Fill
// nên output LUÔN là WebP 500×500 (SkipBelowKB:-1 chỉ để nói rõ chủ đích).
func OptAvatar() *Options {
	return &Options{Mode: ModeFill, MaxWidth: 500, MaxHeight: 500, TargetKB: 100, Sharpen: true, SkipBelowKB: -1}
}

// Preset cho avatar thumbnail nhỏ — crop center 200×200
func OptFill200() *Options {
	return &Options{Mode: ModeFill, MaxWidth: 200, MaxHeight: 200, TargetKB: 50, Sharpen: true}
}

// Preset cho referral card template — giữ nguyên gốc
func OptRaw() *Options {
	return &Options{Mode: ModeRaw}
}

// OptLineArt — preset cho LINE ART: chữ ký vẽ tay trên canvas, sơ đồ nét mảnh, ảnh có nền trong suốt.
//
// Nén LOSSLESS, không phải quality cao. Lý do: WebP lossy lượng tử hoá theo khối DCT nên sinh
// ringing quanh nét mảnh — ở quality nào cũng làm chữ ký bị rỗ/đứt nét. Line art ít màu nên
// lossless lại cho file NHỎ HƠN cả JPEG/PNG gốc.
//
// ⚠ KHÔNG dùng cho ảnh CHỤP (scan giấy, ảnh điện thoại): nội dung nhiếp ảnh có nhiễu/gradient,
// lossless sẽ cho file TO HƠN nhiều lần. Ảnh chụp giấy → OptMedical().
//
// Hộp 1600 chỉ để chặn ảnh quá khổ; canvas chữ ký thường ~600×200 nên không bị resize (không upscale).
func OptLineArt() *Options {
	return &Options{
		Mode: ModeFit, MaxWidth: 1600, MaxHeight: 1600,
		Lossless: true, Sharpen: false, SkipBelowKB: -1,
		// Ngân sách lossy dự phòng khi ảnh gửi vào KHÔNG phải line art (xem encodeWebPPreferLossless).
		// Đặt rộng như preset y tế, KHÔNG dùng mặc định 160KB.
		TargetKB: 700, QMin: 88, QMax: 95,
	}
}

// OptMedical — preset cho ẢNH Y TẾ (ảnh liệu trình before/after, kết quả cận lâm sàng).
//
// User chốt 2026-08-13 (brief.md Q1): chấp nhận nén lossy để tiết kiệm ~85-91% dung lượng,
// đổi lại phải giữ được chi tiết ở mức phóng to 100%.
//
// Ba điểm KHÁC mọi preset khác, đều có chủ đích — đừng "sửa cho nhất quán":
//  1. ModeFit (không crop): ảnh lâm sàng không được cắt mất rìa tổn thương.
//  2. Q 88-95 + TargetKB 700: ngân sách dung lượng rộng hơn nhiều preset thường (160KB).
//  3. Sharpen: false — sharpen tạo CẠNH GIẢ trên ảnh chẩn đoán; artifact đó có thể bị đọc nhầm
//     là ranh giới tổn thương. Đây là ảnh dùng làm bằng chứng lâm sàng, không phải ảnh minh hoạ.
//
// SkipBelowKB thấp: ảnh y tế vốn đã nhỏ thì giữ nguyên bản, không đụng vào.
func OptMedical() *Options {
	return &Options{
		Mode: ModeFit, MaxWidth: 2048, MaxHeight: 2048,
		TargetKB: 700, QMin: 88, QMax: 95,
		Sharpen: false, SkipBelowKB: 100,
	}
}

// Preset cho logo/icon company — giữ tỷ lệ, chất lượng cao
func OptHighQuality() *Options {
	return &Options{Mode: ModeFit, MaxWidth: 1920, MaxHeight: 1080, TargetKB: 300, QMin: 70, QMax: 95, Sharpen: true}
}

// ── Result ───────────────────────────────────────────────────

type ProcessResult struct {
	Data        []byte
	ContentType string
	Filename    string
}

// ── Public API ───────────────────────────────────────────────

func ProcessFile(ctx context.Context, file *multipart.FileHeader, opts *Options) (*ProcessResult, error) {
	return processFile(ctx, file, opts, false)
}

// processFile — bản nội bộ có cờ heicChecked.
//
// `ProcessAndUpload` đã gọi guardHEIC trước ValidateFileType (bắt buộc, xem ở đó), nên nếu
// ProcessFile guard lại thì mỗi upload tốn thêm 1 lần file.Open() + đọc 512B + mimetype.Detect
// — công việc thừa nằm TRONG slot semaphore, giữ chỗ lâu hơn cần.
func processFile(ctx context.Context, file *multipart.FileHeader, opts *Options, heicChecked bool) (*ProcessResult, error) {
	ctx = orBackground(ctx)
	if opts == nil {
		opts = new(defaultOpts)
	}
	applyDefaults(opts)

	ext := strings.ToLower(filepath.Ext(file.Filename))
	// HEIC phải kiểm TRƯỚC allowlist: .heic không nằm trong allowedExts nên nếu để rơi xuống
	// dưới, user nhận lỗi ext chung chung thay vì thông báo riêng "đổi sang JPG".
	if heicExts[ext] {
		return nil, heicError()
	}
	if !allowedExts[ext] {
		return nil, fmt.Errorf("Chỉ chấp nhận ảnh: jpg, png, gif, webp")
	}
	if file.Size > MaxUploadBytes {
		return nil, fmt.Errorf("File quá lớn, tối đa %dMB", MaxUploadBytes/1024/1024)
	}

	// ── Guard decompression bomb ──
	// PHẢI nằm trên MỌI nhánh passthrough bên dưới (ModeRaw / file nhỏ / webp / gif): trước đây
	// 3 nhánh đó `return` sớm nên ảnh bomb đi thẳng lên S3 mà không ai nhìn vào header.
	// cfg (kích thước pixel) được dùng lại cho quyết định passthrough bên dưới — không probe 2 lần.
	cfg, err := guardUploadedFile(file, heicChecked)
	if err != nil {
		return nil, err
	}

	if shouldPassthrough(cfg, ext, file.Size, opts) {
		return passthrough(file)
	}

	// Acquire concurrency slot
	if err := acquireSlot(ctx); err != nil {
		return nil, err
	}
	defer releaseSlot()

	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer src.Close()

	result, err := processReader(src, file.Filename, opts)
	if err != nil {
		return nil, err
	}

	// Pick smaller: output ≥ gốc thì giữ gốc.
	// CHỈ áp khi ảnh gốc ĐÃ nằm trong hộp — nếu không sẽ mở lại đúng lỗ hổng story này đi vá:
	// ảnh 4000×3000 mà WebP ra to hơn sẽ được "giữ nguyên" ở kích thước quá khổ.
	// ModeFill loại trừ hoàn toàn vì nó cam kết output đúng W×H.
	if len(result.Data) >= int(file.Size) && opts.Mode != ModeFill && fitsInBox(cfg, opts) {
		return passthrough(file)
	}
	return result, nil
}

// fitsInBox — ảnh đã nằm gọn trong hộp đích chưa (chỉ có nghĩa với ModeFit).
// Hộp 0×0 là nhánh `Scale` của resizeFitBox: không có hộp để so → coi như CHƯA vừa, luôn xử lý.
func fitsInBox(cfg image.Config, opts *Options) bool {
	if opts.MaxWidth <= 0 && opts.MaxHeight <= 0 {
		return false
	}
	return cfg.Width <= opts.MaxWidth && cfg.Height <= opts.MaxHeight
}

// shouldPassthrough — bảng quyết định "giữ nguyên bản hay xử lý".
//
// ĐỔI TỪ 2026-08-13 (story-03): tiêu chí trước đây chỉ đo DUNG LƯỢNG FILE (`SkipBelowKB`), nên ảnh
// điện thoại nén tốt 4000×3000/180KB được giữ nguyên — FE phải tải và decode 12 triệu pixel để
// hiển thị avatar 40px. Nay pixel là điều kiện CHÍNH, dung lượng chỉ là điều kiện phụ đi kèm.
//
//	ModeRaw                        → giữ nguyên (đã qua guard bomb)
//	.gif                           → giữ nguyên (giữ animation)
//	ModeFill                       → LUÔN xử lý (cam kết output đúng W×H)
//	ModeFit + vừa hộp + .webp      → giữ nguyên (tránh re-encode mất chất vô ích)
//	ModeFit + vừa hộp + file nhẹ   → giữ nguyên
//	còn lại                        → xử lý
func shouldPassthrough(cfg image.Config, ext string, size int64, opts *Options) bool {
	if opts.Mode == ModeRaw {
		return true
	}
	// GIF động: image.Decode chỉ lấy frame ĐẦU → xử lý sẽ mất animation im lặng.
	if ext == ".gif" {
		return true
	}
	// ModeFill không có đường giữ nguyên: trước đây `OptFill500()` để SkipBelowKB=0 → nhận default
	// 200, nên ảnh 3000px/150KB "passthrough" và KHÔNG hề là 500×500 — cam kết bị phá âm thầm.
	if opts.Mode == ModeFill {
		return false
	}
	if !fitsInBox(cfg, opts) {
		return false
	}
	if ext == ".webp" {
		return true
	}
	return opts.SkipBelowKB > 0 && int(size/1024) <= opts.SkipBelowKB
}

func ProcessReader(ctx context.Context, r io.ReadSeeker, filename string, opts *Options) (*ProcessResult, error) {
	ctx = orBackground(ctx)
	if opts == nil {
		opts = new(defaultOpts)
	}
	applyDefaults(opts)

	// Guard trước khi chiếm slot: ảnh bomb bị loại ngay, không giữ chỗ hàng đợi vô ích.
	// ProcessReader KHÔNG có bảng quyết định passthrough như ProcessFile: không có file.Size để
	// áp SkipBelowKB, và caller đã chủ động chọn "xử lý ảnh này". Luôn decode → resize → WebP.
	if _, err := probeAndGuard(r); err != nil {
		return nil, err
	}

	if err := acquireSlot(ctx); err != nil {
		return nil, err
	}
	defer releaseSlot()
	return processReader(r, filename, opts)
}

// UploadProcessed lưu ảnh đã xử lý vào kho, trả KEY (đường dẫn tương đối — giá trị lưu DB).
// URL cho FE ghép lúc trả về bằng storage.PublicURL / storage.ResolveURL.
func UploadProcessed(ctx context.Context, st storage.Store, result *ProcessResult, basePath string) (string, error) {
	if st == nil {
		return "", fmt.Errorf("storage chưa được cấu hình")
	}
	key := strings.TrimLeft(basePath, "/") + result.Filename
	if err := st.Save(orBackground(ctx), bytes.NewReader(result.Data), key, result.ContentType); err != nil {
		return "", fmt.Errorf("upload storage: %w", err)
	}
	return key, nil
}

// ProcessAndUpload validate + xử lý ảnh + lưu vào kho, trả KEY (không phải URL).
func ProcessAndUpload(ctx context.Context, st storage.Store, file *multipart.FileHeader, basePath string, opts *Options) (string, error) {
	// HEIC phải chặn TRƯỚC ValidateFileType: image/heic không nằm trong storage.ImageMIMETypes,
	// nên nếu để ValidateFileType chạy trước, user nhận "loại file không được phép" thay vì
	// hướng dẫn cụ thể "đổi sang JPG".
	if err := guardHEIC(file); err != nil {
		return "", err
	}
	// Validate magic bytes trước khi decode — chặn SVG/HTML/exe giả dạng ảnh.
	if err := storage.ValidateFileType(file, storage.ImageMIMETypes); err != nil {
		return "", err
	}
	// heicChecked=true: guardHEIC đã chạy ngay phía trên, không lặp lại trong slot semaphore.
	result, err := processFile(ctx, file, opts, true)
	if err != nil {
		return "", err
	}
	return UploadProcessed(ctx, st, result, basePath)
}

// EncodeWebPTargetSizeCtx encode ảnh ĐÃ DỰNG SẴN trong bộ nhớ (vd thẻ coupon vẽ bằng gg) ra WebP.
//
// Bắt buộc đi qua hàng đợi như mọi đường xử lý ảnh khác: trước 2026-08-13 luồng vẽ thẻ coupon gọi
// thẳng encoder và là đường DUY NHẤT vượt rào giới hạn đồng thời của package.
func EncodeWebPTargetSizeCtx(ctx context.Context, img image.Image, targetKB, qMin, qMax int) ([]byte, error) {
	ctx = orBackground(ctx)
	if err := acquireSlot(ctx); err != nil {
		return nil, err
	}
	defer releaseSlot()
	return encodeWebPTargetSize(img, targetKB, qMin, qMax)
}

// GuardImageReader đọc header ảnh và từ chối nếu vượt ngưỡng an toàn (MaxImagePixels /
// MaxImageDimension). Dùng cho ảnh KHÔNG đến từ multipart upload — vd ảnh nền tải về từ URL —
// vốn không đi qua ProcessFile nên không được guard tự động.
// Reader được tua về đầu trước khi trả về.
func GuardImageReader(r io.ReadSeeker) error {
	_, err := probeAndGuard(r)
	return err
}

// ── Internal: process pipeline ───────────────────────────────

func processReader(r io.ReadSeeker, filename string, opts *Options) (*ProcessResult, error) {
	img, err := decodeWithOrientation(r)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	// Resize theo mode
	switch opts.Mode {
	case ModeFill:
		img = resizeFill(img, opts.MaxWidth, opts.MaxHeight)
	default: // ModeFit
		img = resizeFitBox(img, opts)
	}

	if opts.Sharpen {
		img = imaging.Sharpen(img, 0.5)
	}

	var data []byte
	if opts.Lossless {
		data, err = encodeWebPPreferLossless(img, opts)
	} else {
		data, err = encodeWebPTargetSize(img, opts.TargetKB, opts.QMin, opts.QMax)
	}
	if err != nil {
		return nil, fmt.Errorf("encode webp: %w", err)
	}

	return &ProcessResult{
		Data:        data,
		ContentType: "image/webp",
		Filename:    genFilename(filename, ".webp"),
	}, nil
}

// ── Resize strategies ────────────────────────────────────────

// resizeFitBox: giữ tỷ lệ, fit vào box, không upscale.
func resizeFitBox(img image.Image, opts *Options) image.Image {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	maxW, maxH := opts.MaxWidth, opts.MaxHeight

	if maxW == 0 && maxH == 0 {
		scale := opts.Scale
		if scale <= 0 || scale > 1 {
			scale = min64(1.0, 1280.0/float64(maxInt(w, h)))
		}
		maxW = int(float64(w) * scale)
		maxH = int(float64(h) * scale)
	}

	if w <= maxW && h <= maxH {
		return img
	}
	return imaging.Fit(img, maxW, maxH, imaging.Lanczos)
}

// resizeFill: crop center đúng width×height. Luôn output đúng kích thước.
// Ảnh nhỏ hơn target → upscale rồi crop (giống Yii2 imagecopyresampled).
func resizeFill(img image.Image, width, height int) image.Image {
	if width <= 0 || height <= 0 {
		return img
	}
	return imaging.Fill(img, width, height, imaging.Center, imaging.Lanczos)
}

// ── EXIF orientation ─────────────────────────────────────────

func decodeWithOrientation(r io.ReadSeeker) (image.Image, error) {
	orientation := 1
	if ex, err := exif.Decode(r); err == nil {
		if tag, err := ex.Get(exif.Orientation); err == nil {
			orientation, _ = tag.Int(0)
		}
	}
	_, _ = r.Seek(0, io.SeekStart) // reset reader after EXIF read; failure → caller's Decode will retry

	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	switch orientation {
	case 2:
		img = imaging.FlipH(img)
	case 3:
		img = imaging.Rotate180(img)
	case 4:
		img = imaging.FlipV(img)
	case 5:
		img = imaging.Transpose(img)
	case 6:
		img = imaging.Rotate270(img)
	case 7:
		img = imaging.Transverse(img)
	case 8:
		img = imaging.Rotate90(img)
	}
	return img, nil
}

// ── WebP encode with binary-search quality ───────────────────

func encodeWebPTargetSize(img image.Image, targetKB, qMin, qMax int) ([]byte, error) {
	tolerance := 10
	lo, hi := qMin, qMax
	var bestBuf []byte

	for lo <= hi {
		q := (lo + hi) / 2
		buf, err := encodeWebP(img, q)
		if err != nil {
			return nil, err
		}
		if len(buf)/1024 > targetKB+tolerance {
			hi = q - 1
		} else {
			bestBuf = buf
			lo = q + 1
		}
	}
	// Không quality nào trong [qMin,qMax] đạt target (ảnh nhiễu/nhiều chi tiết).
	// Fallback về qMin = kết quả NHỎ NHẤT có thể, tức GẦN target nhất.
	//
	// Trước 2026-08-13 chỗ này fallback về `bestQ` = giữa khoảng (nợ D8): đo thật với ảnh nhiễu
	// 800×600 → qMin(60) = 263KB, mid(76) = 296KB. Tức lúc đang cố ép nhỏ nhất có thể, hàm lại
	// trả file TO HƠN ~12% so với mức tốt nhất trong khoảng.
	if bestBuf == nil {
		var err error
		bestBuf, err = encodeWebP(img, qMin)
		if err != nil {
			return nil, err
		}
	}
	return bestBuf, nil
}

// encodeWebPPreferLossless — encode CẢ HAI cách rồi chọn bản nhỏ hơn.
//
// Lý do không tin tưởng tuyệt đối vào cờ Lossless: preset line art được chọn theo `mode` do client
// gửi. Nếu ảnh CHỤP (scan giấy, ảnh điện thoại) bị gửi nhầm vào mode line art, lossless sẽ cho file
// to gấp nhiều lần — nội dung nhiếp ảnh có nhiễu/gradient, không nén lossless được.
//
// So kích thước là cách phân loại rẻ và KHÔNG BAO GIỜ sai hướng: line art thì lossless luôn thắng
// (nên cam kết pixel-exact vẫn giữ đúng ở nơi nó có ý nghĩa); ảnh chụp thì lossy thắng và đó chính
// là điều đúng cần làm. Không cần heuristic đếm màu / tỉ lệ alpha.
//
// Chi phí: encode 2 lần. Chấp nhận được vì hộp của preset line art nhỏ (1600px) và canvas chữ ký
// thực tế chỉ ~600×200.
func encodeWebPPreferLossless(img image.Image, opts *Options) ([]byte, error) {
	lossless, err := encodeWebPLossless(img)
	if err != nil {
		return nil, err
	}
	// Lossless đã nằm trong ngân sách → KHÔNG dò lossy nữa.
	// Line art thật (chữ ký ~600×200) gần như luôn rơi vào đây ⇒ đúng 1 lần encode. Chỉ ảnh chụp
	// gửi nhầm mode mới trả giá binary-search (tới 5 lần encode) — mà đó mới là case cần nó.
	if opts.TargetKB > 0 && len(lossless)/1024 <= opts.TargetKB {
		return lossless, nil
	}

	lossy, err := encodeWebPTargetSize(img, opts.TargetKB, opts.QMin, opts.QMax)
	if err != nil {
		// Lossless đã có sẵn — không để lỗi nhánh dự phòng làm hỏng cả request.
		return lossless, nil
	}
	if len(lossy) < len(lossless) {
		return lossy, nil
	}
	return lossless, nil
}

// encodeWebPLossless — pixel-exact, không có tham số quality.
func encodeWebPLossless(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	// level 6/9: cân bằng giữa kích thước và CPU — encode chạy trong hàng đợi 4 slot dùng chung.
	options, err := gowebp.NewLosslessEncoderOptions(gowebp.PresetDefault, webpLosslessLevel)
	if err != nil {
		return nil, err
	}
	if err := webp.Encode(&buf, img, options); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

const webpLosslessLevel = 6

func encodeWebP(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	options, err := gowebp.NewLossyEncoderOptions(gowebp.PresetDefault, float32(quality))
	if err != nil {
		return nil, err
	}
	if err := webp.Encode(&buf, img, options); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ── Passthrough: upload original, no processing ──────────────

func passthrough(file *multipart.FileHeader) (*ProcessResult, error) {
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}

	// Content-Type + đuôi theo MIME ĐÃ SNIFF từ nội dung, KHÔNG theo tên client gửi: PNG đặt tên
	// `.jpg` vẫn lưu là `.png` với Content-Type image/png (khớp bytes thật).
	ct := storage.BaseMIME(mimetype.Detect(data).String())
	return &ProcessResult{
		Data:        data,
		ContentType: ct,
		Filename:    genFilename(file.Filename, extFromImageMIME(ct, strings.ToLower(filepath.Ext(file.Filename)))),
	}, nil
}

// extFromImageMIME — đuôi file theo MIME ảnh đã sniff; MIME lạ (không xảy ra sau whitelist) ⇒ fallback.
func extFromImageMIME(m, fallback string) string {
	switch m {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}
	return fallback
}

// ── Helpers ──────────────────────────────────────────────────

func applyDefaults(opts *Options) {
	if opts.TargetKB == 0 {
		opts.TargetKB = 160
	}
	if opts.QMin == 0 {
		opts.QMin = 60
	}
	if opts.QMax == 0 {
		opts.QMax = 92
	}
	if opts.SkipBelowKB == 0 {
		opts.SkipBelowKB = 200
	}
}

// genFilename sinh tên file lưu trong kho. CỐ Ý không dùng tên client gửi (`original`): tên gốc
// thường chứa họ tên khách/nhân viên và sẽ lộ ra URL công khai của ảnh.
func genFilename(_ string, ext string) string {
	return fmt.Sprintf("img-%s-%d%s", helpers.RandomString(16), time.Now().Unix(), ext)
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── Fallback encoders ────────────────────────────────────────

func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func EncodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ── File helpers ─────────────────────────────────────────────

func SaveTmp(file *multipart.FileHeader) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()
	tmp, err := os.CreateTemp("", "img-*.tmp")
	if err != nil {
		return "", err
	}
	defer tmp.Close()
	if _, err := io.Copy(tmp, src); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}
