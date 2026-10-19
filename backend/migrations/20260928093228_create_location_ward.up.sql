-- Tạo bảng `location_ward` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `location_ward` (
  `id` int NOT NULL AUTO_INCREMENT,
  `codeWard` varchar(10) DEFAULT NULL,
  `name` varchar(255) NOT NULL,
  `nameWithType` varchar(255) DEFAULT NULL,
  `path` varchar(255) DEFAULT NULL,
  `pathWithType` varchar(255) DEFAULT NULL,
  `slug` varchar(255) DEFAULT NULL,
  `Type` int DEFAULT NULL,
  `province_id` int NOT NULL,
  `SortOrder` int DEFAULT NULL,
  `status` tinyint(1) DEFAULT '1',
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `created_by` int DEFAULT '1',
  `updated_by` int DEFAULT '1',
  PRIMARY KEY (`id`),
  KEY `fk-location_ward-province_id` (`province_id`),
  KEY `fk_location_ward_created_by_user` (`created_by`),
  KEY `fk_location_ward_updated_by_user` (`updated_by`),
  KEY `index-slug` (`slug`),
  CONSTRAINT `fk-location_ward-province_id` FOREIGN KEY (`province_id`) REFERENCES `location_province` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
