-- Seed hồ sơ doanh nghiệp mặc định (D22): bảng 1 dòng, thay cho bảng thương hiệu cũ.
--
-- Nội dung TRUNG TÍNH (placeholder), người vận hành sửa lại trong màn Cấu hình sau khi cài đặt.
-- Không 2FA bắt buộc, không giới hạn IP (allowed_ips NULL), tin thiết bị 30 ngày.
--
-- Idempotent: bảng chỉ có đúng 1 dòng, KHÔNG có khoá tự nhiên nào để bám ⇒ khoá seed là "bảng
-- đã có dòng hay chưa" (`WHERE NOT EXISTS (SELECT 1 FROM business_profile)`). Người dùng đã sửa
-- tên/mã thì chạy lại migration cũng không đè, không thêm dòng thứ hai.
INSERT INTO `business_profile`
    (`name`, `code`, `timezone`, `currency`, `currency_symbol`, `currency_locale`,
     `allowed_ips`, `require_2fa`, `require_2fa_roles`, `totp_trust_days`,
     `created_at`, `updated_at`, `created_by`, `updated_by`)
SELECT 'Doanh nghiệp của tôi', 'DN', 'Asia/Ho_Chi_Minh', 'VND', '₫', 'vi-VN',
       NULL, 0, NULL, 30,
       UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), NULL, NULL
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `business_profile`);
