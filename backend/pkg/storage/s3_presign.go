package storage

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var _ Signer = (*S3)(nil)

// SignedURL — presigned GET (SigV4, tính cục bộ, KHÔNG gọi mạng) kèm response-content-type +
// response-content-disposition: object storage tự trả đúng 2 header này, người cầm URL không sửa
// được (nằm trong phần đã ký). Hỗ trợ Range sẵn ⇒ <video> tua được.
//
// CORS: URL dùng trực tiếp trong <video src>/<audio src>/<a href> (request "no-cors" của trình
// duyệt, không phải XHR) ⇒ bucket KHÔNG cần cấu hình CORS cho luồng này.
func (s *S3) SignedURL(ctx context.Context, key string, ttl time.Duration, opts SignOptions) (string, error) {
	if err := SafeKey(key); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, defaultOpTimeout)
	defer cancel()

	input := &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(s.objectKey(key)),
		ResponseContentDisposition: aws.String(opts.ContentDisposition()),
	}
	if ct := strings.TrimSpace(opts.ContentType); ct != "" {
		input.ResponseContentType = aws.String(ct)
	}
	req, err := s3.NewPresignClient(s.client).PresignGetObject(ctx, input,
		s3.WithPresignExpires(ClampSignedURLTTL(ttl)))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}
