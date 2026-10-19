package define

// EncMarkerV1 — prefix đánh dấu giá trị đã mã hóa AES-256-GCM trong DB (cột *_enc).
// Giá trị có prefix này = đã encrypt; không có = plaintext legacy (đường đọc enc-aware xử lý cả 2).
// Dùng bởi pkg/crypto/token.go. (crm-standalone story-06: dời từ omni_channel.go — file đó xoá cùng
// omnichat/zalo; giá trị giữ nguyên nên dữ liệu đã mã hoá vẫn đọc được.)
const EncMarkerV1 = "enc:v1:"
