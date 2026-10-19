-- Tạo bảng `note_template` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `note_template` (
  `id` int NOT NULL AUTO_INCREMENT,
  `type_id` smallint DEFAULT NULL COMMENT 'Loại mẫu template',
  `name` varchar(255) NOT NULL COMMENT 'Tên template',
  `content` text COMMENT 'Nội dung',
  `status` int DEFAULT '1' COMMENT 'Trạng thái hiển thị',
  `SortOrder` int DEFAULT NULL COMMENT 'Vị trí',
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `created_by` int DEFAULT NULL,
  `updated_by` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk-note_template-created_by` (`created_by`),
  KEY `fk-note_template-updated_by` (`updated_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
