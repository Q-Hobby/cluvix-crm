-- Tạo bảng `note` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `note` (
  `id` int NOT NULL AUTO_INCREMENT,
  `note` text,
  `customer_id` int DEFAULT NULL COMMENT 'Khách hàng',
  `pinned` tinyint DEFAULT NULL COMMENT 'Ghi chú có đang được ghim',
  `pinned_at` bigint DEFAULT NULL COMMENT 'Thời điểm ghim',
  `pinned_by` int DEFAULT NULL COMMENT 'Ai ghim',
  `created_at` bigint DEFAULT NULL,
  `created_by` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_note-customer_id` (`customer_id`),
  KEY `fk_note-pinned_by` (`pinned_by`),
  KEY `fk_note_created_by` (`created_by`),
  KEY `idx_note_created_at` (`created_at`),
  CONSTRAINT `fk_note-customer_id` FOREIGN KEY (`customer_id`) REFERENCES `customer` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `fk_note-pinned_by` FOREIGN KEY (`pinned_by`) REFERENCES `user` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
