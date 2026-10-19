-- Tạo bảng `config_code_monthly` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `config_code_monthly` (
  `id` int NOT NULL AUTO_INCREMENT,
  `config_code_id` int NOT NULL,
  `year` smallint NOT NULL,
  `month` tinyint NOT NULL,
  `last_number` int NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx-unique-config_code_monthly` (`config_code_id`,`year`,`month`),
  CONSTRAINT `fk-config_code_monthly-config_code_id` FOREIGN KEY (`config_code_id`) REFERENCES `config_code` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
