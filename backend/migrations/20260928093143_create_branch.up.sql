-- Tạo bảng `branch` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `branch` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `code` varchar(25) DEFAULT NULL,
  `address` varchar(255) DEFAULT NULL,
  `hotline` varchar(25) DEFAULT NULL,
  `email` varchar(50) DEFAULT NULL,
  `statusOpenAppointment` tinyint(1) DEFAULT '1' COMMENT 'Trạng thái chi nhánh có nhận khách đặt hẹn hay không',
  `status` tinyint(1) DEFAULT '1',
  `SortOrder` int DEFAULT NULL,
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `created_by` int DEFAULT NULL,
  `updated_by` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk-branch_created_by` (`created_by`),
  KEY `fk-branch_updated_by` (`updated_by`),
  UNIQUE KEY `uq_branch_code` (`code`),
  UNIQUE KEY `uq_branch_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
