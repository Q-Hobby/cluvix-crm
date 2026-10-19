//go:build integration

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
)

func TestUpdateCustomer_InvalidStatusTransition(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	require.NoError(t, testhelpers.SetCustomerStatus(testDB, tenant.CustomerID, define.CustomerStatusCustomer))

	svc := NewCustomerService(testDB)
	_, err := svc.Update(
		int(tenant.CustomerID),
		map[string]any{"status": define.CustomerStatusLead}, // Customer→Lead: invalid
		custUpdatedAt(t, tenant.CustomerID),
		0,
	)
	assert.Error(t, err, "expected error for invalid status transition Customer→Lead")
}

func TestUpdateCustomer_ValidStatusTransition(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	require.NoError(t, testhelpers.SetCustomerStatus(testDB, tenant.CustomerID, define.CustomerStatusTicket))

	svc := NewCustomerService(testDB)
	_, err := svc.Update(
		int(tenant.CustomerID),
		map[string]any{"status": define.CustomerStatusContact},
		custUpdatedAt(t, tenant.CustomerID),
		0,
	)
	assert.NoError(t, err, "expected no error for valid status transition Ticket→Contact")
}

// TestUpdateCustomer_NoTransitionIntoLeadOrTicket — D14 (2026-09-23, QA story-33): mọi đích
// từng là Lead(4)/Ticket(2) trong CustomerStatusTransitions nay đổi thành Contact(16). Test
// khoá lại: (a) transition cũ vào Lead/Ticket giờ bị từ chối; (b) transition thay thế (đích
// Contact) hợp lệ và Contact luôn nằm trong CustomerStatuses (hiển thị CRM list).
func TestUpdateCustomer_NoTransitionIntoLeadOrTicket(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	inCustomerStatuses := func(status int) bool {
		for _, s := range define.CustomerStatuses {
			if s == status {
				return true
			}
		}
		return false
	}
	require.True(t, inCustomerStatuses(define.CustomerStatusContact),
		"Contact(16) phải nằm trong CustomerStatuses (hiện ở list)")

	cases := []struct {
		name string
		from int
	}{
		{"Trash→Contact", define.CustomerStatusTrash},
		{"Ticket→Contact", define.CustomerStatusTicket},
		{"AppointmentCancel→Contact", define.CustomerStatusAppointmentCancel},
		{"Lost→Contact", define.CustomerStatusLost},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			tenant := testhelpers.FreshTenant(t, testDB)
			require.NoError(t, testhelpers.SetCustomerStatus(testDB, tenant.CustomerID, tt.from))

			svc := NewCustomerService(testDB)

			// Đích cũ (Lead) từng hợp lệ từ các status này — nay PHẢI bị từ chối.
			_, errLead := svc.Update(
				int(tenant.CustomerID),
				map[string]any{"status": define.CustomerStatusLead},
				custUpdatedAt(t, tenant.CustomerID), 0,
			)
			assert.Error(t, errLead, "%s: transition vào Lead(4) phải bị từ chối (D14)", tt.name)

			// Đích thay thế Contact(16) phải hợp lệ.
			updated, errContact := svc.Update(
				int(tenant.CustomerID),
				map[string]any{"status": define.CustomerStatusContact},
				custUpdatedAt(t, tenant.CustomerID), 0,
			)
			require.NoError(t, errContact, "%s: transition vào Contact(16) phải hợp lệ", tt.name)
			require.NotNil(t, updated)
		})
	}
}

// custUpdatedAt — updated_at hiện tại của khách (khoá lạc quan D6, tham số expectedUpdatedAt của Update).
func custUpdatedAt(t *testing.T, id uint) int64 {
	t.Helper()
	var ts int64
	require.NoError(t, testDB.Raw("SELECT updated_at FROM customer WHERE id = ?", id).Scan(&ts).Error)
	return ts
}
