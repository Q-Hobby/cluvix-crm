-- Tạo bảng `group_source` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `group_source` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL COMMENT 'Tên nhóm nguồn',
  `desc` text COMMENT 'Nội dung chi tiết',
  `SortOrder` int DEFAULT NULL COMMENT 'Vị trí',
  `online` tinyint(1) DEFAULT '0' COMMENT 'Xác định nhóm nguồn online',
  `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT 'Trạng thái',
  `sourceId` json DEFAULT NULL COMMENT 'Danh sách nguồn khách',
  `created_at` bigint DEFAULT NULL COMMENT 'Thời gian tạo mới',
  `updated_at` bigint DEFAULT NULL COMMENT 'Thời gian cập nhật gần đây nhất',
  `created_by` int DEFAULT NULL COMMENT 'Được tạo bởi ai',
  `updated_by` int DEFAULT NULL COMMENT 'Người cập nhật gần đây nhất',
  PRIMARY KEY (`id`),
  KEY `fk-group_source-created_by-user-id` (`created_by`),
  KEY `fk-group_source-updated_by-user-id` (`updated_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
