package define

// BusinessType — loại hình kinh doanh cấp brand, mapping với company.business_type_id
const (
	BusinessPrivate         = 1 // Tư nhân
	BusinessPublic          = 2 // Bệnh viện công
	BusinessClinic          = 3 // Phòng khám
	BusinessClinicGeneral   = 4 // Phòng khám đa khoa
	BusinessClinicSpecial   = 5 // Phòng khám chuyên khoa
	BusinessHospitalPrivate = 6 // Bệnh viện tư nhân
	BusinessMedicalCenter   = 7 // Trung tâm y tế
	BusinessBeautyCenter    = 8 // Thẩm mỹ viện
	BusinessSpa             = 9 // Spa
)

// IsValidBusinessType — business_type_id nằm trong tập hằng BusinessPrivate..BusinessSpa.
func IsValidBusinessType(t int) bool {
	return t >= BusinessPrivate && t <= BusinessSpa
}
