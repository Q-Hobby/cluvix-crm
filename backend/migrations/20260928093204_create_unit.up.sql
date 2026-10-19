-- Tạo bảng `unit` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `unit` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL COMMENT 'Tên của đơn vị tính',
  `type` int DEFAULT NULL COMMENT 'Loại đơn vị',
  `description` text COMMENT 'Mô tả đơn vị tính đó',
  `status` tinyint(1) DEFAULT '1' COMMENT 'Trạng thái còn hoạt động, mặc định 1 - hoạt động',
  `SortOrder` int DEFAULT NULL COMMENT 'Vị trí hiển thị',
  `categoryId` json DEFAULT NULL COMMENT 'Danh sách danh mục',
  `created_at` bigint DEFAULT NULL COMMENT 'Thời gian tạo mới',
  `updated_at` bigint DEFAULT NULL COMMENT 'Thời gian cập nhật gần đây nhất',
  `created_by` int DEFAULT NULL COMMENT 'Được tạo bởi ai',
  `updated_by` int DEFAULT NULL COMMENT 'Người cập nhật gần đây nhất',
  PRIMARY KEY (`id`),
  KEY `fk-unit-created_by-user-id` (`created_by`),
  KEY `fk-unit-updated_by-user-id` (`updated_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
