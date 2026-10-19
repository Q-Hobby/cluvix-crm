package storage

import (
	"context"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// Local — driver ghi thẳng ra đĩa. Đủ cho 1 instance; nhiều instance phải dùng object storage chung.
//
// Kho public được router phục vụ static tại publicPrefix (xem Root/PublicPrefix); kho private
// KHÔNG BAO GIỜ được mount — chỉ đọc qua Open trong handler có auth.
type Local struct {
	root         string // đường dẫn tuyệt đối đã Clean
	publicPrefix string // vd "/uploads" — chỉ có nghĩa khi public
	publicBase   string // vd "https://api.khach.vn" — rỗng ⇒ URL tương đối
	public       bool
}

var _ Store = (*Local)(nil)

// NewLocal tạo thư mục gốc nếu chưa có. public=false ⇒ URL() luôn rỗng.
func NewLocal(root, publicPrefix, publicBase string, public bool) (*Local, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, err
	}
	prefix := "/" + strings.Trim(publicPrefix, "/")
	return &Local{
		root:         filepath.Clean(abs),
		publicPrefix: prefix,
		publicBase:   strings.TrimRight(strings.TrimSpace(publicBase), "/"),
		public:       public,
	}, nil
}

// Root — thư mục gốc tuyệt đối (router dùng để mount static cho kho public).
func (l *Local) Root() string { return l.root }

// PublicPrefix — prefix URL phục vụ static (vd "/uploads").
func (l *Local) PublicPrefix() string { return l.publicPrefix }

func (l *Local) Public() bool { return l.public }

// Save ghi ra đĩa qua file tạm + rename: không bao giờ để lại file ghi dở dưới đúng key.
func (l *Local) Save(ctx context.Context, r io.Reader, key, _ string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	full, err := l.resolve(key)
	if err != nil {
		return err
	}
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op sau khi rename thành công

	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// CreateTemp tạo quyền 0600 — đủ: chỉ chính process backend đọc (static /uploads đi qua Go).
	return os.Rename(tmpName, full)
}

// Open mở file để đọc. Không tồn tại (hoặc là thư mục) ⇒ ErrNotFound.
func (l *Local) Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, ObjectInfo{}, err
	}
	full, err := l.resolve(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	f, err := os.Open(full) //nolint:gosec // full đã qua resolve (chặn traversal)
	if os.IsNotExist(err) {
		return nil, ObjectInfo{}, ErrNotFound
	}
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, ObjectInfo{}, err
	}
	if st.IsDir() {
		_ = f.Close()
		return nil, ObjectInfo{}, ErrNotFound
	}
	return f, ObjectInfo{Size: st.Size(), ContentType: mime.TypeByExtension(filepath.Ext(full))}, nil
}

// Delete — file đã bị xoá tay ⇒ coi như xong, không báo lỗi.
func (l *Local) Delete(_ context.Context, key string) error {
	full, err := l.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// URL — kho private luôn "". Kho public: publicBase + publicPrefix + "/" + key.
//
// publicBase rỗng ⇒ URL tương đối "/uploads/…": chỉ đúng khi FE cùng origin với API (nginx/proxy
// chuyển /uploads về backend). FE khác origin ⇒ khai STORAGE_PUBLIC_BASE, nếu không mọi ảnh 404.
func (l *Local) URL(key string) string {
	if !l.public {
		return ""
	}
	return l.publicBase + l.publicPrefix + "/" + strings.TrimLeft(filepath.ToSlash(key), "/")
}

// resolve chặn path traversal bằng đường dẫn ĐÃ CHUẨN HOÁ (không chỉ tìm chuỗi ".."):
// so prefix sau khi Join+Clean mới đúng cho mọi biến thể.
func (l *Local) resolve(key string) (string, error) {
	if err := SafeKey(key); err != nil {
		return "", err
	}
	if filepath.IsAbs(key) || filepath.VolumeName(key) != "" {
		return "", ErrUnsafePath
	}
	full := filepath.Join(l.root, filepath.FromSlash(key))
	if full == l.root || !strings.HasPrefix(full, l.root+string(os.PathSeparator)) {
		return "", ErrUnsafePath
	}
	return full, nil
}
