//go:build integration

package service

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	crmPaymentDao "erpCluvix/internal/modules/webapp/crm/payment/dao"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/errs"
)

// tableExists kiểm tra bảng có trong schema không (dùng để skip test khi dùng SchemaBaseline)
func tableExists(db *gorm.DB, name string) bool {
	var n int64
	db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", name).Scan(&n)
	return n > 0
}

func TestBeforeSave_VersionConflict(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	payID, err := testhelpers.CreatePayment(
		testDB, tenant,
		decimal.NewFromInt(100),
		define.PaymentForDeposit,
	)
	require.NoError(t, err)

	var p crmPaymentDao.Payment
	require.NoError(t, testDB.First(&p, payID).Error)
	p.UpdatedAt-- // stale (khoá lạc quan updated_at, D6)

	svc := NewPaymentService(testDB)
	txErr := testDB.Transaction(func(tx *gorm.DB) error {
		return svc.beforeSave(tx, &p, false)
	})
	assert.ErrorIs(t, txErr, errs.ErrVersionConflict)
}

func TestBeforeSave_CorrectVersionAccepted(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	payID, err := testhelpers.CreatePayment(
		testDB, tenant,
		decimal.NewFromInt(200),
		define.PaymentForDeposit,
	)
	require.NoError(t, err)

	var p crmPaymentDao.Payment
	require.NoError(t, testDB.First(&p, payID).Error)

	svc := NewPaymentService(testDB)
	txErr := testDB.Transaction(func(tx *gorm.DB) error {
		return svc.beforeSave(tx, &p, false)
	})
	assert.NotErrorIs(t, txErr, errs.ErrVersionConflict)
}
