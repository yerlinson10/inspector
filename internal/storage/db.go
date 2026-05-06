package storage

import (
	"inspector/internal/models"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dbPath string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}

	return DB.AutoMigrate(
		&models.Endpoint{},
		&models.RequestLog{},
		&models.SentRequest{},
	)
}

func Cleanup(maxRequests int) {
	if DB == nil || maxRequests <= 0 {
		return
	}
	var count int64
	if err := DB.Model(&models.RequestLog{}).Count(&count).Error; err != nil {
		return
	}
	if count > int64(maxRequests) {
		excess := count - int64(maxRequests)
		DB.Exec("DELETE FROM request_logs WHERE id IN (SELECT id FROM request_logs ORDER BY created_at ASC LIMIT ?)", excess)
	}
}

func StartCleanupWorker(maxRequests int, interval time.Duration) func() {
	if maxRequests <= 0 {
		return func() {}
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	stop := make(chan struct{})

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				Cleanup(maxRequests)
			case <-stop:
				return
			}
		}
	}()

	return func() {
		select {
		case <-stop:
			return
		default:
			close(stop)
		}
	}
}
