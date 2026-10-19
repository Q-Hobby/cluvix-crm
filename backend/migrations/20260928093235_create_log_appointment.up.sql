-- Tạo bảng `log_appointment` (keep-set CRM). Idempotent: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS `log_appointment` (
  `id` int NOT NULL AUTO_INCREMENT,
  `appointment_id` int DEFAULT NULL COMMENT 'Lịch hẹn',
  `action_type` tinyint DEFAULT '2' COMMENT 'Loại hành động: 1: create, 2: update, 3: delete',
  `changed_fields` json DEFAULT NULL COMMENT 'Field thay đổi',
  `changed_by` int DEFAULT NULL,
  `changed_at` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk-log_appointment-appointment_id` (`appointment_id`),
  KEY `fk-log_appointment-changed_by` (`changed_by`),
  CONSTRAINT `fk-log_appointment-appointment_id` FOREIGN KEY (`appointment_id`) REFERENCES `appointment` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `fk-log_appointment-changed_by` FOREIGN KEY (`changed_by`) REFERENCES `user` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
