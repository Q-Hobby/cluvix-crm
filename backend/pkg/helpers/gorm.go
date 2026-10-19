package helpers

import (
	"errors"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// SelectFields returns a GORM scope that selects only specified columns.
// Usage: Preload("Unit", helpers.SelectFields("id, name"))
func SelectFields(fields string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Select(fields)
	}
}

// IsDuplicateKeyError returns true if err is a MySQL 1062 duplicate key error.
func IsDuplicateKeyError(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
