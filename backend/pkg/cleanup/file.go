package cleanup

import (
	"log"
	"os"
	"time"
)

// ScheduleFileDelete xoá file sau duration, thay thế Yii2 queue job DeleteFileWhenExportSuccess.
func ScheduleFileDelete(filePath string, after time.Duration) {
	time.AfterFunc(after, func() {
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			log.Printf("[cleanup] failed to delete %s: %v", filePath, err)
		}
	})
}
