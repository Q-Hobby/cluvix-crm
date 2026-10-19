-- Tạo bảng `security_audit_log` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `security_audit_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `event` varchar(64) NOT NULL,
  `user_id` int DEFAULT NULL,
  `ip` varchar(45) DEFAULT NULL,
  `request_id` varchar(32) DEFAULT NULL,
  `username` varchar(255) DEFAULT NULL,
  `detail` json DEFAULT NULL,
  `created_at` bigint NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_audit_created_at` (`created_at`),
  KEY `idx_audit_event` (`event`,`created_at`),
  KEY `idx_audit_user` (`user_id`,`created_at`),
  CONSTRAINT `fk-security_audit_log-user_id` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
