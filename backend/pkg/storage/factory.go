package storage

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Tên driver hợp lệ cho STORAGE_DRIVER.
const (
	DriverLocal = "local"
	DriverS3    = "s3"
	// DriverR2 — Cloudflare R2: dùng lại driver S3, chỉ khác lớp chuẩn hoá cấu hình (r2.go).
	DriverR2 = "r2"

	// ACLNone — giá trị S3_PUBLIC_ACL để KHÔNG gửi ACL (bucket policy quyết định quyền đọc).
	ACLNone = "none"

	privateFolder = "private"
)

// Config — đủ để dựng cả 2 kho (public + private) cho bất kỳ driver nào. Map 1-1 từ env (config.go).
type Config struct {
	Driver string // STORAGE_DRIVER

	LocalPath         string // STORAGE_PATH          (kho public local)
	LocalPrivatePath  string // STORAGE_PRIVATE_PATH  (kho private local — KHÔNG mount static)
	LocalPublicPrefix string // STORAGE_PUBLIC_PREFIX (vd /uploads)
	LocalPublicBase   string // STORAGE_PUBLIC_BASE   (gốc URL tuyệt đối; rỗng ⇒ URL tương đối)

	S3Key           string
	S3Secret        string
	S3Region        string
	S3Bucket        string
	S3PrivateBucket string // S3_PRIVATE_BUCKET — s3: mặc định = S3_BUCKET; r2: BẮT BUỘC, khác S3_BUCKET
	S3Endpoint      string
	S3CDN           string
	S3Folder        string
	S3PublicACL     string // S3_PUBLIC_ACL — rỗng ⇒ public-read; "none" ⇒ không gửi ACL

	// SigningKey — STORAGE_SIGNING_KEY: khoá HMAC ký URL ngắn hạn của kho private driver local
	// (s3/r2 ký bằng chính S3_SECRET qua presign, không dùng khoá này). Prod + driver local mà trống
	// hoặc ngắn hơn MinSigningKeyLen ⇒ lỗi (dừng khởi động); dev trống ⇒ DevSigningKey + log cảnh báo.
	SigningKey string
	// Prod — cfg.IsProduction(): bật các ràng buộc chỉ áp ở prod (khoá ký bắt buộc).
	Prod bool
}

// ResolveDriver chuẩn hoá STORAGE_DRIVER. Trống mà có S3_KEY ⇒ suy ra s3 (tương thích cấu hình cũ
// chỉ khai S3_*), inferred=true để caller cảnh báo nên đặt tường minh. Trống cả hai ⇒ local.
func ResolveDriver(driver, s3Key string) (resolved string, inferred bool, err error) {
	d := strings.ToLower(strings.TrimSpace(driver))
	switch d {
	case DriverLocal, DriverS3, DriverR2:
		return d, false, nil
	case "":
		if strings.TrimSpace(s3Key) != "" {
			return DriverS3, true, nil
		}
		return DriverLocal, false, nil
	default:
		return "", false, fmt.Errorf("STORAGE_DRIVER=%q không hợp lệ, chỉ nhận %q, %q hoặc %q",
			driver, DriverLocal, DriverS3, DriverR2)
	}
}

// New dựng kho public + private theo cấu hình. Đây là nơi DUY NHẤT quyết định dùng driver nào.
//
// Cấu hình sai/thiếu ⇒ trả lỗi để bootstrap DỪNG khởi động. KHÔNG BAO GIỜ âm thầm rơi về local hay
// trả kho nil: chạy tiếp ở production nghĩa là file biến mất sau mỗi lần deploy (local) hoặc mọi
// upload lỗi 500 mà không ai biết vì sao.
func New(cfg Config) (public Store, private Store, err error) {
	driver, inferred, err := ResolveDriver(cfg.Driver, cfg.S3Key)
	if err != nil {
		return nil, nil, err
	}
	if inferred {
		log.Printf("storage: STORAGE_DRIVER trống nhưng có S3_KEY ⇒ dùng driver s3 (tương thích cấu hình cũ) — nên đặt STORAGE_DRIVER=s3 tường minh")
	}
	switch driver {
	case DriverS3:
		return newS3Pair(cfg)
	case DriverR2:
		return newR2Pair(cfg)
	default:
		return newLocalPair(cfg)
	}
}

// ── local ────────────────────────────────────────────────────────────────

func newLocalPair(cfg Config) (Store, Store, error) {
	pubPath := orDefault(cfg.LocalPath, "./storage/uploads")
	privPath := orDefault(cfg.LocalPrivatePath, "./storage/private")
	prefix := orDefault(cfg.LocalPublicPrefix, "/uploads")
	if strings.Trim(prefix, "/") == "" {
		return nil, nil, fmt.Errorf("STORAGE_PUBLIC_PREFIX=%q không hợp lệ — phải là đường dẫn con, vd /uploads", cfg.LocalPublicPrefix)
	}

	pubAbs, err := filepath.Abs(pubPath)
	if err != nil {
		return nil, nil, fmt.Errorf("STORAGE_PATH=%q: %w", pubPath, err)
	}
	privAbs, err := filepath.Abs(privPath)
	if err != nil {
		return nil, nil, fmt.Errorf("STORAGE_PRIVATE_PATH=%q: %w", privPath, err)
	}
	// Thư mục private nằm trong (hoặc trùng) thư mục public ⇒ file riêng tư bị phục vụ công khai
	// qua static /uploads. Ngược lại (public nằm trong private) cũng chặn cho rõ ràng.
	if nested(pubAbs, privAbs) || nested(privAbs, pubAbs) {
		return nil, nil, fmt.Errorf("STORAGE_PRIVATE_PATH (%s) không được trùng hoặc lồng với STORAGE_PATH (%s) — "+
			"thư mục public được phục vụ static, file riêng tư sẽ bị lộ", privAbs, pubAbs)
	}

	pub, err := NewLocal(pubAbs, prefix, cfg.LocalPublicBase, true)
	if err != nil {
		return nil, nil, fmt.Errorf("STORAGE_PATH=%q: %w", pubPath, err)
	}
	privLocal, err := NewLocal(privAbs, "", "", false)
	if err != nil {
		return nil, nil, fmt.Errorf("STORAGE_PRIVATE_PATH=%q: %w", privPath, err)
	}
	signKey, err := localSigningKey(cfg)
	if err != nil {
		return nil, nil, err
	}
	// URL ký trỏ về chính API (STORAGE_PUBLIC_BASE = gốc API khi FE khác origin; trống ⇒ tương đối).
	priv, err := NewLocalSigned(privLocal, signKey, cfg.LocalPublicBase)
	if err != nil {
		return nil, nil, err
	}

	if strings.TrimSpace(cfg.LocalPublicBase) == "" {
		log.Printf("storage: driver local, STORAGE_PUBLIC_BASE trống ⇒ URL ảnh tương đối %s/… — FE khác origin với API phải khai STORAGE_PUBLIC_BASE (hoặc proxy %s về backend)", prefix, prefix)
	}
	log.Printf("storage: driver local — public %q, private %q; 2 thư mục này PHẢI nằm ngoài vòng đời deploy (volume/bind mount)", pubAbs, privAbs)
	return pub, priv, nil
}

// localSigningKey — STORAGE_SIGNING_KEY cho driver local. Prod: bắt buộc, ≥ MinSigningKeyLen byte,
// không được là DevSigningKey. Dev: trống ⇒ DevSigningKey (công khai trong mã nguồn) + log cảnh báo.
func localSigningKey(cfg Config) ([]byte, error) {
	k := strings.TrimSpace(cfg.SigningKey)
	if cfg.Prod {
		if k == "" || k == DevSigningKey {
			return nil, fmt.Errorf("STORAGE_DRIVER=local trên production BẮT BUỘC STORAGE_SIGNING_KEY (chuỗi ngẫu nhiên ≥ %d ký tự, vd `openssl rand -hex 32`) — khoá ký URL xem/tải file đính kèm riêng tư", MinSigningKeyLen)
		}
		if len(k) < MinSigningKeyLen {
			return nil, fmt.Errorf("STORAGE_SIGNING_KEY quá ngắn (%d ký tự) — cần ≥ %d", len(k), MinSigningKeyLen)
		}
		return []byte(k), nil
	}
	if k == "" {
		log.Printf("storage: STORAGE_SIGNING_KEY trống ⇒ dùng khoá DEV công khai để ký URL file đính kèm — KHÔNG dùng ở production")
		return []byte(DevSigningKey), nil
	}
	return []byte(k), nil
}

// nested — child trùng hoặc nằm trong parent.
func nested(parent, child string) bool {
	parent, child = filepath.Clean(parent), filepath.Clean(child)
	return child == parent || strings.HasPrefix(child, parent+string(os.PathSeparator))
}

// ── s3 ───────────────────────────────────────────────────────────────────

func newS3Pair(cfg Config) (Store, Store, error) {
	var missing []string
	if strings.TrimSpace(cfg.S3Key) == "" {
		missing = append(missing, "S3_KEY")
	}
	if strings.TrimSpace(cfg.S3Secret) == "" {
		missing = append(missing, "S3_SECRET")
	}
	if strings.TrimSpace(cfg.S3Bucket) == "" {
		missing = append(missing, "S3_BUCKET")
	}
	if strings.TrimSpace(cfg.S3Endpoint) == "" && strings.TrimSpace(cfg.S3Region) == "" {
		missing = append(missing, "S3_ENDPOINT hoặc S3_REGION")
	}
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("STORAGE_DRIVER=s3 nhưng thiếu: %s", strings.Join(missing, ", "))
	}

	acl, err := parsePublicACL(cfg.S3PublicACL)
	if err != nil {
		return nil, nil, err
	}

	base := baseS3(cfg)
	pubCfg := base
	pubCfg.ACL = acl
	pubCfg.Public = true

	privCfg := base
	privCfg.Bucket = orDefault(cfg.S3PrivateBucket, cfg.S3Bucket)
	privCfg.Folder = joinFolder(cfg.S3Folder, privateFolder)
	privCfg.ACL = types.ObjectCannedACLPrivate
	privCfg.CDN = ""
	privCfg.Public = false

	if strings.TrimSpace(privCfg.Bucket) == strings.TrimSpace(cfg.S3Bucket) {
		log.Printf("storage: kho private dùng CHUNG bucket %q (folder %q, ACL private) — nếu bucket có policy public-read toàn bucket thì file private vẫn đọc được: giới hạn policy vào folder public hoặc đặt S3_PRIVATE_BUCKET riêng", privCfg.Bucket, privCfg.Folder)
	}
	return buildPair(pubCfg, privCfg, func(c S3Config) (*S3, error) { return NewS3(c) })
}

// parsePublicACL — S3_PUBLIC_ACL: rỗng ⇒ public-read (tương thích S3 cũ: MinIO/S3Cloud chưa có
// bucket policy); "none" ⇒ không gửi ACL; còn lại phải là canned ACL hợp lệ.
func parsePublicACL(v string) (types.ObjectCannedACL, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "":
		return types.ObjectCannedACLPublicRead, nil
	case ACLNone:
		return "", nil
	}
	for _, a := range types.ObjectCannedACL("").Values() {
		if string(a) == v {
			return a, nil
		}
	}
	return "", fmt.Errorf("S3_PUBLIC_ACL=%q không hợp lệ — dùng public-read, none, hoặc một canned ACL của S3", v)
}

// ── r2 ───────────────────────────────────────────────────────────────────

func newR2Pair(cfg Config) (Store, Store, error) {
	var missing []string
	for _, kv := range [][2]string{
		{"S3_KEY", cfg.S3Key}, {"S3_SECRET", cfg.S3Secret}, {"S3_BUCKET", cfg.S3Bucket},
		{"S3_ENDPOINT", cfg.S3Endpoint}, {"S3_CDN", cfg.S3CDN}, {"S3_PRIVATE_BUCKET", cfg.S3PrivateBucket},
	} {
		if strings.TrimSpace(kv[1]) == "" {
			missing = append(missing, kv[0])
		}
	}
	if len(missing) > 0 {
		// Nêu lý do riêng của R2 để người cài không phải đoán vì sao S3 thường không cần.
		return nil, nil, fmt.Errorf("STORAGE_DRIVER=r2 nhưng thiếu: %s — R2: S3_ENDPOINT=https://<account_id>.r2.cloudflarestorage.com; "+
			"S3_CDN = domain r2.dev đã bật Public Access hoặc custom domain (endpoint API không phục vụ file công khai); "+
			"S3_PRIVATE_BUCKET = bucket RIÊNG không bật public (R2 không có ACL object, public là cấp bucket)",
			strings.Join(missing, ", "))
	}
	if strings.TrimSpace(cfg.S3PrivateBucket) == strings.TrimSpace(cfg.S3Bucket) {
		return nil, nil, fmt.Errorf("STORAGE_DRIVER=r2: S3_PRIVATE_BUCKET phải KHÁC S3_BUCKET — R2 public là cấp bucket, dùng chung bucket public là lộ file riêng tư")
	}

	base := baseS3(cfg)
	pubCfg := base
	pubCfg.Public = true

	privCfg := base
	privCfg.Bucket = strings.TrimSpace(cfg.S3PrivateBucket)
	privCfg.Folder = joinFolder(cfg.S3Folder, privateFolder)
	privCfg.CDN = ""
	privCfg.Public = false

	return buildPair(pubCfg, privCfg, newR2)
}

func baseS3(cfg Config) S3Config {
	return S3Config{
		Key:      strings.TrimSpace(cfg.S3Key),
		Secret:   strings.TrimSpace(cfg.S3Secret),
		Region:   strings.TrimSpace(cfg.S3Region),
		Bucket:   strings.TrimSpace(cfg.S3Bucket),
		Endpoint: strings.TrimSpace(cfg.S3Endpoint),
		CDN:      strings.TrimSpace(cfg.S3CDN),
		Folder:   strings.TrimSpace(cfg.S3Folder),
	}
}

func buildPair(pubCfg, privCfg S3Config, mk func(S3Config) (*S3, error)) (Store, Store, error) {
	pub, err := mk(pubCfg)
	if err != nil {
		return nil, nil, err
	}
	priv, err := mk(privCfg)
	if err != nil {
		return nil, nil, err
	}
	return pub, priv, nil
}

func joinFolder(folder, sub string) string {
	f := strings.Trim(strings.TrimSpace(folder), "/")
	if f == "" {
		return sub
	}
	return path.Join(f, sub)
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}
