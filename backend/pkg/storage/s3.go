package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// ── Deadline cho thao tác S3 ─────────────────────────────────
//
// Mọi lời gọi SDK đều có trần thời gian: S3 treo mà không deadline = goroutine treo VĨNH VIỄN.
const (
	// defaultOpTimeout — thao tác nhẹ: delete, head.
	defaultOpTimeout = 60 * time.Second

	// uploadBaseTimeout / uploadMinThroughput — deadline upload tính THEO DUNG LƯỢNG, không cố
	// định: đường truyền phòng khám có thể rất chậm, deadline cứng sẽ cắt oan file lớn.
	// 256KB/s là mức sàn thận trọng, KHÔNG phải băng thông kỳ vọng.
	uploadBaseTimeout   = 30 * time.Second
	uploadMinThroughput = 256 * 1024 // byte/giây
	uploadMaxTimeout    = 10 * time.Minute

	// downloadMaxTimeout — GET object (stream về client). Phủ CẢ việc đọc body, không chỉ lời gọi
	// GetObject: huỷ context là đứt luôn stream.
	downloadMaxTimeout = 10 * time.Minute
)

// uploadTimeout — deadline co giãn theo dung lượng, chặn trần uploadMaxTimeout.
// size ≤ 0 (nguồn io.Reader không biết trước độ dài) → dùng thẳng trần, KHÔNG dùng deadline ngắn.
func uploadTimeout(size int64) time.Duration {
	if size <= 0 {
		return uploadMaxTimeout
	}
	// Chặn trần TRƯỚC khi đổi sang time.Duration — size rất lớn nhân với nanosecond sẽ tràn int64.
	seconds := size / uploadMinThroughput
	if seconds > int64(uploadMaxTimeout/time.Second) {
		return uploadMaxTimeout
	}
	timeout := uploadBaseTimeout + time.Duration(seconds)*time.Second
	if timeout > uploadMaxTimeout {
		timeout = uploadMaxTimeout
	}
	return timeout
}

// readerLen — độ dài nếu reader tự biết (bytes.Reader, strings.Reader...), -1 nếu không.
func readerLen(r io.Reader) int64 {
	if l, ok := r.(interface{ Len() int }); ok {
		return int64(l.Len())
	}
	return -1
}

// S3Config — cấu hình MỘT kho trên object storage tương thích S3 (S3Cloud, MinIO, AWS, R2...).
type S3Config struct {
	Key      string
	Secret   string
	Region   string
	Bucket   string
	Endpoint string // rỗng ⇒ AWS S3 chuẩn (virtual-host); có ⇒ path-style qua endpoint này
	CDN      string // gốc URL công khai; rỗng ⇒ URL qua endpoint/bucket
	Folder   string // prefix trong bucket — tách dev/staging/prod dùng chung 1 bucket
	// ACL đặt cho MỖI object khi ghi. Rỗng ⇒ KHÔNG gửi ACL (bucket policy quyết định — bắt buộc
	// với R2 vì R2 không hỗ trợ ACL object). "public-read" cho S3 đời cũ (MinIO/S3Cloud) mà
	// bucket chưa có policy public; "private" cho kho private.
	ACL    types.ObjectCannedACL
	Public bool
}

// S3 — driver lưu file trên object storage tương thích S3.
type S3 struct {
	client   *s3.Client
	bucket   string
	endpoint string
	region   string
	cdn      string
	folder   string
	acl      types.ObjectCannedACL
	public   bool
}

var _ Store = (*S3)(nil)

// NewS3 dựng driver. opts để preset nhà cung cấp (R2) chỉnh client mà KHÔNG nhân bản
// Save/Open/Delete/URL — áp SAU mặc định nên preset đè được.
func NewS3(cfg S3Config, opts ...func(*s3.Options)) (*S3, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("storage s3: thiếu bucket")
	}
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	client := s3.NewFromConfig(aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.Key, cfg.Secret, ""),
	}, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			// Path-style: https://<endpoint>/<bucket>/<key> — không phụ thuộc wildcard DNS từng
			// bucket (S3Cloud/MinIO tài liệu hoá kiểu này).
			o.UsePathStyle = true
		}
		for _, fn := range opts {
			fn(o)
		}
	})
	return &S3{
		client:   client,
		bucket:   strings.TrimSpace(cfg.Bucket),
		endpoint: endpoint,
		region:   cfg.Region,
		cdn:      strings.TrimRight(strings.TrimSpace(cfg.CDN), "/"),
		folder:   strings.Trim(strings.TrimSpace(cfg.Folder), "/"),
		acl:      cfg.ACL,
		public:   cfg.Public,
	}, nil
}

func (s *S3) Public() bool { return s.public }

// objectKey ghép folder prefix vào key. DB KHÔNG lưu prefix ⇒ đổi S3_FOLDER không sửa dữ liệu.
func (s *S3) objectKey(key string) string {
	key = strings.TrimLeft(key, "/")
	if s.folder == "" {
		return key
	}
	return s.folder + "/" + key
}

func (s *S3) Save(ctx context.Context, r io.Reader, key, contentType string) error {
	if err := SafeKey(key); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, uploadTimeout(readerLen(r)))
	defer cancel()

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
		Body:   r,
	}
	if s.acl != "" {
		input.ACL = s.acl
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	_, err := s.client.PutObject(ctx, input)
	return err
}

// cancelOnClose — huỷ context của GetObject khi caller đóng body (không sớm hơn: huỷ sớm là đứt stream).
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

func (s *S3) Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	if err := SafeKey(key); err != nil {
		return nil, ObjectInfo{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, downloadMaxTimeout)
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
	})
	if err != nil {
		cancel()
		if isNotFound(err) {
			return nil, ObjectInfo{}, ErrNotFound
		}
		return nil, ObjectInfo{}, err
	}
	info := ObjectInfo{Size: -1, ContentType: aws.ToString(out.ContentType)}
	if out.ContentLength != nil {
		info.Size = *out.ContentLength
	}
	return cancelOnClose{ReadCloser: out.Body, cancel: cancel}, info, nil
}

// Delete — key không tồn tại KHÔNG phải lỗi (giữ ngữ nghĩa giống driver local).
func (s *S3) Delete(ctx context.Context, key string) error {
	if err := SafeKey(key); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, defaultOpTimeout)
	defer cancel()
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
	})
	if err != nil && isNotFound(err) {
		return nil
	}
	return err
}

// URL — kho private luôn "". Có CDN ⇒ CDN/<folder>/<key>; không ⇒ qua endpoint (path-style) hoặc
// domain AWS chuẩn khi không khai endpoint.
func (s *S3) URL(key string) string {
	if !s.public {
		return ""
	}
	k := s.objectKey(key)
	if s.cdn != "" {
		return s.cdn + "/" + k
	}
	if s.endpoint != "" {
		return s.endpoint + "/" + s.bucket + "/" + k
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, k)
}

func isNotFound(err error) bool {
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	var re interface{ HTTPStatusCode() int }
	return errors.As(err, &re) && re.HTTPStatusCode() == http.StatusNotFound
}
