package dao

import (
	"time"

	"erpCluvix/pkg/define"

	"gorm.io/gorm"
)

type Dao struct{ DB *gorm.DB }

// Upsert ghi nhận thiết bị: đã có (user_id, fingerprint) → update last_seen/last_ip/
// device_name; chưa có → tạo mới status trusted (Phase 1 auto-trust).
// minTouchInterval > 0: bỏ qua write nếu last_seen còn mới hơn ngưỡng — giảm tải
// hot-path refresh (login truyền 0 để luôn ghi). Trả (device, isNew) — isNew=true
// để caller phát security event new_device.
func (d *Dao) Upsert(userID uint, fingerprint, deviceName, ip string, minTouchInterval time.Duration) (*UserDevice, bool, error) {
	now := time.Now().Unix()

	var existing UserDevice
	err := d.DB.Select("id, status, last_seen").
		Where("user_id = ? AND fingerprint = ?", userID, fingerprint).
		Take(&existing).Error
	if err == nil {
		// Throttle: session đang active refresh mỗi ~13 phút → không cần ghi DB mỗi lần.
		// Bỏ qua throttle khi device revoked (phải re-trust ngay lập tức).
		if minTouchInterval > 0 && existing.Status != define.UserDeviceRevoked &&
			existing.LastSeen != nil && now-*existing.LastSeen < int64(minTouchInterval.Seconds()) {
			return &existing, false, nil
		}
		updates := map[string]any{
			"last_ip":    ip,
			"last_seen":  now,
			"updated_at": now,
		}
		if deviceName != "" {
			updates["device_name"] = deviceName
		}
		// Thiết bị revoked login lại bằng đúng credentials → tin lại (revoke chỉ
		// kill session, không phải ban thiết bị; ban user = đổi user.status).
		if existing.Status == define.UserDeviceRevoked {
			updates["status"] = define.UserDeviceTrusted
		}
		if err := d.DB.Model(&UserDevice{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return nil, false, err
		}
		return &existing, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, err
	}

	dev := UserDevice{
		UserID:      userID,
		Fingerprint: fingerprint,
		Status:      define.UserDeviceTrusted,
		LastSeen:    &now,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
	if deviceName != "" {
		dev.DeviceName = &deviceName
	}
	if ip != "" {
		dev.LastIP = &ip
	}
	if err := d.DB.Create(&dev).Error; err != nil {
		return nil, false, err
	}
	return &dev, true, nil
}

// ListByUser trả thiết bị của user, mới hoạt động lên đầu.
func (d *Dao) ListByUser(userID uint) ([]UserDevice, error) {
	var items []UserDevice
	err := d.DB.
		Select("id, user_id, device_name, status, last_ip, last_seen, created_at, updated_at, fingerprint").
		Where("user_id = ? AND status <> ?", userID, define.UserDeviceRevoked).
		Order("last_seen DESC").
		Find(&items).Error
	return items, err
}

// GetOwned load thiết bị theo id VÀ user_id — chặn thao tác lên thiết bị của user khác.
func (d *Dao) GetOwned(id, userID uint) (*UserDevice, error) {
	var dev UserDevice
	err := d.DB.Select("id, user_id, fingerprint").
		Where("id = ? AND user_id = ?", id, userID).Take(&dev).Error
	if err != nil {
		return nil, err
	}
	return &dev, nil
}

// MarkRevoked set status revoked (record giữ lại làm lịch sử; login lại sẽ trust lại).
func (d *Dao) MarkRevoked(id uint) error {
	return d.DB.Model(&UserDevice{}).Where("id = ?", id).Updates(map[string]any{
		"status":     define.UserDeviceRevoked,
		"updated_at": time.Now().Unix(),
	}).Error
}

// DeleteStale xoá device không hoạt động từ trước mốc `before` (Unix) — chống bảng
// phình vô hạn khi user đổi browser/OS. Fallback created_at khi last_seen NULL.
// Trả số row đã xoá.
func (d *Dao) DeleteStale(before int64) (int64, error) {
	res := d.DB.Where("COALESCE(last_seen, created_at, 0) < ?", before).Delete(&UserDevice{})
	return res.RowsAffected, res.Error
}
