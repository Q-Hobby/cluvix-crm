// Package service — ChangeServiceForOSU: đổi service_id của 1 order_service_used (OSU)
// đã có payment, CÓ KIỂM SOÁT (order-service-change story-03).
//
// Khác với handleServices() (selective-merge cho update field thông thường của cả order):
// đây là 1 hành động nghiệp vụ riêng biệt, luôn chạy trong CHÍNH transaction của nó,
// KHÔNG best-effort — lỗi ở bất kỳ bước nào đều rollback toàn bộ.
package service

import (
	"gorm.io/gorm"

	orderPlanDao "erpCluvix/internal/modules/webapp/crm/order_plan/dao"
	crmServiceDao "erpCluvix/internal/modules/webapp/crm/service/dao"
	history "erpCluvix/internal/modules/webapp/history/log/service"
	"erpCluvix/pkg/errs"
	"erpCluvix/pkg/optlock"
)

type serviceChangeInfo struct {
	ID                int  `gorm:"column:id"`
	ServiceCategoryId *int `gorm:"column:serviceCategory_id"`
	TimeReminder      *int `gorm:"column:time_reminder"`
}

// ChangeServiceForOSU đổi service_id của OSU (scopedDB đã áp tenant scope từ handler, dùng
// cho load + ownership check). Điều kiện (story-03/PRD FR1; luật "cùng khoa" bỏ ở story-12
// crm-standalone — D16 bỏ khái niệm khoa; luật "chưa bắt đầu điều trị" bỏ ở story-14 cùng liệu trình — D1;
// luật "dịch vụ mới cùng company với OSU" bỏ ở story-37 — D15 1 thương hiệu/DB):
//   - Dịch vụ mới phải tồn tại
//
// Side-effect trong CÙNG 1 transaction (KHÔNG best-effort):
//  1. Snapshot service_id CŨ vào payment/order_commission_telesale/order_revenue_telesale liên quan
//     (chỉ set nếu NULL — idempotent, không ghi đè snapshot đã có).
//  2. Refresh CommissionTelesaleSnap theo dịch vụ MỚI
//     (dùng lại snapshotCommissionRates() đã có, vá luôn gap "chỉ set lúc create").
//  3. Recalc DayReExploitation theo dịch vụ MỚI.
//  4. (crm-standalone story-09: đã bỏ bước resync kho vật tư cùng module inventory.)
//  5. Audit log qua infra log_order_service_used sẵn có (history.SnapshotRow/LogChanges).
func (s *OrderPlanService) ChangeServiceForOSU(
	scopedDB *gorm.DB, osuID uint, newServiceID int, reason string, expectedUpdatedAt *int64, userID uint,
) (*crmServiceDao.OrderServiceUsed, error) {
	var svc crmServiceDao.OrderServiceUsed
	if err := scopedDB.First(&svc, osuID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if expectedUpdatedAt == nil {
		return nil, errs.NewLocalizedError("version_missing")
	}
	if *expectedUpdatedAt != svc.UpdatedAt { // khoá lạc quan theo updated_at (D6)
		return nil, errs.ErrVersionConflict
	}
	if newServiceID == svc.ServiceId {
		// No-op: không tạo audit log/snapshot rác.
		return &svc, nil
	}

	// 1 query lấy đủ thông tin CẢ dịch vụ cũ lẫn mới (category, time_reminder)
	// — tránh 3 round-trip riêng lẻ tới cùng bảng `service` như bản trước.
	var infoRows []serviceChangeInfo
	s.DB.Table("service").
		Select("id, serviceCategory_id, time_reminder").
		Where("id IN ?", []int{svc.ServiceId, newServiceID}).
		Find(&infoRows)
	var oldInfo, newInfo *serviceChangeInfo
	for i := range infoRows {
		switch infoRows[i].ID {
		case svc.ServiceId:
			oldInfo = &infoRows[i]
		case newServiceID:
			newInfo = &infoRows[i]
		}
	}
	if oldInfo == nil || newInfo == nil {
		return nil, errs.ErrNotFound
	}

	oldServiceID := svc.ServiceId

	var order orderPlanDao.OrderPlan
	if err := s.DB.First(&order, svc.OrderPlanId).Error; err != nil {
		return nil, errs.ErrNotFound
	}

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		oldSnapshot := history.SnapshotRow(tx, "order_service_used", svc.ID, history.OrderServiceUsedIncludeFields)

		// 1. Snapshot service_id CŨ — chỉ set nếu chưa có (idempotent, không ghi đè lần đổi trước).
		snapshotTargets := []string{"payment", "order_commission_telesale", "order_revenue_telesale"}
		fkColumn := map[string]string{"payment": "serviceUsed_id"}
		for _, table := range snapshotTargets {
			col := fkColumn[table]
			if col == "" {
				col = "order_service_used_id"
			}
			if err := tx.Table(table).
				Where(col+" = ? AND service_id_snapshot IS NULL", svc.ID).
				Update("service_id_snapshot", oldServiceID).Error; err != nil {
				return err
			}
		}

		// 2. Đổi service_id + refresh commission rate snapshot theo dịch vụ MỚI.
		svc.ServiceId = newServiceID
		snapshotCommissionRates(tx, &order, &svc)

		// 3. Recalc dayReExploitation theo dịch vụ MỚI (giống logic lúc create, order_plan.go) —
		// dùng lại newInfo đã load ở trên, KHÔNG query lại `service`.
		if newInfo.ServiceCategoryId != nil && *newInfo.ServiceCategoryId == 2 &&
			newInfo.TimeReminder != nil && *newInfo.TimeReminder > 0 && svc.CreatedDay != nil {
			svc.DayReExploitation = new(*svc.CreatedDay + int64(*newInfo.TimeReminder)*86400)
		} else {
			svc.DayReExploitation = nil
		}

		// Ghi bằng conditional update (WHERE id AND updated_at, pkg/optlock) thay vì tx.Save(&svc) full-row —
		// Save() ghi đè TOÀN BỘ field bằng giá trị trong struct đã load từ TRƯỚC lúc transaction
		// bắt đầu, không re-check khoá tại thời điểm ghi thật (TOCTOU: request khác có thể đã
		// đổi field khác của cùng OSU trong lúc này). Updates(map) + WHERE updated_at=? + RowsAffected
		// check biến bước kiểm khoá thành authoritative tại đúng thời điểm ghi.
		newUpdatedAt, err := optlock.Updates(tx.Model(&crmServiceDao.OrderServiceUsed{}).Where("id = ?", svc.ID),
			svc.UpdatedAt, map[string]interface{}{
				"service_id":                    svc.ServiceId,
				"dayReExploitation":             svc.DayReExploitation,
				"commission_telesale_snap":      svc.CommissionTelesaleSnap,
				"commission_telesale_type_snap": svc.CommissionTelesaleTypeSnap,
			})
		if err != nil {
			return err
		}
		svc.UpdatedAt = newUpdatedAt

		// 5. Audit log — tái dùng NGUYÊN VẸN infra log_order_service_used đã có (KHÔNG cần bảng mới).
		newSnapshot := history.SnapshotRow(tx, "order_service_used", svc.ID, history.OrderServiceUsedIncludeFields)
		var extra map[string]interface{}
		if reason != "" {
			extra = map[string]interface{}{"reason": reason}
		}
		_ = history.LogChanges(tx, "log_order_service_used", "order_service_used_id", int(svc.ID),
			oldSnapshot, newSnapshot,
			history.OrderServiceUsedIncludeFields, history.OrderServiceUsedSkipNullFields, int(userID), extra)

		return nil
	})
	if err != nil {
		return nil, err
	}
	return &svc, nil
}
