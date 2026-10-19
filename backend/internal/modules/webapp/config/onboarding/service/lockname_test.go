package service

import (
	"strings"
	"testing"

	"erpCluvix/pkg/define"
)

func TestBootstrapLockName_IncludesDatabase(t *testing.T) {
	got := BootstrapLockName("cluvix_khach_a")
	if got != define.OnboardingBootstrapLock+":cluvix_khach_a" {
		t.Fatalf("BootstrapLockName = %q", got)
	}
	if BootstrapLockName("cluvix_khach_a") == BootstrapLockName("cluvix_khach_b") {
		t.Fatal("2 DB khác nhau ra cùng tên khoá — khoá lẫn nhau trên cùng instance MySQL")
	}
}

func TestBootstrapLockName_LongDatabaseName_FitsMySQLLimit(t *testing.T) {
	long := strings.Repeat("d", 64) // tên DB tối đa của MySQL
	a := BootstrapLockName(long)
	if len(a) > define.MySQLNamedLockMaxLen {
		t.Fatalf("tên khoá dài %d > %d: %q", len(a), define.MySQLNamedLockMaxLen, a)
	}
	if !strings.HasPrefix(a, define.OnboardingBootstrapLock+":") {
		t.Fatalf("mất tiền tố: %q", a)
	}
	if a != BootstrapLockName(long) {
		t.Fatal("tên khoá không tất định")
	}
	if a == BootstrapLockName(strings.Repeat("d", 63)+"e") {
		t.Fatal("2 tên DB dài khác nhau ra cùng tên khoá")
	}
	// Sát biên: đúng 64 ký tự thì giữ nguyên dạng đọc được.
	edge := strings.Repeat("x", define.MySQLNamedLockMaxLen-len(define.OnboardingBootstrapLock)-1)
	if got := BootstrapLockName(edge); got != define.OnboardingBootstrapLock+":"+edge {
		t.Fatalf("tên vừa đủ 64 bị băm: %q", got)
	}
}
