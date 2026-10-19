-- Tạo bảng `customer_category` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `customer_category` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL COMMENT 'Tên nhóm khách hàng',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT 'Trạng thái',
  `SortOrder` int DEFAULT NULL COMMENT 'Vị trí hiển thị',
  `time_nhaclich` int DEFAULT NULL,
  `type_nhaclich` tinyint DEFAULT '1' COMMENT 'Loại thời gian nhắc lịch',
  `created_at` bigint DEFAULT NULL COMMENT 'Thời gian tạo mới',
  `updated_at` bigint DEFAULT NULL COMMENT 'Thời gian cập nhật gần đây nhất',
  `created_by` int DEFAULT NULL COMMENT 'Được tạo bởi ai',
  `updated_by` int DEFAULT NULL COMMENT 'Người cập nhật gần đây nhất',
  PRIMARY KEY (`id`),
  KEY `fk-customer_category-created_by-user-id` (`created_by`),
  KEY `fk-customer_category-updated_by-user-id` (`updated_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
