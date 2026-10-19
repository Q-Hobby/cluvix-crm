package service

import (
	"fmt"

	"gorm.io/gorm"

	customerService "erpCluvix/internal/modules/webapp/crm/customer/service"
	crmPaymentDao "erpCluvix/internal/modules/webapp/crm/payment/dao"
	history "erpCluvix/internal/modules/webapp/history/log/service"
)

// DeleteCustomerWithPayments xoá khách hàng (hard-delete cascade) KÈM mọi phiếu thu/chi của khách,
// trong 1 transaction. Đường xoá khách DUY NHẤT của handler (web, mobile delegate, /emr/patient).
//
// Phiếu đi qua deletePayment từng cái — KHÔNG xoá thẳng bằng SQL: deletePayment dọn hoa hồng/doanh
// thu, gỡ deposit_id. (story-09: bỏ bút toán đảo sổ kép, guard kỳ kế toán đã khoá và guard/dọn hoá đơn
// điện tử cùng module kế toán/einvoice.) Lỗi bất kỳ phiếu nào ⇒ huỷ CẢ lần xoá khách.
//
// Vì sao nằm ở crm/service chứ không trong CustomerService.DeleteWithCleanup: crm/service đã import
// customer/service (gọi DeleteWithCleanup) nên chiều ngược lại là vòng import. FK payment.customer_id (RESTRICT)
// là lưới cuối: gọi thẳng DeleteWithCleanup mà khách còn phiếu thì lỗi 1451, không xoá âm thầm.
func DeleteCustomerWithPayments(db *gorm.DB, customerID uint, userID int) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// R-9: khoá customer là câu ĐẦU TIÊN (thứ tự khoá chung — payment_lock.go): phiếu mới của khách
		// phải xếp hàng sau lần xoá này, và danh sách phiếu đọc ngay sau đó thấy mọi phiếu đã commit
		// trước khi có khoá (ảnh chụp REPEATABLE READ chốt ở lần đọc thường đầu tiên).
		if err := lockCustomers(tx, int(customerID)); err != nil {
			return err
		}
		if err := deleteCustomerPayments(tx, customerID, userID); err != nil {
			return err
		}
		return customerService.NewCustomerService(tx).DeleteWithCleanupTx(tx, customerID, userID)
	})
}

// deleteCustomerPayments xoá 3 nhóm phiếu của khách: gắn thẳng customer_id (đặt cọc/hoàn cọc…), theo
// dòng dịch vụ (serviceUsed_id), theo đơn (parent_id). Mới → cũ: phiếu hoàn/chuyển cọc (tạo sau) được
// gỡ trước phiếu cọc gốc, đúng chiều hoàn tác.
//
// Log: xoá khách ⇒ chỉ giữ 1 dòng log xoá khách (user chốt 2026-09-22) — dọn log cũ của các phiếu trước
// khi xoá, và dọn luôn dòng log xoá phiếu mà deletePayment vừa ghi (khớp theo snapshot id).
func deleteCustomerPayments(tx *gorm.DB, customerID uint, userID int) error {
	var payments []crmPaymentDao.Payment
	if err := tx.Where(`customer_id = ?
		OR serviceUsed_id IN (SELECT id FROM order_service_used WHERE customer_id = ?)
		OR parent_id IN (SELECT id FROM order_plan WHERE customer_id = ?)`, customerID, customerID, customerID).
		Order("id DESC").Find(&payments).Error; err != nil {
		return fmt.Errorf("list customer payments: %w", err)
	}
	if len(payments) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(payments))
	for _, p := range payments {
		ids = append(ids, p.ID)
	}
	if err := tx.Exec("DELETE FROM log_payment WHERE payment_id IN ?", ids).Error; err != nil {
		return fmt.Errorf("clean log_payment: %w", err)
	}

	svc := &PaymentService{DB: tx}
	// Phần tính lại gom lại chạy 1 lần sau cùng — xem deferredPaymentRecalc (giữ/gom/bỏ từng tác dụng).
	deferred := newDeferredPaymentRecalc()
	deleted := make(map[uint]bool, len(payments))
	for i := range payments {
		p := &payments[i]
		// Phiếu mang customer_id của khách KHÁC (dữ liệu lệch: dòng dịch vụ/đơn của khách này nhưng phiếu
		// ghi khách khác) ⇒ khách kia KHÔNG bị xoá, phần tính lại của họ phải chạy đủ ⇒ xoá kiểu thường.
		d := deferred
		if p.CustomerId != nil && uint(*p.CustomerId) != customerID {
			d = nil
		}
		if err := svc.deletePaymentWith(p, userID, d); err != nil {
			return fmt.Errorf("delete payment %d: %w", p.ID, err)
		}
		deleted[p.ID] = true
	}
	deferred.run(tx, svc, deleted)

	if err := tx.Exec("DELETE FROM log_payment WHERE payment_id IS NULL AND action_type = ? AND "+
		"JSON_VALUE(changed_fields, '$.id.from' RETURNING UNSIGNED) IN ?", history.ActionDelete, ids).Error; err != nil {
		return fmt.Errorf("clean payment delete logs: %w", err)
	}
	return nil
}
