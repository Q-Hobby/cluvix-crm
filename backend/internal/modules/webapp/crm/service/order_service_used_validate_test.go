package service

import (
	"testing"

	crmServiceDao "erpCluvix/internal/modules/webapp/crm/service/dao"

	"github.com/shopspring/decimal"
)

// baseValidOSU trả về 1 svc đủ field required để validateOrderServiceUsed không có lỗi.
// (crm-standalone story-14: test nhánh numTreatment theo serviceCategory_id đã bỏ cùng rule đó.)
func baseValidOSU() *crmServiceDao.OrderServiceUsed {
	price := decimal.NewFromInt(100)
	qty := 1
	return &crmServiceDao.OrderServiceUsed{
		CustomerId:  1,
		ServiceId:   1,
		UnitPrice:   &price,
		OrderPlanId: 1,
		Quantity:    &qty,
	}
}

func TestValidateOrderServiceUsed_Basic(t *testing.T) {
	if err := validateOrderServiceUsed(baseValidOSU()); err != nil {
		t.Fatalf("svc hợp lệ không được lỗi, có %v", err)
	}

	svc := baseValidOSU()
	pay, total := decimal.NewFromInt(200), decimal.NewFromInt(100)
	svc.Payment, svc.TotalEnd = &pay, &total
	if err := validateOrderServiceUsed(svc); err == nil {
		t.Fatal("payment > totalEnd phải bị chặn")
	}

	svc = baseValidOSU()
	zero := 0
	svc.Quantity = &zero
	if err := validateOrderServiceUsed(svc); err == nil {
		t.Fatal("quantity < 1 phải bị chặn")
	}
}
