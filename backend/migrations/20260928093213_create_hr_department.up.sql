-- Tạo bảng `hr_department` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `hr_department` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL COMMENT 'Tên phòng ban',
  `desc` text COMMENT 'Nội dung chi tiết',
  `head_user_id` int DEFAULT NULL COMMENT 'Trưởng phòng ban (FK user)',
  `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT 'Trạng thái',
  `SortOrder` int DEFAULT NULL COMMENT 'Vị trí',
  `created_at` bigint DEFAULT NULL COMMENT 'Thời gian tạo mới',
  `updated_at` bigint DEFAULT NULL COMMENT 'Thời gian cập nhật gần đây nhất',
  `created_by` int DEFAULT NULL COMMENT 'Được tạo bởi ai',
  `updated_by` int DEFAULT NULL COMMENT 'Người cập nhật gần đây nhất',
  PRIMARY KEY (`id`),
  KEY `fk-hr_department-created_by-user-id` (`created_by`),
  KEY `fk-hr_department-updated_by-user-id` (`updated_by`),
  KEY `idx-hr_department-head_user_id` (`head_user_id`),
  UNIQUE KEY `name` (`name`),
  CONSTRAINT `fk-hr_department-head_user_id` FOREIGN KEY (`head_user_id`) REFERENCES `user` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
