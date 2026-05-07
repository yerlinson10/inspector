package storage

import (
	"fmt"
	"inspector/internal/models"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dbPath string) error {
	if err := ensureDatabaseDir(dbPath); err != nil {
		return err
	}

	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}
	if err := configureSQLite(dbPath); err != nil {
		return err
	}

	return DB.AutoMigrate(
		&models.Endpoint{},
		&models.MockRule{},
		&models.RequestLog{},
		&models.SentRequest{},
	)
}

func configureSQLite(dbPath string) error {
	trimmed := strings.TrimSpace(dbPath)
	if trimmed == "" {
		return fmt.Errorf("database path is required")
	}
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	if !strings.HasPrefix(trimmed, ":memory:") {
		if err := DB.Exec("PRAGMA journal_mode = WAL;").Error; err != nil {
			return err
		}
	}
	if err := DB.Exec("PRAGMA busy_timeout = 5000;").Error; err != nil {
		return err
	}
	if err := DB.Exec("PRAGMA synchronous = NORMAL;").Error; err != nil {
		return err
	}
	if err := DB.Exec("PRAGMA foreign_keys = ON;").Error; err != nil {
		return err
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	return nil
}

func ensureDatabaseDir(dbPath string) error {
	trimmed := strings.TrimSpace(dbPath)
	if trimmed == "" {
		return fmt.Errorf("database path is required")
	}
	if strings.HasPrefix(trimmed, ":memory:") {
		return nil
	}

	dir := filepath.Dir(trimmed)
	if dir == "." || dir == "" {
		return nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create database directory %s: %w", dir, err)
	}

	return nil
}

func Cleanup(maxRequests int) {
	if DB == nil || maxRequests <= 0 {
		return
	}
	cleanupTable(&models.RequestLog{}, "request_logs", maxRequests)
	cleanupTable(&models.SentRequest{}, "sent_requests", maxRequests)
}

func cleanupTable(model interface{}, table string, maxRows int) {
	var count int64
	if err := DB.Model(model).Count(&count).Error; err != nil {
		return
	}
	if count > int64(maxRows) {
		excess := count - int64(maxRows)
		DB.Exec("DELETE FROM "+table+" WHERE id IN (SELECT id FROM "+table+" ORDER BY created_at ASC, id ASC LIMIT ?)", excess)
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
