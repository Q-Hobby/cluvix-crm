package validation

// Reusable validator tag combos for go-playground/validator (used qua
// Gin binding `binding:"..."`). Centralizing tag strings tránh drift giữa
// các DTO ("min=8,max=72" vs "min=8,max=70" rải rác).
//
// Cách dùng:
//
//	type SignupInput struct {
//	    Email    string `json:"email"    binding:"`+validation.RuleEmail+`"`
//	    Password string `json:"password" binding:"`+validation.RulePassword+`"`
//	}
//
// Vì Go struct tag chỉ accept string literal, không phải concatenation, cách
// dùng thực tế là **đặt constant vào tag string trực tiếp** hoặc dùng helper
// `Bind` trong cùng package mà validator-translation đã hỗ trợ trong
// `pkg/response/validator.go`.
//
// Khuyến nghị: copy giá trị constant vào tag để IDE auto-complete tốt.
// Constants ở đây = single source of truth khi cần đổi rule.
const (
	RuleEmail    = "required,email,max=255"
	RuleEmailOpt = "omitempty,email,max=255"

	// RulePassword: 8–72 ký tự (72 = bcrypt limit). Không enforce
	// complexity ở tag — để service layer check (zxcvbn / blacklist).
	RulePassword = "required,min=8,max=72"

	// RulePhoneVN: VN phone — 0XXXXXXXXX hoặc +84XXXXXXXXX (9–10 số sau prefix).
	// Custom tag, đăng ký trong RegisterCustomTags().
	RulePhoneVN    = "required,phone_vn"
	RulePhoneVNOpt = "omitempty,phone_vn"

	RuleUsername = "required,alphanum,min=3,max=32"

	// RuleID: uint64 ID > 0 (FK reference).
	RuleID    = "required,gt=0"
	RuleIDOpt = "omitempty,gt=0"

	// RuleVersion: optimistic lock version (B.3). 0 = chưa load, hợp lệ.
	RuleVersion = "required,gte=0"
)
