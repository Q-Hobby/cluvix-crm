//go:build integration

package attachmentprivatize

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/storage"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

const pdfA = "%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n% A\n"
const pdfB = "%PDF-1.4\n% legacy B via HTTP\n%%EOF\n"

type env struct {
	pub, priv  *storage.Local
	cdn, other *httptest.Server
	otherHits  atomic.Int32
}

// newEnv — kho public local được "CDN giả" (httptest) phục vụ tại /uploads/, kho private local riêng.
// /legacy/b.pdf: file chỉ có qua HTTP (ngoài kho public). /redir: redirect sang server khác (host lạ).
func newEnv(t *testing.T) *env {
	t.Helper()
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	require.NoError(t, testDB.Exec("DELETE FROM attachment").Error)
	require.NoError(t, testDB.Exec("DROP TRIGGER IF EXISTS trg_att_fail").Error)

	e := &env{}
	dir := t.TempDir()
	pubDir := filepath.Join(dir, "uploads")
	require.NoError(t, os.MkdirAll(pubDir, 0o750))

	e.other = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		e.otherHits.Add(1)
		_, _ = w.Write([]byte(pdfB))
	}))
	t.Cleanup(e.other.Close)

	mux := http.NewServeMux()
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(pubDir))))
	mux.HandleFunc("/legacy/b.pdf", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(pdfB)) })
	mux.HandleFunc("/legacy/big.pdf", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(pdfB + strings.Repeat("x", 4096)))
	})
	mux.HandleFunc("/redir", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, e.other.URL+"/b.pdf", http.StatusFound)
	})
	e.cdn = httptest.NewServer(mux)
	t.Cleanup(e.cdn.Close)

	var err error
	e.pub, err = storage.NewLocal(pubDir, "/uploads", e.cdn.URL, true)
	require.NoError(t, err)
	e.priv, err = storage.NewLocal(filepath.Join(dir, "private"), "", "", false)
	require.NoError(t, err)
	return e
}

func (e *env) migrator(apply, del bool) *Migrator {
	return &Migrator{DB: testDB, Public: e.pub, Private: e.priv, Opts: Options{
		Apply: apply, DeletePublic: del, Batch: 2,
		AllowHosts: HostsFromConfig(e.cdn.URL), Log: io.Discard,
	}}
}

// putPublic ghi object vào kho public, trả URL tuyệt đối cũ (như dữ liệu trước 2026-10-19).
func (e *env) putPublic(t *testing.T, key, body string) string {
	t.Helper()
	require.NoError(t, e.pub.Save(context.Background(), strings.NewReader(body), key, "application/pdf"))
	return e.pub.URL(key)
}

func insertAtt(t *testing.T, customerID uint, fileURL string) uint {
	t.Helper()
	// foreign_key_checks=0 ở session của suite ⇒ không cần dựng customer.
	require.NoError(t, testDB.Exec(
		"INSERT INTO attachment (ref_type, ref_id, customer_id, file_name, file_url, file_type) VALUES ('customer', ?, ?, 'ho-so.pdf', ?, 'application/octet-stream')",
		customerID, customerID, fileURL).Error)
	var id uint
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&id).Error)
	return id
}

func fileURL(t *testing.T, id uint) string {
	t.Helper()
	var v string
	require.NoError(t, testDB.Raw("SELECT file_url FROM attachment WHERE id = ?", id).Scan(&v).Error)
	return v
}

func countFiles(t *testing.T, root string) int {
	t.Helper()
	n := 0
	require.NoError(t, filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			n++
		}
		return nil
	}))
	return n
}

func readPriv(t *testing.T, s *storage.Local, key string) []byte {
	t.Helper()
	rc, _, err := s.Open(context.Background(), key)
	require.NoError(t, err)
	defer rc.Close()
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	return b
}

func TestDryRun_ChangesNothing(t *testing.T) {
	e := newEnv(t)
	u1 := e.putPublic(t, "crm/attachments/7/old-a.pdf", pdfA)
	id1 := insertAtt(t, 7, u1)
	id2 := insertAtt(t, 8, e.cdn.URL+"/legacy/b.pdf")
	id3 := insertAtt(t, 9, "crm/attachments/9/already-key.pdf")

	rep, err := e.migrator(false, false).Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, rep.Total)
	require.Equal(t, 1, rep.AlreadyKey)
	require.Equal(t, 2, rep.Planned)
	require.Zero(t, rep.Migrated)
	require.Empty(t, rep.Failed)

	require.Equal(t, u1, fileURL(t, id1))
	require.Equal(t, e.cdn.URL+"/legacy/b.pdf", fileURL(t, id2))
	require.Equal(t, "crm/attachments/9/already-key.pdf", fileURL(t, id3))
	require.Zero(t, countFiles(t, e.priv.Root()))
}

func TestApply_MigratesByteIdentical_AndRerunIsNoop(t *testing.T) {
	e := newEnv(t)
	u1 := e.putPublic(t, "crm/attachments/7/old-a.pdf", pdfA)
	id1 := insertAtt(t, 7, u1)
	id2 := insertAtt(t, 8, e.cdn.URL+"/legacy/b.pdf")
	id3 := insertAtt(t, 9, "crm/attachments/9/already-key.pdf")

	rep, err := e.migrator(true, false).Run(context.Background())
	require.NoError(t, err)
	require.Empty(t, rep.Failed)
	require.Equal(t, 2, rep.Migrated)
	require.Equal(t, 1, rep.AlreadyKey)

	k1, k2 := fileURL(t, id1), fileURL(t, id2)
	require.True(t, strings.HasPrefix(k1, "crm/attachments/7/") && strings.HasSuffix(k1, ".pdf"), k1)
	require.True(t, strings.HasPrefix(k2, "crm/attachments/8/") && strings.HasSuffix(k2, ".pdf"), k2)
	require.False(t, storage.IsAbsoluteURL(k1))
	require.Equal(t, []byte(pdfA), readPriv(t, e.priv, k1))
	require.Equal(t, []byte(pdfB), readPriv(t, e.priv, k2))
	require.Equal(t, "crm/attachments/9/already-key.pdf", fileURL(t, id3))

	var ft string
	var fsz int
	require.NoError(t, testDB.Raw("SELECT file_type, file_size FROM attachment WHERE id = ?", id1).Row().Scan(&ft, &fsz))
	require.Equal(t, "application/pdf", ft)
	require.Equal(t, len(pdfA), fsz)

	// Không --delete-public ⇒ object public cũ còn nguyên.
	_, _, err = e.pub.Open(context.Background(), "crm/attachments/7/old-a.pdf")
	require.NoError(t, err)

	// Chạy lại: không còn gì để chuyển, không ghi thêm file.
	before := countFiles(t, e.priv.Root())
	rep2, err := e.migrator(true, false).Run(context.Background())
	require.NoError(t, err)
	require.Zero(t, rep2.Candidates)
	require.Zero(t, rep2.Migrated)
	require.Equal(t, 3, rep2.AlreadyKey)
	require.Equal(t, before, countFiles(t, e.priv.Root()))
	require.Equal(t, k1, fileURL(t, id1))
	require.Equal(t, k2, fileURL(t, id2))
}

func TestApply_BlocksForeignHostAndRedirect(t *testing.T) {
	e := newEnv(t)
	evil := "http://169.254.169.254/latest/meta-data/x.pdf"
	idEvil := insertAtt(t, 7, evil)
	idOther := insertAtt(t, 7, e.other.URL+"/b.pdf") // cùng IP 127.0.0.1 nhưng khác port ⇒ không thuộc allowlist
	idRedir := insertAtt(t, 7, e.cdn.URL+"/redir")
	idUserinfo := insertAtt(t, 7, "https://"+strings.TrimPrefix(e.cdn.URL, "http://")+"@evil.example/x.pdf")

	rep, err := e.migrator(true, false).Run(context.Background())
	require.NoError(t, err)
	require.Zero(t, rep.Migrated)
	require.Len(t, rep.Failed, 4)
	require.Zero(t, e.otherHits.Load(), "không được gọi tới host lạ (kể cả qua redirect)")
	require.Equal(t, evil, fileURL(t, idEvil))
	require.Equal(t, e.other.URL+"/b.pdf", fileURL(t, idOther))
	require.Equal(t, e.cdn.URL+"/redir", fileURL(t, idRedir))
	require.True(t, storage.IsAbsoluteURL(fileURL(t, idUserinfo)))
	require.Zero(t, countFiles(t, e.priv.Root()))
}

func TestApply_RejectsOversize(t *testing.T) {
	e := newEnv(t)
	id := insertAtt(t, 7, e.cdn.URL+"/legacy/big.pdf")
	m := e.migrator(true, false)
	m.Opts.MaxBytes = 1024
	rep, err := m.Run(context.Background())
	require.NoError(t, err)
	require.Len(t, rep.Failed, 1)
	require.Contains(t, rep.Failed[0].Reason, "kích thước")
	require.True(t, storage.IsAbsoluteURL(fileURL(t, id)))
	require.Zero(t, countFiles(t, e.priv.Root()))
}

func TestApply_UpdateFailure_CleansPrivate_KeepsPublic(t *testing.T) {
	e := newEnv(t)
	u1 := e.putPublic(t, "crm/attachments/7/old-a.pdf", pdfA)
	id1 := insertAtt(t, 7, u1)
	require.NoError(t, testDB.Exec(
		"CREATE TRIGGER trg_att_fail BEFORE UPDATE ON attachment FOR EACH ROW "+
			"BEGIN IF OLD.id = "+itoa(id1)+" THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'forced'; END IF; END").Error)
	t.Cleanup(func() { _ = testDB.Exec("DROP TRIGGER IF EXISTS trg_att_fail").Error })

	rep, err := e.migrator(true, true).Run(context.Background())
	require.NoError(t, err)
	require.Zero(t, rep.Migrated)
	require.Len(t, rep.Failed, 1)
	require.Contains(t, rep.Failed[0].Reason, "UPDATE lỗi")
	require.Contains(t, rep.Failed[0].Reason, "đã dọn object private")
	require.Zero(t, countFiles(t, e.priv.Root()), "object private vừa ghi phải bị dọn")
	require.Equal(t, u1, fileURL(t, id1))
	require.Zero(t, rep.PublicDeleted)
	_, _, err = e.pub.Open(context.Background(), "crm/attachments/7/old-a.pdf")
	require.NoError(t, err, "DB chưa đổi ⇒ KHÔNG được xoá object public")
}

func TestApply_DeletePublic_AfterDBChange(t *testing.T) {
	e := newEnv(t)
	u1 := e.putPublic(t, "crm/attachments/7/old-a.pdf", pdfA)
	id1 := insertAtt(t, 7, u1)
	// 2 bản ghi dùng chung 1 URL: object chỉ được xoá khi bản ghi cuối cùng đã đổi.
	uShared := e.putPublic(t, "crm/attachments/8/shared.pdf", pdfA)
	idS1 := insertAtt(t, 8, uShared)
	idS2 := insertAtt(t, 8, uShared)
	idHTTP := insertAtt(t, 9, e.cdn.URL+"/legacy/b.pdf")

	rep, err := e.migrator(true, true).Run(context.Background())
	require.NoError(t, err)
	require.Empty(t, rep.Failed)
	require.Equal(t, 4, rep.Migrated)
	require.Equal(t, 2, rep.PublicDeleted)
	require.Equal(t, 2, rep.PublicKept) // 1 lần "còn bản ghi khác dùng chung" + 1 nguồn HTTP ngoài kho

	for _, id := range []uint{id1, idS1, idS2, idHTTP} {
		require.False(t, storage.IsAbsoluteURL(fileURL(t, id)))
	}
	require.Equal(t, []byte(pdfA), readPriv(t, e.priv, fileURL(t, idS1)))
	require.Equal(t, []byte(pdfA), readPriv(t, e.priv, fileURL(t, idS2)))
	for _, k := range []string{"crm/attachments/7/old-a.pdf", "crm/attachments/8/shared.pdf"} {
		_, _, err := e.pub.Open(context.Background(), k)
		require.ErrorIs(t, err, storage.ErrNotFound, k)
	}
}

// Luồng 2 bước: --apply ghi manifest (KHÔNG xoá) → user kiểm → --delete-public-from xoá object cũ,
// chỉ cho bản ghi vẫn đang trỏ new_key.
func TestManifest_DeleteLater_OnlyWhenRowStillPointsToNewKey(t *testing.T) {
	e := newEnv(t)
	u1 := e.putPublic(t, "crm/attachments/7/a.pdf", pdfA)
	u2 := e.putPublic(t, "crm/attachments/7/b.pdf", pdfA)
	id1 := insertAtt(t, 7, u1)
	id2 := insertAtt(t, 7, u2)

	var man bytes.Buffer
	m := e.migrator(true, false)
	m.Opts.Manifest = &man
	rep, err := m.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, rep.Migrated)
	require.Equal(t, 2, strings.Count(man.String(), "\n"))
	for _, k := range []string{"crm/attachments/7/a.pdf", "crm/attachments/7/b.pdf"} {
		_, _, err := e.pub.Open(context.Background(), k)
		require.NoError(t, err, "apply không --delete-public ⇒ object cũ còn")
	}

	// Bản ghi 2 bị đổi sau đó (vd user thay file) ⇒ không được xoá theo manifest cũ.
	require.NoError(t, testDB.Exec("UPDATE attachment SET file_url = 'crm/attachments/7/other.pdf' WHERE id = ?", id2).Error)

	rep2, err := e.migrator(false, false).DeleteFromManifest(context.Background(), strings.NewReader(man.String()))
	require.NoError(t, err)
	require.Equal(t, 2, rep2.Candidates)
	require.Equal(t, 1, rep2.PublicDeleted)
	require.Equal(t, 1, rep2.PublicKept)
	_, _, err = e.pub.Open(context.Background(), "crm/attachments/7/a.pdf")
	require.ErrorIs(t, err, storage.ErrNotFound)
	_, _, err = e.pub.Open(context.Background(), "crm/attachments/7/b.pdf")
	require.NoError(t, err)
	require.False(t, storage.IsAbsoluteURL(fileURL(t, id1)))
}

func itoa(v uint) string { return strconv.FormatUint(uint64(v), 10) }
