package codegen

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// directCustomerGenerate — gọi thẳng Generate(<db>, "customer", …): bỏ qua quy tắc "chưa có chi nhánh ⇒
// chưa có mã", CUSTOMER_CODE_OPTION, dò trùng và bộ đếm ngoài tx. Phải qua NewCustomerCode /
// EnsureCustomerCode / NewCustomerCoder.
var directCustomerGenerate = regexp.MustCompile(`\bGenerate\(\s*[^,()]+,\s*"customer"`)

func TestNoDirectCustomerGenerate(t *testing.T) {
	// Đối chứng: regex PHẢI bắt được mẫu vi phạm, nếu không gate "0 vi phạm" là xanh giả.
	for _, bad := range []string{
		`codegen.Generate(tx, "customer", &bid)`,
		`Generate(s.DB,"customer", nil)`,
	} {
		if !directCustomerGenerate.MatchString(bad) {
			t.Fatalf("regex không bắt được mẫu vi phạm %q", bad)
		}
	}

	root := filepath.Join("..", "..") // backend/
	scanned := 0
	var hits []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "tmp", "vendor", ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if filepath.Dir(path) == "." { // chính package codegen
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scanned++
		for i, line := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if directCustomerGenerate.MatchString(line) {
				hits = append(hits, path+":"+strconv.Itoa(i+1))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned < 100 { // đối chứng: gate có quét thật, không phải đi sai thư mục rồi trả 0
		t.Fatalf("chỉ quét %d file .go — sai thư mục gốc?", scanned)
	}
	if len(hits) > 0 {
		t.Fatalf("gọi thẳng Generate(…, \"customer\") — dùng codegen.NewCustomerCode/EnsureCustomerCode:\n%s", strings.Join(hits, "\n"))
	}
}
