package handler

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"erpCluvix/internal/middleware"
	customerEntity "erpCluvix/internal/modules/webapp/crm/customer/dao"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/imaging"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
)

// UploadForCustomer POST /crm/customer/:id/attachments
// Upload file gắn thẳng vào customer (ref_type="customer").
func (h *Handler) UploadForCustomer(c *gin.Context) {
	customerID, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if response.RejectMultipartTooLarge(c) {
		return
	}

	h.upload(c, define.AttachmentRefCustomer, int(customerID), customerID)
}

// Upload POST /crm/attachment
// Upload file với ref_type (whitelist define.AttachmentRef*) + ref_id.
// customer_id được BE resolve tự động từ ref_type/ref_id khi có thể;
// nếu không resolve được thì bắt buộc caller cung cấp customer_id trong form.
func (h *Handler) Upload(c *gin.Context) {
	// Parse multipart SỚM: c.PostForm nuốt lỗi parse ⇒ body quá cỡ sẽ thành "thiếu ref_type" (422 sai).
	if response.RejectMultipartTooLarge(c) {
		return
	}
	refType := c.PostForm("ref_type")
	refIDStr := c.PostForm("ref_id")
	if refType == "" || refIDStr == "" {
		response.Validate(c, i18n.T(c, "attachment_ref_required"))
		return
	}
	if !isAllowedRefType(refType) {
		response.Validate(c, i18n.T(c, "attachment_ref_type_invalid"))
		return
	}

	refID, err := strconv.ParseUint(refIDStr, 10, 31)
	if err != nil || refID == 0 {
		response.Validate(c, i18n.T(c, "attachment_ref_id_invalid"))
		return
	}

	customerID, resolveErr := h.resolveCustomerID(c, refType, int(refID))
	if resolveErr != nil {
		// Không resolve được — yêu cầu caller truyền
		cidStr := c.PostForm("customer_id")
		if cidStr == "" {
			response.Validate(c, i18n.TF(c, "attachment_customer_required", refType))
			return
		}
		cid, e := strconv.ParseUint(cidStr, 10, 31)
		if e != nil || cid == 0 {
			response.Validate(c, i18n.T(c, "attachment_customer_id_invalid"))
			return
		}
		customerID = int(cid)
	}

	h.upload(c, refType, int(refID), uint(customerID))
}

// isAllowedRefType — whitelist ref_type (FE hiện chỉ gửi "customer" qua UploadForCustomer).
func isAllowedRefType(refType string) bool {
	switch refType {
	case define.AttachmentRefCustomer, define.AttachmentRefAppointment:
		return true
	}
	return false
}

// resolveCustomerID lấy customer_id từ ref_type + ref_id (ref_type ĐÃ qua isAllowedRefType).
func (h *Handler) resolveCustomerID(c *gin.Context, refType string, refID int) (int, error) {
	var customerID int
	switch refType {
	case define.AttachmentRefCustomer:
		return refID, nil
	case define.AttachmentRefAppointment:
		err := tenant.ScopedDBNullBranch(c, h.DB, "appointment").
			Table("appointment").Select("customer_id").Where("id = ?", refID).Scan(&customerID).Error
		return customerID, err
	default:
		return 0, fmt.Errorf("ref_type '%s' không hỗ trợ tự động resolve customer_id", refType)
	}
}

// maxFileSizeBytes — ceiling tuyệt đối BE chấp nhận cho 1 file (define.AttachmentMaxFileMB).
// FE giới hạn thấp hơn theo từng tab (audio/file: 20 MB, video: 31 MB — chừa biên dưới trần BE).
const maxFileSizeBytes = define.AttachmentMaxFileMB * 1024 * 1024

// upload — logic chung: kiểm quyền xem khách, validate, lưu vào kho PRIVATE, lưu DB.
func (h *Handler) upload(c *gin.Context, refType string, refID int, customerID uint) {
	if h.PrivateStore == nil {
		response.ServerError(c, fmt.Errorf("storage private chưa được cấu hình"))
		return
	}

	// Không mở được hồ sơ khách ⇒ không được gắn file vào khách đó (trước đây customer_id từ
	// path/form đi thẳng vào DB, gắn được file vào khách ngoài phạm vi).
	visible, err := h.customerVisible(c, customerID)
	if err != nil {
		response.ServerError(c, err)
		return
	}
	if !visible {
		response.NotFound(c)
		return
	}

	file, err := c.FormFile("file")
	if response.IsPayloadTooLarge(err) {
		response.PayloadTooLarge(c)
		return
	}
	if err != nil {
		response.Validate(c, i18n.T(c, "file_required"))
		return
	}

	if file.Size > maxFileSizeBytes {
		response.Validate(c, i18n.TF(c, "attachment_file_too_large", define.AttachmentMaxFileMB))
		return
	}

	// SaveUploadSmart phân nhánh theo MAGIC BYTES: ảnh → pipeline (resize 1280 + WebP);
	// PDF/video/audio/office → lưu nguyên bản (đuôi theo MIME đã sniff, không theo tên client).
	// Preset mặc định (không phải OptMedical) — đây là chứng từ hành chính, không phải bằng
	// chứng lâm sàng. Kho PRIVATE: không có URL công khai.
	res, err := imaging.SaveUploadSmart(c.Request.Context(), h.PrivateStore, file,
		"crm/attachments/"+strconv.FormatUint(uint64(customerID), 10)+"/", nil)
	if err != nil {
		// HandleError: lỗi từ pkg/imaging là *errs.LocalizedError, Error() trả về KEY i18n —
		// Validate(err.Error()) sẽ in thẳng key ra màn hình.
		response.HandleError(c, err)
		return
	}

	createdBy := middleware.GetUserID(c)

	branchIDVal, _ := strconv.ParseUint(c.PostForm("branch_id"), 10, 64)

	note := c.PostForm("note")

	att := customerEntity.Attachment{
		RefType:    refType,
		RefId:      refID,
		CustomerID: customerID,
		BranchID:   uintPtrIfNonZero(uint(branchIDVal)),
		// FileName giữ tên gốc để user nhận ra file của mình (chỉ dùng làm tên tải về); FileType/
		// FileSize lấy từ KẾT QUẢ trong kho vì ảnh đã được convert sang WebP. FileUrl = KEY kho private.
		FileName: file.Filename,
		FileUrl:  res.Key,
		FileType: res.MIME,
		FileSize: sizePtr(int(res.Size)),
		Note:     note,
	}
	att.CreatedBy = uintPtrIfNonZero(uint(createdBy))

	if err := h.DB.Create(&att).Error; err != nil {
		// File đã nằm trong kho nhưng không có bản ghi trỏ tới ⇒ dọn ngay, không để mồ côi.
		if delErr := h.PrivateStore.Delete(context.Background(), res.Key); delErr != nil {
			log.Printf("attachment: dọn file mồ côi %q sau lỗi tạo bản ghi thất bại: %v", res.Key, delErr)
		}
		response.ServerError(c, err)
		return
	}

	response.Data(c, present(att))
}

func sizePtr(v int) *int { return &v }
func uintPtrIfNonZero(v uint) *uint {
	if v == 0 {
		return nil
	}
	return &v
}
