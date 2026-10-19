package dao

import (
	"encoding/json"
	"fmt"

	"erpCluvix/internal/models"
)

// FlexString là string có thể unmarshal từ cả JSON string lẫn JSON number
// Dùng cho các field như số tài khoản ngân hàng — FE hay gửi dạng number
type FlexString string

func (f *FlexString) UnmarshalJSON(data []byte) error {
	// Thử unmarshal dạng string trước
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexString(s)
		return nil
	}
	// Nếu không được thì thử dạng number
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexString(n.String())
		return nil
	}
	return fmt.Errorf("cannot unmarshal %s into FlexString", data)
}

// BankAccount lưu thông tin tài khoản ngân hàng để sinh QR VietQR khi in bill
type BankAccount struct {
	models.BaseModel
	BankCode    string     `gorm:"column:bank_code;size:20;not null;comment:Mã ngân hàng theo chuẩn VietQR/Napas (VCB, TCB, MB...)" json:"bank_code" binding:"required"`
	AccountNo   FlexString `gorm:"column:account_no;size:50;not null;comment:Số tài khoản ngân hàng" json:"account_no" binding:"required"`
	AccountName string     `gorm:"column:account_name;size:100;not null;comment:Tên chủ tài khoản" json:"account_name" binding:"required"`
	SortOrder   *int       `gorm:"column:sort_order;default:0;comment:Thứ tự hiển thị (BE tự set = id sau create)" json:"sort_order"`
}

func (BankAccount) TableName() string { return "bank_accounts" }
