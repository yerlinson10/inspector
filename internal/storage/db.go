package storage

import (
	"inspector/internal/models"

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
	if maxRequests <= 0 {
		return
	}
	var count int64
	DB.Model(&models.RequestLog{}).Count(&count)
	if count > int64(maxRequests) {
		excess := count - int64(maxRequests)
		DB.Exec("DELETE FROM request_logs WHERE id IN (SELECT id FROM request_logs ORDER BY created_at ASC LIMIT ?)", excess)
	}
}
