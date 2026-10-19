-- Tạo bảng `log_user` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `log_user` (
  `id` int NOT NULL AUTO_INCREMENT,
  `user_id` int DEFAULT NULL COMMENT 'FK → user.id, NULL nếu user đã bị xóa cứng',
  `action_type` tinyint NOT NULL DEFAULT '2' COMMENT '1=create 2=update 3=delete',
  `changed_fields` json DEFAULT NULL COMMENT 'Diff {field: {from, to}}',
  `changed_by` int DEFAULT NULL COMMENT 'FK → user.id',
  `changed_at` int NOT NULL COMMENT 'Unix timestamp',
  PRIMARY KEY (`id`),
  KEY `fk-log_user-changed_by` (`changed_by`),
  KEY `idx_log_user_changed_at` (`changed_at`),
  KEY `idx_log_user_user_id` (`user_id`),
  CONSTRAINT `fk-log_user-changed_by` FOREIGN KEY (`changed_by`) REFERENCES `user` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `fk-log_user-user_id` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Lịch sử sửa/xóa user — user_id giữ NULL khi user gốc bị xóa cứng để giữ audit trail';
