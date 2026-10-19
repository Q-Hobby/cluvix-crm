-- Thêm FK fk-appointment-order_service_used_id (appointment.order_service_used_id → order_service_used.id) tách khỏi create_appointment vì vòng FK / bảng cha tạo sau. Idempotent.
SET @fk_exists := (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS
                   WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'appointment' AND CONSTRAINT_NAME = 'fk-appointment-order_service_used_id');
SET @sql := IF(@fk_exists = 0,
  'ALTER TABLE `appointment` ADD CONSTRAINT `fk-appointment-order_service_used_id` FOREIGN KEY (`order_service_used_id`) REFERENCES `order_service_used` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
