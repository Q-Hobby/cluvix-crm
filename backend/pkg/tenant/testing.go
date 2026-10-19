//go:build integration

package tenant

// RegisterTestTable cho phép integration test ở package khác (vd
// pkg/repository) đăng ký synthetic table vào branch whitelist.
// Chỉ build trong tag `integration` — không ảnh hưởng production binary.
// Story-41 (E6/D15): bỏ tham số hasCompany cùng registry bảng company.
func RegisterTestTable(table string, hasBranch bool) {
	if hasBranch {
		branchTables[table] = true
	}
}
