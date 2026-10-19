-- Tạo bảng `ethnicity` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `ethnicity` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(100) NOT NULL COMMENT 'Tên dân tộc',
  `codeEthnicity` varchar(20) DEFAULT NULL COMMENT 'Mã dân tộc',
  `SortOrder` int DEFAULT NULL COMMENT 'Vị trí',
  `status` tinyint(1) DEFAULT '1' COMMENT 'Trạng thái (1: Hoạt động, 0: Không hoạt động)',
  `created_at` bigint DEFAULT NULL COMMENT 'Ngày tạo',
  PRIMARY KEY (`id`),
  KEY `idx-ethnicity-code` (`codeEthnicity`),
  KEY `idx-ethnicity-status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
