//go:build integration

package handler

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
)

// TestGetPayment_NotFound: GET /payment/:id for an ID that does not exist → 404.
func TestGetPayment_NotFound(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	h := New(Deps{DB: testDB})
	r := testhelpers.NewGin()
	r.Use(testhelpers.AuthMiddleware(1, 1))
	r.GET("/:id", h.GetPayment)

	req := httptest.NewRequest("GET", "/99999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestGetPayment_WithAuth_NoData: confirms that a request WITH auth middleware
// reaches the handler and returns a clean 404 (not a 500 panic) when no data exists.
// This verifies the auth middleware → tenant.ScopedDB path does not blow up.
func TestGetPayment_WithAuth_NoData(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	h := New(Deps{DB: testDB})
	r := testhelpers.NewGin()
	r.Use(testhelpers.AuthMiddleware(99998, 99998))
	r.GET("/:id", h.GetPayment)

	req := httptest.NewRequest("GET", "/88888", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should be 404 — not a 500 panic, proving the auth+scoping path is safe.
	assert.Equal(t, 404, w.Code)
}

// TestGetPayment_Found: seed a real payment, then GET by ID → 200 with data.
// The auth middleware must carry the same branchID used when seeding.
func TestGetPayment_Found(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	payID, err := testhelpers.CreatePayment(testDB, tenant, decimal.NewFromInt(500), define.PaymentForDeposit)
	require.NoError(t, err)

	h := New(Deps{DB: testDB})
	r := testhelpers.NewGin()
	r.Use(testhelpers.AuthMiddleware(tenant.BranchID, 1))
	r.GET("/:id", h.GetPayment)

	req := httptest.NewRequest("GET", fmt.Sprintf("/%d", payID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "data")
}
