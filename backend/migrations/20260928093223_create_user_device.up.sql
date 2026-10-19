-- Tạo bảng `user_device` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `user_device` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `fingerprint` varchar(64) NOT NULL,
  `device_name` varchar(255) DEFAULT NULL,
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '1=trusted, 0=pending, -1=revoked',
  `last_ip` varchar(45) DEFAULT NULL,
  `last_seen` bigint DEFAULT NULL,
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `totp_trusted_until` bigint DEFAULT NULL COMMENT 'Skip 2FA tới mốc này',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_user_device_fp` (`user_id`,`fingerprint`),
  CONSTRAINT `fk-user_device-user_id` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
