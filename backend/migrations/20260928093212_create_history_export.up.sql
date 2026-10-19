-- Tạo bảng `history_export` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `history_export` (
  `id` int NOT NULL AUTO_INCREMENT,
  `positionExport` varchar(500) DEFAULT NULL COMMENT 'Vị trí export',
  `quantityRecord` int DEFAULT NULL COMMENT 'Số lượng dữ liệu',
  `ip_address` varchar(25) DEFAULT NULL COMMENT 'Địa chỉ IP export',
  `userAgent` json DEFAULT NULL COMMENT 'Thông tin máy user export',
  `createdBy` int DEFAULT NULL COMMENT 'Người export',
  `createdAt` int DEFAULT NULL COMMENT 'Thời gian export',
  PRIMARY KEY (`id`),
  KEY `fk-history_export-createdBy` (`createdBy`),
  CONSTRAINT `fk-history_export-createdBy` FOREIGN KEY (`createdBy`) REFERENCES `user` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
