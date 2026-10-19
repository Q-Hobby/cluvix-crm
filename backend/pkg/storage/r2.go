package storage

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// r2Region — R2 chỉ nhận đúng "auto" (SDK ký SigV4 với "auto", endpoint tự định tuyến). Ép giá
// trị thay vì bắt khai: S3_REGION mặc định của dự án là "hcm", để nguyên là mọi request bị từ chối.
const r2Region = "auto"

// newR2 dựng một kho Cloudflare R2. R2 nói giao thức S3 nên KHÔNG có driver riêng — chỉ chuẩn hoá
// cấu hình rồi gọi NewS3 (nhân bản Save/Open/Delete là tự chuốc cảnh sửa bug một nơi quên nơi kia).
//
// Khác S3 thường (cả 3 đều hỏng ÂM THẦM nếu bỏ qua):
//  1. Region "auto".
//  2. KHÔNG gửi ACL: R2 không hỗ trợ ACL object, public/private là cấp BUCKET (bật Public Access
//     hoặc gắn custom domain) — vì vậy kho private BẮT BUỘC là bucket riêng (xem factory).
//  3. Checksum trailer — xem r2ChecksumOption.
func newR2(cfg S3Config) (*S3, error) {
	cfg.Region = r2Region
	cfg.ACL = ""
	return NewS3(cfg, r2ChecksumOption)
}

// r2ChecksumOption tắt checksum "khi được hỗ trợ" mặc định của aws-sdk-go-v2 (từ 2025): PutObject
// gửi body aws-chunked kèm trailer CRC32 — mở rộng riêng của AWS; endpoint tương thích S3 không
// cài trailer có thể trả 501/400 hoặc tệ hơn lưu luôn khung aws-chunked vào nội dung file.
// WhenRequired vẫn giữ checksum cho thao tác bắt buộc. Kênh đã là HTTPS nên không làm yếu gì.
func r2ChecksumOption(o *s3.Options) {
	o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
}
