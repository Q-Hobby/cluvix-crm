-- Tạo bảng `user` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `user` (
  `id` int NOT NULL AUTO_INCREMENT,
  `id_old` int DEFAULT NULL,
  `username` varchar(255) NOT NULL,
  `password_hash` varchar(255) NOT NULL,
  `must_change_password` tinyint NOT NULL DEFAULT '0' COMMENT '1 = bắt buộc đổi mật khẩu trước khi dùng hệ thống',
  `branchId` json DEFAULT NULL COMMENT 'Danh sách chi nhánh',
  `status` smallint NOT NULL DEFAULT '10',
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint NOT NULL DEFAULT '0',
  `logged_at` int DEFAULT NULL,
  `created_by` smallint DEFAULT NULL,
  `updated_by` smallint DEFAULT NULL,
  `allowed_ips` json DEFAULT NULL COMMENT 'Override IP allowlist riêng cho user (rỗng = dùng cấu hình chung ở business_profile)',
  `totp_secret` varchar(255) DEFAULT NULL COMMENT 'TOTP secret mã hóa AES-GCM',
  `totp_enabled` tinyint NOT NULL DEFAULT '0' COMMENT '1=đã bật 2FA',
  `totp_enrolled_at` bigint DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `id_old` (`id_old`),
  UNIQUE KEY `username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
