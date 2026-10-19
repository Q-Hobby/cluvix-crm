package storage_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"erpCluvix/pkg/storage"
)

// Test gác #11: hai cơ chế ghép URL ảnh (map storage.TableImageColumns dùng bởi
// handlerutil.AttachRelation*, và các handler tự gọi ResolveFields/PublicURL) phải khớp với
// các cột THỰC SỰ được ghi qua pkg/imaging. Test không đổi hành vi runtime, chỉ quét mã nguồn.

// imageWriters — khai báo: file handler (tính từ backend/) gọi pkg/imaging để ghi cột ảnh
// vào kho public ⇒ các "bảng.cột" nó ghi. File mới gọi imaging mà chưa khai báo ở đây (hoặc
// ở imageWriterExempt) ⇒ test đỏ, buộc người sửa quyết định có thêm vào TableImageColumns không.
var imageWriters = map[string][]string{
	"internal/modules/webapp/config/business_profile/handler/handler.go": {"business_profile.logo_image"},
	"internal/modules/webapp/config/onboarding/handler/bootstrap.go":     {"business_profile.logo_image"},
	"internal/modules/webapp/config/user_profile/handler/handler.go":     {"user_profile.avatar"},
	"internal/modules/webapp/crm/customer/handler/customer.go":           {"customer.avatar"},
}

// imageWriterExempt — gọi imaging nhưng KHÔNG ghi cột ảnh public (không cần ghép URL theo bảng).
var imageWriterExempt = map[string]string{
	"internal/modules/webapp/crm/tinymce/handler/handler.go":   "ảnh nhúng trong nội dung HTML, không có cột",
	"internal/modules/webapp/crm/attachment/handler/upload.go": "kho private (PrivateStore), tải qua endpoint riêng",
}

var imagingWriteCall = regexp.MustCompile(`imaging\.(HandleFormImage|HandleFormImageRaw|ProcessAndUpload|ResolvePayloadImage|SaveUploadSmart|UploadProcessed)\(`)

func scanImagingWriterFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, root := range []string{"../../internal", "../../pkg"} {
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			if strings.HasPrefix(filepath.ToSlash(p), "../../pkg/imaging/") {
				return nil
			}
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			// bỏ dòng comment để ví dụ trong doc không tính là call site
			for _, ln := range strings.Split(string(b), "\n") {
				if s := strings.TrimSpace(ln); strings.HasPrefix(s, "//") {
					continue
				}
				if imagingWriteCall.MatchString(ln) {
					files = append(files, strings.TrimPrefix(filepath.ToSlash(p), "../../"))
					break
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("quét %s: %v", root, err)
		}
	}
	sort.Strings(files)
	if len(files) < 5 {
		t.Fatalf("quét chỉ thấy %d file gọi imaging — bộ quét hỏng (mong ~9): %v", len(files), files)
	}
	return files
}

func TestImageWriters_AllDeclared(t *testing.T) {
	for _, f := range scanImagingWriterFiles(t) {
		_, w := imageWriters[f]
		_, e := imageWriterExempt[f]
		if !w && !e {
			t.Errorf("%s gọi pkg/imaging để ghi ảnh nhưng chưa khai báo: thêm vào imageWriters (kèm \"bảng.cột\" + thêm cột vào storage.TableImageColumns) hoặc imageWriterExempt", f)
		}
	}
	// khai báo thừa/lỗi thời (file không còn gọi imaging)
	scanned := map[string]bool{}
	for _, f := range scanImagingWriterFiles(t) {
		scanned[f] = true
	}
	for f := range imageWriters {
		if !scanned[f] {
			t.Errorf("imageWriters khai báo %s nhưng file không còn (hoặc không gọi) pkg/imaging — dọn khai báo", f)
		}
	}
	for f := range imageWriterExempt {
		if !scanned[f] {
			t.Errorf("imageWriterExempt khai báo %s nhưng file không còn gọi pkg/imaging — dọn khai báo", f)
		}
	}
}

func declaredColumns() map[string]bool {
	m := map[string]bool{}
	for _, cols := range imageWriters {
		for _, tc := range cols {
			m[tc] = true
		}
	}
	return m
}

func mapColumns() map[string]bool {
	m := map[string]bool{}
	for table, cols := range storage.TableImageColumns {
		for _, c := range cols {
			m[table+"."+c] = true
		}
	}
	return m
}

func TestTableImageColumns_CoversEveryWrittenColumn(t *testing.T) {
	in := mapColumns()
	for tc := range declaredColumns() {
		if !in[tc] {
			t.Errorf("cột ảnh %s được ghi qua pkg/imaging nhưng thiếu trong storage.TableImageColumns — thêm cột %s vào TableImageColumns", tc, tc)
		}
	}
}

func TestTableImageColumns_NoStaleEntries(t *testing.T) {
	declared := declaredColumns()
	for tc := range mapColumns() {
		if !declared[tc] {
			t.Errorf("storage.TableImageColumns có %s nhưng không có handler nào ghi cột này qua pkg/imaging (imageWriters) — xoá entry hoặc khai báo writer", tc)
		}
	}
}

func TestTableImageColumns_ColumnsExistInMigrations(t *testing.T) {
	files, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("không đọc được migrations (%v, %d file)", err, len(files))
	}
	var sb strings.Builder
	for _, f := range files {
		b, rerr := os.ReadFile(f)
		if rerr != nil {
			t.Fatal(rerr)
		}
		sb.Write(b)
		sb.WriteString("\n")
	}
	all := sb.String()
	for table, cols := range storage.TableImageColumns {
		re := regexp.MustCompile("(?is)CREATE TABLE (?:IF NOT EXISTS )?`" + regexp.QuoteMeta(table) + "`\\s*\\((.*?)\\)\\s*ENGINE")
		m := re.FindStringSubmatch(all)
		if m == nil {
			t.Errorf("TableImageColumns trỏ tới bảng %q nhưng không có CREATE TABLE tương ứng trong migrations", table)
			continue
		}
		for _, c := range cols {
			if !strings.Contains(m[1], "`"+c+"` ") {
				t.Errorf("TableImageColumns[%q] có cột %q nhưng CREATE TABLE %s không có cột này", table, c, table)
			}
		}
	}
}
