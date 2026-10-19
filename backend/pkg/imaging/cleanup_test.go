package imaging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"erpCluvix/pkg/storage"
)

func newCleanupStore(t *testing.T) *storage.Local {
	t.Helper()
	st, err := storage.NewLocal(filepath.Join(t.TempDir(), "uploads"), "/uploads", "https://cdn.example.com", true)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func putFile(t *testing.T, st *storage.Local, key string) {
	t.Helper()
	if err := st.Save(t.Context(), strings.NewReader("x"), key, "image/webp"); err != nil {
		t.Fatal(err)
	}
}

func exists(st *storage.Local, key string) bool {
	_, err := os.Stat(filepath.Join(st.Root(), filepath.FromSlash(key)))
	return err == nil
}

func TestCleanupNewOnError(t *testing.T) {
	st := newCleanupStore(t)
	putFile(t, st, "branch/old.webp")
	putFile(t, st, "branch/new.webp")

	// Key mới khác key cũ ⇒ xoá key mới, KHÔNG đụng key cũ.
	CleanupNewOnError(st, "branch/old.webp", "branch/new.webp", "branch/")
	if exists(st, "branch/new.webp") {
		t.Fatal("file mới vừa upload phải bị dọn khi DB lỗi")
	}
	if !exists(st, "branch/old.webp") {
		t.Fatal("file cũ KHÔNG được bị xoá")
	}

	// Không upload gì ⇒ no-op.
	CleanupNewOnError(st, "branch/old.webp", "", "branch/")
	if !exists(st, "branch/old.webp") {
		t.Fatal("uploadedKey rỗng phải là no-op")
	}

	// uploadedKey trùng ảnh cũ (cũ lưu dạng URL tuyệt đối của chính kho) ⇒ không xoá.
	CleanupNewOnError(st, st.URL("branch/old.webp"), "branch/old.webp", "branch/")
	if !exists(st, "branch/old.webp") {
		t.Fatal("key trùng ảnh cũ (sau chuẩn hoá) KHÔNG được xoá")
	}

	// Key không an toàn / URL tuyệt đối ⇒ bỏ qua, không panic.
	CleanupNewOnError(st, "", "../escape.webp", "branch/")
	CleanupNewOnError(st, "", "https://elsewhere.example.com/a.webp", "branch/")
	CleanupNewOnError(nil, "", "branch/old.webp", "branch/")
	if !exists(st, "branch/old.webp") {
		t.Fatal("store nil phải là no-op")
	}

	// Key ngoài prefix cho phép (vd file đính kèm riêng tư) ⇒ KHÔNG xoá; không truyền prefix ⇒ không xoá.
	putFile(t, st, "private/customer/att.pdf")
	CleanupNewOnError(st, "", "private/customer/att.pdf", "branch/")
	CleanupNewOnError(st, "", "branch/old.webp")
	if !exists(st, "private/customer/att.pdf") || !exists(st, "branch/old.webp") {
		t.Fatal("key ngoài prefix / thiếu prefix KHÔNG được xoá")
	}
}

func TestPendingUploads(t *testing.T) {
	st := newCleanupStore(t)

	// Chưa Keep ⇒ Cleanup dọn mọi file đã Track.
	putFile(t, st, "svc/a.webp")
	putFile(t, st, "svc/b.webp")
	p := NewPendingUploads(st, "svc/")
	p.Track("", "svc/a.webp")
	p.Track("svc/old.webp", "svc/b.webp")
	p.Track("svc/old.webp", "") // không upload ⇒ bỏ qua
	p.Cleanup()
	if exists(st, "svc/a.webp") || exists(st, "svc/b.webp") {
		t.Fatal("chưa Keep ⇒ file đã Track phải bị dọn")
	}

	// Đã Keep ⇒ Cleanup không làm gì.
	putFile(t, st, "svc/c.webp")
	p2 := NewPendingUploads(st, "svc/")
	p2.Track("", "svc/c.webp")
	p2.Keep()
	p2.Cleanup()
	if !exists(st, "svc/c.webp") {
		t.Fatal("đã Keep ⇒ file phải được giữ")
	}
}

func TestPendingUploads_OutsidePrefixKept(t *testing.T) {
	st := newCleanupStore(t)
	putFile(t, st, "business_profile/logo.webp")
	p := NewPendingUploads(st, "svc/")
	p.Track("", "business_profile/logo.webp")
	p.Cleanup()
	if !exists(st, "business_profile/logo.webp") {
		t.Fatal("PendingUploads KHÔNG được dọn key ngoài prefix")
	}
}
