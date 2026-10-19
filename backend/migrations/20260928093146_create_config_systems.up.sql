-- Tạo bảng `config_systems` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `config_systems` (
  `id` int NOT NULL AUTO_INCREMENT,
  `content` json DEFAULT NULL COMMENT 'Giá trị',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
