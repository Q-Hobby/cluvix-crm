// Package handler — OrderServiceUsed (dòng dịch vụ) HTTP layer.
//
// Plan B.2 (2026-05-16): Handler migrate ra layered module. Không có service
// file riêng (order_service_used logic inline trong handler). Đây cũng là file
// gốc định nghĩa attachRelation / attachRelationReverse / collectIDs — sau khi
// migrate ra module, copy về crm/helpers.go cho customer_detail.go vẫn còn dùng.
//
// Note: 2 endpoint thuộc /order-service-used group wire RIÊNG ở router/crm.go:
//   - /list-service-used-customer (ListServiceUsedCustomer ở customer_detail.go)
//   - /report-service-used (report.ReportServiceUsed ở report package)
package handler

import (
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"

	"gorm.io/gorm"
)

type Deps struct {
	DB *gorm.DB
	// RbacSvc — chỉ route.go dùng (guard export `?_export=1`, S-7).
	RbacSvc *authSvc.RbacService
}

type Handler struct {
	DB *gorm.DB
}

func New(d Deps) *Handler { return &Handler{DB: d.DB} }
