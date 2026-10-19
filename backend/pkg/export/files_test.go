package export

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// S-2: tên file export phải chứa ≥128 bit ngẫu nhiên và luôn qua được ValidFileName của endpoint tải.
func TestNewFileName_NgauNhienVaHopLe(t *testing.T) {
	re := regexp.MustCompile(`^Khach-Hang-export-2026-10-18-10-00-00-[0-9a-f]{32}\.xlsx$`)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		n, err := newFileName("Khach-Hang-export", "2026-10-18-10-00-00", "xlsx")
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(n) {
			t.Fatalf("tên %q sai mẫu <base>-<ts>-<32 hex>.xlsx", n)
		}
		if !ValidFileName(n) {
			t.Fatalf("tên sinh ra %q không qua ValidFileName", n)
		}
		if seen[n] {
			t.Fatalf("trùng tên trong cùng giây: %q", n)
		}
		seen[n] = true
	}
	// Phần base do caller đặt có thể chứa giá trị query string ⇒ bị làm sạch, không thoát thư mục.
	n, err := newFileName("../../etc/Lich hen_2026", "ts", "csv")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(n, "/\\. ") && !strings.HasSuffix(n, ".csv") || strings.Count(n, ".") != 1 || !ValidFileName(n) {
		t.Fatalf("base không được làm sạch: %q", n)
	}
	// Base quá dài vẫn cho ra tên hợp lệ.
	if n, _ := newFileName(strings.Repeat("a", 500), "ts", "xlsx"); !ValidFileName(n) {
		t.Fatalf("base dài ⇒ tên không hợp lệ: len=%d", len(n))
	}
}

func TestValidFileName(t *testing.T) {
	for name, want := range map[string]bool{
		"Khach-Hang-export-2026-10-18-10-00-00-0123456789abcdef0123456789abcdef.xlsx": true,
		"a.csv":                            true,
		"../a.xlsx":                        false,
		"..":                               false,
		"a/b.xlsx":                         false,
		`a\b.xlsx`:                         false,
		".export-123.part":                 false,
		".hidden.xlsx":                     false,
		"a.xlsx.exe":                       false,
		"a.txt":                            false,
		"a..xlsx":                          false,
		"":                                 false,
		strings.Repeat("a", 300) + ".xlsx": false,
	} {
		if got := ValidFileName(name); got != want {
			t.Errorf("ValidFileName(%q) = %v, muốn %v", name, got, want)
		}
	}
}

func TestOwnerFilePath(t *testing.T) {
	if _, ok := OwnerFilePath("./storage/export/", 0, "a.xlsx"); ok {
		t.Fatal("owner 0 không được phục vụ")
	}
	if _, ok := OwnerFilePath("./storage/export/", 5, "../6/a.xlsx"); ok {
		t.Fatal("tên traversal phải bị từ chối")
	}
	p, ok := OwnerFilePath("./storage/export/", 5, "a.xlsx")
	if !ok || p != filepath.Join("storage", "export", "5", "a.xlsx") {
		t.Fatalf("OwnerFilePath = %q, %v", p, ok)
	}
}

// O-6: file cũ hơn TTL bị dọn (kể cả trong thư mục owner và file tạm .part mồ côi); file mới giữ.
func TestCleanupOld(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	mk := func(rel string, age time.Duration) string {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
		return p
	}
	oldOwned := mk("7/old.xlsx", 25*time.Hour)
	oldRoot := mk("legacy-2026-01-01-00-00-00.xlsx", 48*time.Hour)
	oldPart := mk("7/.export-1.part", 30*time.Hour)
	fresh := mk("7/fresh.xlsx", time.Hour)

	n, err := CleanupOld(dir, 24*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("đã xoá %d file, muốn 3", n)
	}
	for _, p := range []string{oldOwned, oldRoot, oldPart} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s phải bị dọn", p)
		}
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("file mới bị xoá nhầm: %v", err)
	}
	// Thư mục chưa tồn tại (chưa ai export) không phải lỗi.
	if _, err := CleanupOld(filepath.Join(dir, "khong-co"), time.Hour, now); err != nil {
		t.Fatalf("thư mục không tồn tại: %v", err)
	}
	if _, err := CleanupOld(dir, 0, now); err == nil {
		t.Fatal("maxAge 0 phải lỗi (tránh xoá sạch)")
	}
}
