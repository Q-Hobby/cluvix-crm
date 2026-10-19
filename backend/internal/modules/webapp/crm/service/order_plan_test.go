package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	otpRepo "erpCluvix/internal/modules/webapp/crm/order_plan/repo"
	"erpCluvix/pkg/errs"
)

// --- Phase 4 expand: pure unit tests of order_plan Delete pre-flight ---

type fakeOTPReader struct {
	custByID    map[int]otpRepo.CustomerInfo
	custErr     map[int]error
	branchByID  map[int]*int
	svcsByOrder map[uint][]uint
	paymentSum  int64
	queryErr    error
}

func newFakeOTPReader() *fakeOTPReader {
	return &fakeOTPReader{
		custByID:    map[int]otpRepo.CustomerInfo{},
		custErr:     map[int]error{},
		branchByID:  map[int]*int{},
		svcsByOrder: map[uint][]uint{},
	}
}

func (f *fakeOTPReader) CustomerInfo(_ *gorm.DB, id int) (otpRepo.CustomerInfo, error) {
	if e, ok := f.custErr[id]; ok {
		return otpRepo.CustomerInfo{}, e
	}
	return f.custByID[id], nil
}

func (f *fakeOTPReader) CustomerBranchID(_ *gorm.DB, id int) (*int, error) {
	return f.branchByID[id], nil
}

func (f *fakeOTPReader) OrderServiceUsedIDs(_ *gorm.DB, orderID uint) ([]uint, error) {
	return f.svcsByOrder[orderID], f.queryErr
}

func (f *fakeOTPReader) CountPaymentsForServices(_ *gorm.DB, _ []uint) (int64, error) {
	return f.paymentSum, nil
}

var _ otpRepo.Reader = (*fakeOTPReader)(nil)

func TestOTP_CheckDelete_NoServices_OK(t *testing.T) {
	fake := newFakeOTPReader()
	// svcsByOrder[1] = nil → 0 services → early return OK.
	svc := NewOrderPlanServiceWithReader(nil, fake)

	ids, err := svc.checkDeleteConstraints(nil, 1)
	require.NoError(t, err)
	assert.Empty(t, ids)
}

func TestOTP_CheckDelete_HasPayment_Rejected(t *testing.T) {
	fake := newFakeOTPReader()
	fake.svcsByOrder[1] = []uint{10}
	fake.paymentSum = 1

	svc := NewOrderPlanServiceWithReader(nil, fake)
	_, err := svc.checkDeleteConstraints(nil, 1)
	require.ErrorIs(t, err, errs.ErrServiceHasPayment)
}

func TestOTP_CheckDelete_AllClean_ReturnsIDs(t *testing.T) {
	fake := newFakeOTPReader()
	fake.svcsByOrder[1] = []uint{10, 11, 12}
	// all counts default 0

	svc := NewOrderPlanServiceWithReader(nil, fake)
	ids, err := svc.checkDeleteConstraints(nil, 1)
	require.NoError(t, err)
	assert.Equal(t, []uint{10, 11, 12}, ids)
}

func TestOTP_CheckDelete_PropagatesReadError(t *testing.T) {
	fake := newFakeOTPReader()
	fake.queryErr = errors.New("db connection lost")

	svc := NewOrderPlanServiceWithReader(nil, fake)
	_, err := svc.checkDeleteConstraints(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db connection lost")
}
