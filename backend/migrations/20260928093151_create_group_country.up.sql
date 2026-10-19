-- Tạo bảng `group_country` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `group_country` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL COMMENT 'Tên khu vực quốc tế',
  `desc` text COMMENT 'Nội dung chi tiết',
  `SortOrder` int DEFAULT NULL COMMENT 'Vị trí',
  `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT 'Trạng thái',
  `locationCountryId` json DEFAULT NULL COMMENT 'Danh sách quốc gia',
  `created_at` bigint DEFAULT NULL COMMENT 'Thời gian tạo mới',
  `updated_at` bigint DEFAULT NULL COMMENT 'Thời gian cập nhật gần đây nhất',
  `created_by` int DEFAULT NULL COMMENT 'Được tạo bởi ai',
  `updated_by` int DEFAULT NULL COMMENT 'Người cập nhật gần đây nhất',
  PRIMARY KEY (`id`),
  KEY `fk-group_country-created_by-user-id` (`created_by`),
  KEY `fk-group_country-updated_by-user-id` (`updated_by`),
  UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
