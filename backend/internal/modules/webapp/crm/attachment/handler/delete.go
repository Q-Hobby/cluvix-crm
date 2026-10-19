package handler

import (
	"errors"
	"fmt"
	"log"

	"erpCluvix/internal/middleware"
	customerEntity "erpCluvix/internal/modules/webapp/crm/customer/dao"
	history "erpCluvix/internal/modules/webapp/history/log/service"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/storage"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Delete DELETE /crm/attachment/:id
// Xoá attachment: xoá bản ghi DB + file trong kho, NGUYÊN TỬ theo hướng an toàn.
//
// Thứ tự: xoá bản ghi trong transaction → xoá file → commit. Xoá file lỗi ⇒ rollback bản ghi và trả
// 500 (user thử lại được). Trước đây lỗi xoá file bị nuốt (lại còn truyền URL đầy đủ vào hàm chờ
// key nên LẦN NÀO cũng lỗi) ⇒ bản ghi biến mất còn file chứa dữ liệu khách nằm lại vĩnh viễn trong
// kho mà không còn gì trỏ tới để dọn.
func (h *Handler) Delete(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if h.PrivateStore == nil {
		response.ServerError(c, fmt.Errorf("storage private chưa được cấu hình"))
		return
	}

	var att customerEntity.Attachment
	tx := tenant.ScopedDBNullBranch(c, h.DB, "attachment").
		Select("id, customer_id, file_name, file_url").Where("id = ?", id).Limit(1).Find(&att)
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}
	if att.ID == 0 {
		response.NotFound(c)
		return
	}
	visible, err := h.customerVisible(c, att.CustomerID)
	if err != nil {
		response.ServerError(c, err)
		return
	}
	if !visible {
		response.NotFound(c)
		return
	}

	err = h.DB.Transaction(func(dbTx *gorm.DB) error {
		res := tenant.ScopedDBNullBranch(c, dbTx, "attachment").
			Where("id = ?", att.ID).Delete(&customerEntity.Attachment{})
		if res.Error != nil {
			return res.Error
		}
		// Audit: file đính kèm là dữ liệu của khách ⇒ xoá phải để lại dấu vết (ai xoá — changed_by,
		// khi nào, file nào, của khách nào) trong lịch sử khách hàng. CÙNG transaction: ghi log lỗi
		// ⇒ rollback, không có lần xoá nào "vô hình". Ghi TRƯỚC khi xoá file trong kho (bước không
		// rollback được) để lỗi log không để lại bản ghi còn mà file đã mất.
		if err := history.LogChanges(dbTx, "log_customer", "customer_id", int(att.CustomerID),
			map[string]interface{}{attachmentLogField: attachmentLogValue(att)},
			map[string]interface{}{attachmentLogField: nil},
			[]string{attachmentLogField}, nil, int(middleware.GetUserID(c))); err != nil {
			return err
		}
		return h.deleteStoredFile(c, att.FileUrl)
	})
	if err != nil {
		response.ServerError(c, err)
		return
	}

	response.Deleted(c)
}

// attachmentLogField — key trong log_customer.changed_fields cho thao tác xoá file đính kèm
// (FE customer-log có nhãn riêng cho key này).
const attachmentLogField = "attachment"

// attachmentLogValue — "tên file (#id)": đủ để nhận ra file đã xoá sau khi bản ghi biến mất.
func attachmentLogValue(att customerEntity.Attachment) string {
	return fmt.Sprintf("%s (#%d)", att.FileName, att.ID)
}

// deleteStoredFile xoá file mà cột file_url trỏ tới.
//
//   - KEY (bản ghi mới) ⇒ xoá trong kho private; lỗi ⇒ trả lỗi (caller rollback).
//   - URL tuyệt đối (bản ghi cũ, object public) ⇒ bóc gốc URL kho public lấy key rồi xoá ở kho public;
//     URL không thuộc kho hiện tại (CDN đã đổi, link ngoài) ⇒ không thể xoá, log rồi cho xoá bản ghi
//     (giữ bản ghi cũng không giúp dọn được file đó).
func (h *Handler) deleteStoredFile(c *gin.Context, fileURL string) error {
	if fileURL == "" {
		return nil
	}
	ctx := c.Request.Context()
	if !storage.IsAbsoluteURL(fileURL) {
		err := h.PrivateStore.Delete(ctx, fileURL)
		if errors.Is(err, storage.ErrUnsafePath) {
			// Key hỏng không thể trỏ tới file nào trong kho — không có gì để xoá.
			log.Printf("attachment: file_url %q không phải key hợp lệ, bỏ qua xoá file", fileURL)
			return nil
		}
		return err
	}
	if h.PublicStore != nil {
		if key := storage.ToKey(h.PublicStore, fileURL); key != fileURL && storage.SafeKey(key) == nil {
			return h.PublicStore.Delete(ctx, key)
		}
	}
	log.Printf("attachment: file cũ %q không thuộc kho hiện tại — không xoá được file, chỉ xoá bản ghi", fileURL)
	return nil
}
