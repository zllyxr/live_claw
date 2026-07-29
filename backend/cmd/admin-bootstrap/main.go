package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/config"
	"github.com/zllyxr/live_claw/backend/internal/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}
	username := strings.TrimSpace(os.Getenv("V2_ADMIN_USERNAME"))
	password := os.Getenv("V2_ADMIN_PASSWORD")
	displayName := strings.TrimSpace(os.Getenv("V2_ADMIN_DISPLAY_NAME"))
	if username == "" || password == "" {
		logger.Error("V2_ADMIN_USERNAME and V2_ADMIN_PASSWORD are required")
		os.Exit(1)
	}
	if displayName == "" {
		displayName = username
	}
	passwordHash, err := adminauth.HashPassword(password)
	if err != nil {
		logger.Error("validate administrator password", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg.MySQLDSN)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		logger.Error("begin transaction", "error", err)
		os.Exit(1)
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(ctx, `
		INSERT INTO admin_users(username,password_hash,display_name,status)
		VALUES(?,?,?,1)`,
		username, passwordHash, displayName,
	)
	if err != nil {
		var mysqlErr *mysqlDriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			logger.Error("administrator already exists; bootstrap never overwrites passwords", "username", username)
		} else {
			logger.Error("create administrator", "error", err)
		}
		os.Exit(1)
	}
	adminID, err := result.LastInsertId()
	if err != nil {
		logger.Error("read administrator id", "error", err)
		os.Exit(1)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO admin_user_roles(admin_user_id,role_id)
		SELECT ?,id FROM admin_roles WHERE role_key='super_admin'`,
		adminID,
	); err != nil {
		logger.Error("assign super administrator role", "error", err)
		os.Exit(1)
	}
	if err = tx.Commit(); err != nil {
		logger.Error("commit administrator", "error", err)
		os.Exit(1)
	}
	logger.Info("administrator created", "username", username, "id", adminID)
}
