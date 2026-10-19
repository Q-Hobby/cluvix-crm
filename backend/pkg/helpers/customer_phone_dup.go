package helpers

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"

	"erpCluvix/pkg/define"
)

// IsCustomerPhoneDuplicate — err là lỗi MySQL 1062 trên UNIQUE index SĐT khách
// (define.CustomerPhoneUniqueIndex, D-3). Handler tạo/sửa/import khách dùng hàm này để trả lỗi i18n
// "customer_duplicate" thay vì 500/422 thô — 1062 trên index KHÁC (vd mã khách) vẫn đi đường cũ.
func IsCustomerPhoneDuplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 &&
		strings.Contains(mysqlErr.Message, define.CustomerPhoneUniqueIndex)
}
