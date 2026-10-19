package export

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/internal/middleware"
	pkgExport "erpCluvix/pkg/export"
)

// DB is set once at bootstrap via SetDB.
var DB *gorm.DB

func SetDB(db *gorm.DB) { DB = db }

// doExport wraps pkg/export.Export and records a history_export row on success.
func doExport(c *gin.Context, q *gorm.DB, cfg pkgExport.Config, mapper pkgExport.RowMapper) (string, error) {
	position := cfg.FileName
	// File ghi vào thư mục của người tạo — endpoint tải chỉ phục vụ file của chính user đó (S-2).
	cfg.OwnerID = middleware.GetUserID(c)
	cfg.OnSuccess = func(count int64) {
		recordHistory(c, position, count)
	}
	return pkgExport.Export(q, cfg, mapper)
}

// jsonStr encode s thành JSON string ("...") để insert vào cột kiểu JSON.
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func recordHistory(c *gin.Context, position string, count int64) {
	if DB == nil {
		return
	}
	userID := int(middleware.GetUserID(c))
	n := int(count)
	now := int(time.Now().Unix())
	// KHÔNG nuốt lỗi: trước đây `Create` không kiểm `.Error`, nên dòng audit ghi hỏng sẽ biến mất
	// mà export vẫn trả URL thành công — không dấu hiệu nào. Đã gặp thật khi `createdBy` không thoả
	// FK `fk-history_export-createdBy` (user đã xoá, hoặc context thiếu userID ⇒ userID = 0):
	// MySQL trả 1452, export vẫn "thành công", `history_export` thiếu bản ghi.
	//
	// Vẫn KHÔNG trả lỗi ra ngoài: file export đã ghi xong và người dùng có quyền tải nó; hỏng audit
	// không phải lý do để huỷ kết quả. Nhưng phải LỘ RA trong log để còn truy được.
	if err := DB.Table("history_export").Create(map[string]interface{}{
		"positionExport": position,
		"quantityRecord": n,
		"ip_address":     c.ClientIP(),
		"userAgent":      jsonStr(c.GetHeader("User-Agent")),
		"createdBy":      userID,
		"createdAt":      now,
	}).Error; err != nil {
		log.Printf("[export] GHI AUDIT THẤT BẠI position=%q rows=%d createdBy=%d: %v — "+
			"file export VẪN được trả về, nhưng history_export thiếu bản ghi này", position, n, userID, err)
	}
}
