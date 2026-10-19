-- Tạo bảng `reminder` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `reminder` (
  `id` int NOT NULL AUTO_INCREMENT,
  `code` varchar(25) NOT NULL COMMENT 'Mã lời nhắc',
  `customer_id` int DEFAULT NULL COMMENT 'Khách hàng',
  `status` tinyint(1) DEFAULT NULL COMMENT 'Tình trạng',
  `type` tinyint(1) DEFAULT NULL COMMENT 'Loại',
  `start_time` bigint DEFAULT NULL COMMENT 'Thời gian bắt đầu',
  `note` text NOT NULL COMMENT 'Nội dung lời nhắc',
  `relatedUser` json DEFAULT NULL COMMENT 'Người liên quan',
  `created_at` bigint DEFAULT NULL COMMENT 'Thời gian tạo mới',
  `updated_at` bigint NOT NULL DEFAULT '0',
  `created_by` int DEFAULT NULL COMMENT 'Được tạo bởi ai',
  `updated_by` int DEFAULT NULL COMMENT 'Người cập nhật gần đây nhất',
  PRIMARY KEY (`id`),
  UNIQUE KEY `code` (`code`),
  KEY `fk-reminder-created_by` (`created_by`),
  KEY `fk-reminder-customer_id` (`customer_id`),
  KEY `fk-reminder-updated_by` (`updated_by`),
  FULLTEXT KEY `ft_reminder_note_ngram` (`note`) /*!50100 WITH PARSER `ngram` */ ,
  KEY `idx_reminder_start_status` (`start_time`,`status`),
  CONSTRAINT `fk-reminder-customer_id` FOREIGN KEY (`customer_id`) REFERENCES `customer` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
