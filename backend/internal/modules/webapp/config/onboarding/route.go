// Package onboarding — màn thiết lập ban đầu cho DB SaaS trắng (chưa có thương hiệu nào).
//
// ⚠ Mọi user đã đăng nhập đều gọi tới được handler ở đây (story-58 xoá gate chọn thương hiệu — trước
// đó prefix này nằm trong danh sách miễn gate).
// Toàn bộ kiểm quyền nằm TRONG handler, không dựa vào middleware/perm map (đang audit-mode).
package onboarding

import (
	"erpCluvix/internal/modules/webapp/config/onboarding/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/onboarding/status", h.Status)
	cfg.POST("/onboarding/bootstrap", h.Bootstrap)
}
