//go:build integration

package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/errs"
)

// requireSchemaAll skips the test when the suite was booted with the default
// testhelpers.SchemaBaseline (chỉ baseline.up.sql — xem main_integration_test.go). Các test
// trong file này cần cột từ migration MỚI (service_id_snapshot...) nên chỉ chạy thật khi ai đó
// chủ động boot suite với testhelpers.SchemaAll (đổi tạm TestMain hoặc set qua biến môi trường
// riêng nếu về sau có) — theo đúng precedent `tableExists` skip-pattern đã có trong payment_integration_test.go.
func requireSchemaAll(t *testing.T) {
	t.Helper()
	if !tableExists(testDB, "payment") {
		t.Skip("payment table not found — schema not applied")
	}
	var n int64
	testDB.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'payment' AND column_name = 'service_id_snapshot'").Scan(&n)
	if n == 0 {
		t.Skip("service_id_snapshot column not present — suite booted with SchemaBaseline (default); rerun with testhelpers.SchemaAll in TestMain for full order-service-change coverage")
	}
}

// createTestService inserts a minimal service row.
func createTestService(t *testing.T, tenant *testhelpers.TestTenant) uint {
	t.Helper()
	now := time.Now().Unix()
	row := testhelpers.WithTenantCols(tenant, map[string]any{
		"name":               fmt.Sprintf("Service %d", time.Now().UnixNano()),
		"code":               fmt.Sprintf("SV%d", time.Now().UnixNano()),
		"serviceCategory_id": 4, // Fee — trigger AutoDeductMaterialsForPayment path khi payment_status=Done
		"status":             1,
		"created_at":         now,
		"updated_at":         now,
	})
	delete(row, "branch_id") // service KHÔNG có cột branch_id
	require.NoError(t, testDB.Table("service").Create(&row).Error)
	return testhelpers.LastInsertID(testDB)
}

// createTestOrderPlan inserts a minimal order_plan row.
func createTestOrderPlan(t *testing.T, tenant *testhelpers.TestTenant) uint {
	t.Helper()
	now := time.Now().Unix()
	row := testhelpers.WithTenantCols(tenant, map[string]any{
		"code":         fmt.Sprintf("DH%d", time.Now().UnixNano()),
		"customer_id":  tenant.CustomerID,
		"created_time": now,
		"created_at":   now,
		"updated_at":   now,
	})
	require.NoError(t, testDB.Table("order_plan").Create(&row).Error)
	return testhelpers.LastInsertID(testDB)
}

// createTestOSU inserts a minimal order_service_used row with the given service and payment
// status — trọng tâm test resume ChangeServiceForOSU.
func createTestOSU(t *testing.T, tenant *testhelpers.TestTenant, planID, serviceID uint, paymentStatus int) uint {
	t.Helper()
	now := time.Now().Unix()
	row := testhelpers.WithTenantCols(tenant, map[string]any{
		"code":           fmt.Sprintf("SV%d", time.Now().UnixNano()),
		"customer_id":    tenant.CustomerID,
		"order_plan_id":  planID,
		"service_id":     serviceID,
		"payment":        decimal.NewFromInt(500000),
		"totalEnd":       decimal.NewFromInt(1000000),
		"payment_status": paymentStatus,
		"created_day":    now,
		"created_at":     now,
		"updated_at":     now,
	})
	require.NoError(t, testDB.Table("order_service_used").Create(&row).Error)
	return testhelpers.LastInsertID(testDB)
}

func TestChangeServiceForOSU_VersionMissing(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	requireSchemaAll(t)
	tenant := testhelpers.FreshTenant(t, testDB)
	svcA := createTestService(t, tenant)
	planID := createTestOrderPlan(t, tenant)
	osuID := createTestOSU(t, tenant, planID, svcA, define.ServicePaymentStatusStart)

	svc := NewOrderPlanService(testDB)
	_, err := svc.ChangeServiceForOSU(testDB, osuID, int(svcA)+1, "", nil, 1)
	assert.Error(t, err)
	if le, ok := err.(*errs.LocalizedError); ok {
		assert.Equal(t, "version_missing", le.Key)
	} else {
		t.Fatalf("expected LocalizedError version_missing, got %v (%T)", err, err)
	}
}

// TestChangeServiceForOSU_NewServiceNotFound — dịch vụ mới không tồn tại → ErrNotFound (luật "cùng
// khoa" đã bỏ ở story-12 crm-standalone, D16; trước đây ca này rơi vào service_change_department_mismatch).
func TestChangeServiceForOSU_NewServiceNotFound(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	requireSchemaAll(t)
	tenant := testhelpers.FreshTenant(t, testDB)
	svcA := createTestService(t, tenant)
	planID := createTestOrderPlan(t, tenant)
	osuID := createTestOSU(t, tenant, planID, svcA, define.ServicePaymentStatusStart)

	svc := NewOrderPlanService(testDB)
	_, err := svc.ChangeServiceForOSU(testDB, osuID, 999999999, "", osuUpdatedAt(t, osuID), 1)
	require.ErrorIs(t, err, errs.ErrNotFound)
}

// TestChangeServiceForOSU_Success — golden path: chưa điều trị, updated_at đúng.
// Xác nhận snapshot service_id CŨ được ghi vào payment đã có, service_id đổi đúng, updated_at bump.
func TestChangeServiceForOSU_Success(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	requireSchemaAll(t)
	tenant := testhelpers.FreshTenant(t, testDB)
	svcA := createTestService(t, tenant)
	svcB := createTestService(t, tenant)
	planID := createTestOrderPlan(t, tenant)
	osuID := createTestOSU(t, tenant, planID, svcA, define.ServicePaymentStatusStart)

	// Payment đã phát sinh TRƯỚC khi đổi — phải được snapshot service_id CŨ (svcA).
	now := time.Now().Unix()
	paymentRow := testhelpers.WithTenantCols(tenant, map[string]any{
		"code":           fmt.Sprintf("PAY%d", time.Now().UnixNano()),
		"customer_id":    tenant.CustomerID,
		"serviceUsed_id": osuID,
		"amount":         decimal.NewFromInt(500000),
		"method":         1,
		"payment_for":    define.PaymentForOrder,
		"payment_time":   now,
		"created_at":     now,
		"updated_at":     now,
	})
	require.NoError(t, testDB.Table("payment").Create(&paymentRow).Error)
	payID := testhelpers.LastInsertID(testDB)

	svc := NewOrderPlanService(testDB)
	beforeTS := osuUpdatedAt(t, osuID)
	updated, err := svc.ChangeServiceForOSU(testDB, osuID, int(svcB), "chọn nhầm dịch vụ", beforeTS, 1)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, int(svcB), updated.ServiceId)
	// Khoá lạc quan D6: updated_at tăng NGHIÊM NGẶT (kể cả cùng giây), giá trị trả về = giá trị DB.
	assert.Greater(t, updated.UpdatedAt, *beforeTS)
	assert.Equal(t, *osuUpdatedAt(t, osuID), updated.UpdatedAt)
	// Gửi lại updated_at cũ (cùng giây) ⇒ xung đột.
	_, err = svc.ChangeServiceForOSU(testDB, osuID, int(svcA), "", beforeTS, 1)
	require.ErrorIs(t, err, errs.ErrVersionConflict)

	var snapshot struct {
		ServiceIdSnapshot *int `gorm:"column:service_id_snapshot"`
	}
	require.NoError(t, testDB.Table("payment").Select("service_id_snapshot").Where("id = ?", payID).Take(&snapshot).Error)
	require.NotNil(t, snapshot.ServiceIdSnapshot)
	assert.Equal(t, int(svcA), *snapshot.ServiceIdSnapshot)

	var logCount int64
	testDB.Table("log_order_service_used").Where("order_service_used_id = ?", osuID).Count(&logCount)
	assert.GreaterOrEqual(t, logCount, int64(1), "expect audit log row for service_id change")
}

// TestChangeServiceForOSU_NoOp — đổi sang chính dịch vụ hiện tại → no-op, không tạo audit log.
func TestChangeServiceForOSU_NoOp(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	requireSchemaAll(t)
	tenant := testhelpers.FreshTenant(t, testDB)
	svcA := createTestService(t, tenant)
	planID := createTestOrderPlan(t, tenant)
	osuID := createTestOSU(t, tenant, planID, svcA, define.ServicePaymentStatusStart)

	svc := NewOrderPlanService(testDB)
	var before int64
	testDB.Table("log_order_service_used").Where("order_service_used_id = ?", osuID).Count(&before)

	updated, err := svc.ChangeServiceForOSU(testDB, osuID, int(svcA), "", osuUpdatedAt(t, osuID), 1)
	require.NoError(t, err)
	assert.Equal(t, int(svcA), updated.ServiceId)

	var after int64
	testDB.Table("log_order_service_used").Where("order_service_used_id = ?", osuID).Count(&after)
	assert.Equal(t, before, after, "no-op không được tạo audit log mới")
}

// osuUpdatedAt — updated_at hiện tại của OSU (khoá lạc quan D6), dạng con trỏ như body request.
func osuUpdatedAt(t *testing.T, osuID uint) *int64 {
	t.Helper()
	var ts int64
	require.NoError(t, testDB.Raw("SELECT updated_at FROM order_service_used WHERE id = ?", osuID).Scan(&ts).Error)
	return &ts
}
