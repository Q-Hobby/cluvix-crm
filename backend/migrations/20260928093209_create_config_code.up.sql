-- Tạo bảng `config_code` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `config_code` (
  `id` int NOT NULL AUTO_INCREMENT,
  `branch_id` int NOT NULL DEFAULT '1' COMMENT 'Chi nhánh',
  `name` varchar(255) DEFAULT NULL COMMENT 'Tên bảng',
  `parent_table` varchar(100) DEFAULT NULL COMMENT 'Tên bảng',
  `infix_code` varchar(50) DEFAULT NULL COMMENT 'Trung tố của mã code',
  `created_at` bigint DEFAULT NULL COMMENT 'Thời gian tạo mới',
  `updated_at` bigint DEFAULT NULL COMMENT 'Thời gian cập nhật gần đây nhất',
  `created_by` int DEFAULT NULL COMMENT 'Được tạo bởi ai',
  `updated_by` int DEFAULT NULL COMMENT 'Người cập nhật gần đây nhất',
  `include_branch_code` tinyint(1) NOT NULL DEFAULT '1' COMMENT 'Có ghép mã chi nhánh vào mã sinh tự động không',
  `monthly_reset` tinyint(1) NOT NULL DEFAULT '1' COMMENT 'Serial có reset theo tháng (kèm YYMM trong mã) không',
  `serial_length` tinyint NOT NULL DEFAULT '2' COMMENT 'Độ dài tối thiểu số serial, tự nới rộng khi tràn',
  PRIMARY KEY (`id`),
  KEY `fk-config_code-created_by` (`created_by`),
  KEY `fk-config_code-updated_by` (`updated_by`),
  UNIQUE KEY `uq_config_code_branch_table` (`branch_id`,`parent_table`),
  CONSTRAINT `fk-config_code-branch_id` FOREIGN KEY (`branch_id`) REFERENCES `branch` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
