-- Tạo bảng `job` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `job` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL COMMENT 'Tên nghề nghiệp',
  `desc` text COMMENT 'Mô tả nghề nghiệp',
  `status` tinyint(1) DEFAULT '1' COMMENT 'Trạng thái (1: Hoạt động, 0: Không hoạt động)',
  `SortOrder` int DEFAULT NULL COMMENT 'Vị trí',
  `created_at` bigint DEFAULT NULL COMMENT 'Ngày tạo',
  `updated_at` bigint DEFAULT NULL COMMENT 'Ngày cập nhật',
  PRIMARY KEY (`id`),
  KEY `idx-job-name` (`name`),
  KEY `idx-job-status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
