// Package attachmentprivatize chuyển file đính kèm khách CŨ (bảng `attachment`, cột file_url là URL
// tuyệt đối public-read trên CDN/S3 cũ, trước 2026-10-19) sang kho PRIVATE, lưu lại KEY.
//
// Chạy 1 lần qua cmd/attachment-privatize. Logic nằm ở đây (không trong main) để test được bằng
// integration test với kho local + httptest giả CDN.
//
// Thứ tự an toàn cho MỖI bản ghi:
//  1. Đọc nội dung cũ: URL thuộc kho public hiện tại (storage.ToKey bóc được gốc URL) ⇒ đọc qua
//     kho public; còn lại ⇒ HTTP GET, CHỈ tới host trong allowlist (chống SSRF), không theo redirect
//     sang host lạ, giới hạn kích thước như upload (32MB).
//  2. Sniff MIME từ magic bytes theo whitelist attachment (như upload).
//  3. Save vào kho private với key theo quy ước upload: crm/attachments/<customer_id>/<random>-<unix><ext>.
//  4. UPDATE attachment theo id VÀ file_url cũ (đổi giữa chừng ⇒ RowsAffected=0 ⇒ coi là lỗi).
//     UPDATE lỗi ⇒ xoá object private vừa ghi (không để mồ côi).
//  5. CHỈ khi DB đã đổi thành công VÀ bật DeletePublic ⇒ xoá object public cũ (chỉ khi object đó
//     nằm trong kho public hiện tại và không còn bản ghi nào khác trỏ tới cùng URL).
//
// Idempotent: chỉ chọn bản ghi có file_url là URL tuyệt đối — bản ghi đã là key bị bỏ qua, chạy lại
// không đổi gì.
package attachmentprivatize

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"erpCluvix/pkg/storage"

	"github.com/gabriel-vasile/mimetype"
	"gorm.io/gorm"
)

const (
	// MaxFileBytes — trần kích thước như upload attachment (crm/attachment/handler/upload.go).
	MaxFileBytes int64 = 32 * 1024 * 1024
	// DefaultBatch — số bản ghi đọc mỗi trang.
	DefaultBatch = 200
	// KeyPrefix — quy ước key upload attachment hiện tại: KeyPrefix + <customer_id> + "/".
	KeyPrefix = "crm/attachments/"

	httpTimeout  = 2 * time.Minute
	maxRedirects = 5
	sniffBytes   = 512 // như storage.DetectMIMEType / detectAllowed lúc upload
)

// Options — cấu hình 1 lần chạy.
type Options struct {
	Apply        bool     // false ⇒ dry-run: chỉ liệt kê, KHÔNG ghi kho, KHÔNG ghi DB
	DeletePublic bool     // xoá object public cũ sau khi DB đổi thành công (chỉ khi Apply)
	Limit        int      // tối đa số bản ghi cần chuyển xử lý; <=0 ⇒ không giới hạn
	Batch        int      // cỡ trang SELECT; <=0 ⇒ DefaultBatch
	AllowHosts   []string // host được phép tải qua HTTP ("host" khớp mọi port, "host:port" khớp đúng)
	MaxBytes     int64    // <=0 ⇒ MaxFileBytes
	HTTPClient   *http.Client
	Log          io.Writer // nil ⇒ io.Discard
	// Manifest — nhận 1 dòng CSV `id,old_url,new_key` cho MỖI bản ghi đã UPDATE thành công (apply).
	// Dùng cho DeleteFromManifest: xoá object public cũ ở lần chạy SAU, khi user đã kiểm xong.
	Manifest io.Writer
}

// Failure — 1 bản ghi lỗi.
type Failure struct {
	ID     uint
	Reason string
}

// Report — kết quả cuối.
type Report struct {
	Total         int // tổng bản ghi attachment trong DB lúc bắt đầu
	AlreadyKey    int // đã là key (bỏ qua — idempotent)
	Candidates    int // bản ghi URL tuyệt đối đã xét (sau --limit)
	Planned       int // dry-run: số bản ghi sẽ chuyển
	Migrated      int // apply: đã chuyển + đổi DB
	PublicDeleted int // apply + DeletePublic: object public cũ đã xoá
	PublicKept    int // apply + DeletePublic: không xoá được/không được phép xoá (lý do trong log)
	Failed        []Failure
}

// Migrator — chạy chuyển đổi.
type Migrator struct {
	DB      *gorm.DB
	Public  storage.Store // kho public HIỆN TẠI (nơi chứa object cũ nếu URL thuộc kho này)
	Private storage.Store // kho private ĐÍCH
	Opts    Options

	allow    []string
	client   *http.Client
	manifest *csv.Writer
}

type row struct {
	ID         uint
	CustomerID uint
	FileURL    string
}

// sourceKind — nơi đọc nội dung cũ.
type sourceKind int

const (
	srcPublicStore sourceKind = iota + 1
	srcHTTP
)

type source struct {
	kind sourceKind
	key  string   // srcPublicStore
	url  *url.URL // srcHTTP
}

// HostsFromConfig trả host (kèm port nếu có) của các URL cấu hình (S3_CDN, S3_ENDPOINT,
// STORAGE_PUBLIC_BASE...). Giá trị rỗng/không parse được bị bỏ qua.
func HostsFromConfig(urls ...string) []string {
	var out []string
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if !strings.Contains(u, "://") {
			u = "https://" + u
		}
		p, err := url.Parse(u)
		if err != nil || p.Host == "" {
			continue
		}
		out = append(out, strings.ToLower(p.Host))
	}
	return out
}

// Run xử lý toàn bộ (hoặc tới Limit) bản ghi có file_url là URL tuyệt đối.
func (m *Migrator) Run(ctx context.Context) (Report, error) {
	var rep Report
	if m.DB == nil || m.Public == nil || m.Private == nil {
		return rep, errors.New("attachmentprivatize: thiếu DB/kho public/kho private")
	}
	if m.Private.Public() {
		return rep, errors.New("attachmentprivatize: kho đích không phải kho private — từ chối chạy")
	}
	m.init()

	var total int64
	if err := m.DB.WithContext(ctx).Table("attachment").Count(&total).Error; err != nil {
		return rep, fmt.Errorf("đếm attachment: %w", err)
	}
	rep.Total = int(total)
	var legacy int64
	if err := legacyScope(m.DB.WithContext(ctx).Table("attachment")).Count(&legacy).Error; err != nil {
		return rep, fmt.Errorf("đếm attachment URL cũ: %w", err)
	}
	rep.AlreadyKey = rep.Total - int(legacy)

	batch := m.Opts.Batch
	if batch <= 0 {
		batch = DefaultBatch
	}
	var lastID uint
	for {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		if m.Opts.Limit > 0 && rep.Candidates >= m.Opts.Limit {
			break
		}
		n := batch
		if m.Opts.Limit > 0 && m.Opts.Limit-rep.Candidates < n {
			n = m.Opts.Limit - rep.Candidates
		}
		var rows []row
		err := legacyScope(m.DB.WithContext(ctx).Table("attachment")).
			Select("id, customer_id, file_url").
			Where("id > ?", lastID).Order("id ASC").Limit(n).
			Scan(&rows).Error
		if err != nil {
			return rep, fmt.Errorf("đọc attachment sau id %d: %w", lastID, err)
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			if err := ctx.Err(); err != nil {
				return rep, err
			}
			lastID = r.ID
			rep.Candidates++
			m.process(ctx, r, &rep)
		}
	}
	return rep, nil
}

// legacyScope — bản ghi còn lưu URL tuyệt đối (đồng nhất với storage.IsAbsoluteURL).
func legacyScope(db *gorm.DB) *gorm.DB {
	return db.Where("(LOWER(file_url) LIKE 'http://%' OR LOWER(file_url) LIKE 'https://%' OR file_url LIKE '//%')")
}

func (m *Migrator) init() {
	if m.Opts.MaxBytes <= 0 {
		m.Opts.MaxBytes = MaxFileBytes
	}
	if m.Opts.Log == nil {
		m.Opts.Log = io.Discard
	}
	m.allow = nil
	m.manifest = nil
	if m.Opts.Manifest != nil {
		m.manifest = csv.NewWriter(m.Opts.Manifest)
	}
	for _, h := range m.Opts.AllowHosts {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			m.allow = append(m.allow, h)
		}
	}
	base := m.Opts.HTTPClient
	if base == nil {
		base = &http.Client{Timeout: httpTimeout}
	}
	c := *base
	if c.Timeout <= 0 {
		c.Timeout = httpTimeout
	}
	// Redirect CHỈ được đi tiếp khi đích vẫn là http/https và thuộc allowlist — một CDN bị chiếm
	// hoặc cấu hình lạ không thể dẫn lệnh này đi gọi metadata cloud / dịch vụ nội bộ.
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("quá %d lần redirect", maxRedirects)
		}
		if err := m.checkURL(req.URL); err != nil {
			return fmt.Errorf("redirect bị chặn: %w", err)
		}
		return nil
	}
	m.client = &c
}

func (m *Migrator) logf(format string, args ...any) {
	fmt.Fprintf(m.Opts.Log, format+"\n", args...)
}

func (m *Migrator) fail(rep *Report, id uint, reason string) {
	rep.Failed = append(rep.Failed, Failure{ID: id, Reason: reason})
	m.logf("  [LỖI]  id=%d: %s", id, reason)
}

// checkURL — chỉ http/https, host thuộc allowlist.
func (m *Migrator) checkURL(u *url.URL) error {
	if u == nil {
		return errors.New("URL rỗng")
	}
	if s := strings.ToLower(u.Scheme); s != "http" && s != "https" {
		return fmt.Errorf("scheme %q không được phép (chỉ http/https)", u.Scheme)
	}
	host := strings.ToLower(u.Host)
	name := strings.ToLower(u.Hostname())
	if name == "" {
		return errors.New("URL không có host")
	}
	for _, a := range m.allow {
		if a == host {
			return nil
		}
		// Mục không kèm port ⇒ khớp theo hostname, mọi port. Kèm port ⇒ chỉ khớp đúng host:port.
		if _, _, err := net.SplitHostPort(a); err != nil && strings.Trim(a, "[]") == name {
			return nil
		}
	}
	return fmt.Errorf("host %q không thuộc allowlist (S3_CDN/S3_ENDPOINT/STORAGE_PUBLIC_BASE hoặc --allow-host)", u.Host)
}

// resolveSource quyết định đọc nội dung cũ từ đâu.
func (m *Migrator) resolveSource(raw string) (source, error) {
	if k := storage.ToKey(m.Public, raw); k != raw && k != "" {
		if err := storage.SafeKey(k); err != nil {
			return source{}, fmt.Errorf("key suy từ URL không an toàn: %q", k)
		}
		return source{kind: srcPublicStore, key: k}, nil
	}
	v := raw
	if strings.HasPrefix(v, "//") {
		v = "https:" + v
	}
	u, err := url.Parse(v)
	if err != nil {
		return source{}, fmt.Errorf("URL không hợp lệ: %v", err)
	}
	if err := m.checkURL(u); err != nil {
		return source{}, err
	}
	return source{kind: srcHTTP, url: u}, nil
}

func (s source) String() string {
	switch s.kind {
	case srcPublicStore:
		return "kho public key=" + s.key
	case srcHTTP:
		return "HTTP " + s.url.Redacted()
	default:
		return "ngoài kho public"
	}
}

func (m *Migrator) process(ctx context.Context, r row, rep *Report) {
	src, err := m.resolveSource(r.FileURL)
	if err != nil {
		m.fail(rep, r.ID, err.Error())
		return
	}
	if r.CustomerID == 0 {
		m.fail(rep, r.ID, "customer_id = 0 — không dựng được key theo quy ước")
		return
	}
	if !m.Opts.Apply {
		rep.Planned++
		m.logf("  [DRY]  id=%d customer=%d nguồn: %s", r.ID, r.CustomerID, src)
		return
	}

	data, err := m.fetch(ctx, src)
	if err != nil {
		m.fail(rep, r.ID, "đọc nguồn ("+src.String()+"): "+err.Error())
		return
	}
	mt, err := sniff(data)
	if err != nil {
		m.fail(rep, r.ID, err.Error())
		return
	}

	key := storage.NewKey(KeyPrefix+strconv.FormatUint(uint64(r.CustomerID), 10)+"/", mt.Extension())
	ctype := storage.BaseMIME(mt.String())
	if err := m.Private.Save(ctx, bytes.NewReader(data), key, ctype); err != nil {
		m.fail(rep, r.ID, "ghi kho private: "+err.Error())
		return
	}

	// WHERE thêm file_url cũ: bản ghi bị sửa giữa chừng ⇒ RowsAffected=0 ⇒ không đè lên giá trị mới.
	// file_type/file_size cập nhật theo nội dung thật (MIME đã sniff), như bản ghi upload mới.
	tx := m.DB.WithContext(ctx).Table("attachment").
		Where("id = ? AND file_url = ?", r.ID, r.FileURL).
		Updates(map[string]any{"file_url": key, "file_type": mt.String(), "file_size": len(data)})
	if tx.Error != nil || tx.RowsAffected != 1 {
		reason := fmt.Sprintf("UPDATE không đổi đúng 1 dòng (rows=%d)", tx.RowsAffected)
		if tx.Error != nil {
			reason = "UPDATE lỗi: " + tx.Error.Error()
		}
		// Dọn object private vừa ghi — context riêng: ctx của caller có thể vừa bị huỷ (Ctrl-C).
		if delErr := m.Private.Delete(context.Background(), key); delErr != nil {
			reason += fmt.Sprintf("; dọn object private %q lỗi: %v", key, delErr)
		} else {
			reason += "; đã dọn object private vừa ghi"
		}
		m.fail(rep, r.ID, reason)
		return
	}
	rep.Migrated++
	m.logf("  [OK]   id=%d → %s (%s, %d byte)", r.ID, key, ctype, len(data))
	if m.manifest != nil {
		_ = m.manifest.Write([]string{strconv.FormatUint(uint64(r.ID), 10), r.FileURL, key})
		m.manifest.Flush()
		if err := m.manifest.Error(); err != nil {
			m.logf("  [CẢNH BÁO] id=%d: ghi manifest lỗi: %v", r.ID, err)
		}
	}

	if m.Opts.DeletePublic {
		m.deletePublic(ctx, r, src, rep)
	}
}

// DeleteFromManifest xoá object public cũ theo manifest của một lần --apply trước đó. Mỗi dòng chỉ
// được xoá khi bản ghi HIỆN đang trỏ đúng new_key (DB đã đổi và chưa bị đổi tiếp) — cùng điều kiện
// của deletePublic. Dòng sai định dạng/không thoả ⇒ giữ object, ghi lý do.
func (m *Migrator) DeleteFromManifest(ctx context.Context, r io.Reader) (Report, error) {
	var rep Report
	if m.DB == nil || m.Public == nil {
		return rep, errors.New("attachmentprivatize: thiếu DB/kho public")
	}
	m.init()
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = 3
	for {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return rep, fmt.Errorf("đọc manifest: %w", err)
		}
		rep.Candidates++
		id64, err := strconv.ParseUint(rec[0], 10, 32)
		if err != nil || id64 == 0 {
			m.fail(&rep, 0, fmt.Sprintf("dòng manifest id không hợp lệ: %q", rec[0]))
			continue
		}
		id, oldURL, newKey := uint(id64), rec[1], rec[2]
		var cur []string
		if err := m.DB.WithContext(ctx).Table("attachment").Where("id = ?", id).Limit(1).
			Pluck("file_url", &cur).Error; err != nil {
			m.fail(&rep, id, "đọc bản ghi: "+err.Error())
			continue
		}
		if len(cur) == 0 || cur[0] != newKey {
			rep.PublicKept++
			m.logf("  [GIỮ]  id=%d: bản ghi không còn trỏ %q — không xoá object cũ", id, newKey)
			continue
		}
		src, err := m.resolveSource(oldURL)
		if err != nil {
			src = source{} // URL ngoài allowlist vẫn có thể là object ngoài kho ⇒ để deletePublic báo GIỮ
		}
		m.deletePublic(ctx, row{ID: id, FileURL: oldURL}, src, &rep)
	}
	return rep, nil
}

// deletePublic — chỉ gọi SAU khi UPDATE đã thành công.
func (m *Migrator) deletePublic(ctx context.Context, r row, src source, rep *Report) {
	if src.kind != srcPublicStore {
		rep.PublicKept++
		m.logf("  [GIỮ]  id=%d: object cũ không thuộc kho public hiện tại (%s) — xoá tay nếu cần", r.ID, src)
		return
	}
	// Bản ghi khác vẫn trỏ cùng URL ⇒ chưa xoá; bản ghi cuối cùng được chuyển sẽ xoá.
	var others int64
	if err := m.DB.WithContext(ctx).Table("attachment").
		Where("file_url = ? AND id <> ?", r.FileURL, r.ID).Count(&others).Error; err != nil {
		rep.PublicKept++
		m.logf("  [GIỮ]  id=%d: không kiểm được bản ghi dùng chung URL (%v) — chưa xoá", r.ID, err)
		return
	}
	if others > 0 {
		rep.PublicKept++
		m.logf("  [GIỮ]  id=%d: còn %d bản ghi khác trỏ cùng URL — chưa xoá", r.ID, others)
		return
	}
	if err := m.Public.Delete(ctx, src.key); err != nil {
		rep.PublicKept++
		m.logf("  [GIỮ]  id=%d: xoá object public %q lỗi: %v", r.ID, src.key, err)
		return
	}
	rep.PublicDeleted++
	m.logf("  [XOÁ]  id=%d: đã xoá object public %q", r.ID, src.key)
}

var errTooLarge = errors.New("vượt giới hạn kích thước")

func (m *Migrator) fetch(ctx context.Context, src source) ([]byte, error) {
	switch src.kind {
	case srcPublicStore:
		rc, _, err := m.Public.Open(ctx, src.key)
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return readLimited(rc, m.Opts.MaxBytes)
	case srcHTTP:
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.url.String(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := m.client.Do(req)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				return nil, fmt.Errorf("hết thời gian chờ: %w", err)
			}
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		if resp.ContentLength > m.Opts.MaxBytes {
			return nil, fmt.Errorf("%w (%d > %d byte)", errTooLarge, resp.ContentLength, m.Opts.MaxBytes)
		}
		return readLimited(resp.Body, m.Opts.MaxBytes)
	default:
		return nil, errors.New("nguồn không xác định")
	}
}

func readLimited(r io.Reader, max int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("%w (> %d byte)", errTooLarge, max)
	}
	return data, nil
}

// sniff — như upload: magic bytes của 512 byte đầu, phải thuộc whitelist attachment.
func sniff(data []byte) (*mimetype.MIME, error) {
	if len(data) == 0 {
		return nil, errors.New("file rỗng")
	}
	head := data
	if len(head) > sniffBytes {
		head = head[:sniffBytes]
	}
	mt := mimetype.Detect(head)
	for _, a := range storage.AttachmentMIMETypes {
		if mt.Is(a) {
			return mt, nil
		}
	}
	return nil, fmt.Errorf("loại file '%s' không thuộc whitelist attachment", mt.String())
}
