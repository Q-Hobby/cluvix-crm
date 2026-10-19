package export

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"erpCluvix/pkg/cleanup"

	"github.com/xuri/excelize/v2"
)

// DefaultDir — thư mục gốc chứa file export. Mỗi người tạo có thư mục con `<DefaultDir>/<userID>/`
// (xem Config.OwnerID) — endpoint tải chỉ tra trong thư mục con của chính user đang đăng nhập, nên
// file của người khác trả 404 mà không cần thêm bảng/cột nào.
const DefaultDir = "./storage/export/"

// DownloadPath — prefix URL tải file export. Nằm trong nhóm `/api` CÓ JWT (S-2/O-6, 2026-10-18):
// trước đây thư mục được `r.Static("/download/export", …)` phục vụ công khai với tên đoán được tới giây.
const DownloadPath = "/api/download/export/"

// fileTokenBytes — 16 byte = 128 bit ngẫu nhiên trong tên file (crypto/rand).
const fileTokenBytes = 16

// fileNameMaxLen — trần độ dài tên file chấp nhận ở endpoint tải (tên sinh ra luôn ngắn hơn nhiều).
const fileNameMaxLen = 200

// validFileName — tên file hợp lệ ở endpoint tải: CHỈ basename, ký tự [A-Za-z0-9_-], đúng một dấu
// chấm trước đuôi xlsx/csv. Không có `/`, `\`, `..`, không bắt đầu bằng `.` (file tạm `.export-*.part`).
var validFileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*\.(xlsx|csv)$`)

// unsafeBaseChars — mọi ký tự ngoài tập an toàn trong phần tên do caller đặt (có caller ghép cả giá
// trị query string vào tên, vd ngày của lịch hẹn theo giờ).
var unsafeBaseChars = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// ValidFileName báo tên file có hợp lệ để phục vụ tải không.
func ValidFileName(name string) bool {
	return len(name) <= fileNameMaxLen && validFileName.MatchString(name)
}

// OwnerDir trả thư mục chứa file export của owner. owner = 0 (không có người tạo — chỉ gặp ở test
// gọi thẳng pkg/export) ⇒ chính `dir`, KHÔNG endpoint nào phục vụ được thư mục này.
func OwnerDir(dir string, owner uint) string {
	if dir == "" {
		dir = DefaultDir
	}
	if owner == 0 {
		return dir
	}
	return filepath.Join(dir, strconv.FormatUint(uint64(owner), 10)) + string(os.PathSeparator)
}

// OwnerFilePath trả đường dẫn file `name` của owner trong `dir`; false khi tên không hợp lệ hoặc
// owner = 0. Đây là cửa DUY NHẤT endpoint tải dùng để dựng path — không ghép chuỗi ở nơi khác.
func OwnerFilePath(dir string, owner uint, name string) (string, bool) {
	if owner == 0 || !ValidFileName(name) {
		return "", false
	}
	return filepath.Join(OwnerDir(dir, owner), name), true
}

// DownloadLink — URL tải file `name` (đi qua endpoint có auth).
func DownloadLink(downloadURL, name string) string {
	return downloadURL + DownloadPath + name
}

// newFileName = `<base an toàn>-<ts>-<32 hex ngẫu nhiên>.<ext>`. Thành phần ngẫu nhiên 128 bit làm
// tên KHÔNG đoán được kể cả khi biết tên báo cáo + giây export.
func newFileName(base, ts, ext string) (string, error) {
	buf := make([]byte, fileTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("random file token error: %v", err)
	}
	base = strings.Trim(unsafeBaseChars.ReplaceAllString(base, "_"), "_-")
	if base == "" {
		base = "export"
	}
	// Chừa chỗ cho "-<ts>-<token>.<ext>" để tên luôn qua được ValidFileName.
	if maxBase := fileNameMaxLen - len(ts) - 2*fileTokenBytes - len(ext) - 3; len(base) > maxBase {
		base = base[:maxBase]
	}
	return base + "-" + ts + "-" + hex.EncodeToString(buf) + "." + ext, nil
}

// prepareFile tạo thư mục owner và trả (đường dẫn file, tên file) cho 1 file export mới.
func prepareFile(cfg Config, ts, ext string) (string, string, error) {
	dir := OwnerDir(cfg.ExportDir, cfg.OwnerID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", "", fmt.Errorf("mkdir error: %v", err)
	}
	name, err := newFileName(cfg.FileName, ts, ext)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, name), name, nil
}

// SaveWorkbook ghi 1 workbook do caller tự dựng (layout đặc thù không qua Export) theo đúng quy ước
// đặt tên/owner/URL của package, ghi nguyên tử và hẹn xoá như Export. Dùng cfg.FileName, ExportDir,
// OwnerID, DownloadURL.
func SaveWorkbook(f *excelize.File, cfg Config) (string, error) {
	filePath, name, err := prepareFile(cfg, time.Now().Format("2006-01-02-15-04-05"), "xlsx")
	if err != nil {
		return "", err
	}
	if err := writeFileAtomically(filePath, func(out *os.File) error {
		return f.Write(out)
	}); err != nil {
		return "", err
	}
	cleanup.ScheduleFileDelete(filePath, 30*time.Minute)
	return DownloadLink(cfg.DownloadURL, name), nil
}

// CleanupOld xoá mọi file thường trong `dir` (đệ quy, gồm cả file tạm `.part` mồ côi) có mtime cũ hơn
// maxAge tính tới `now`. Trả số file đã xoá. Thư mục con owner GIỮ lại (số lượng = số user, không
// phình): xoá thư mục rỗng sẽ đua với export đang MkdirAll→CreateTemp cùng lúc.
//
// Lý do tồn tại: `cleanup.ScheduleFileDelete` chỉ là hẹn giờ trong RAM — server restart giữa chừng
// là file nằm lại vĩnh viễn (O-6). Hàm này chạy lúc boot + định kỳ (bootstrap, helpers.RunBackground).
func CleanupOld(dir string, maxAge time.Duration, now time.Time) (int, error) {
	if maxAge <= 0 {
		return 0, fmt.Errorf("export cleanup: maxAge phải > 0")
	}
	cutoff := now.Add(-maxAge)
	removed := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil // file vừa bị xoá bởi ScheduleFileDelete — bỏ qua
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(p); err == nil {
				removed++
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	})
	return removed, err
}
