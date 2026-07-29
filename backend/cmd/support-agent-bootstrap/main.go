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
	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}
	username := strings.TrimSpace(os.Getenv("V2_SUPPORT_AGENT_USERNAME"))
	password := os.Getenv("V2_SUPPORT_AGENT_PASSWORD")
	displayName := strings.TrimSpace(os.Getenv("V2_SUPPORT_AGENT_DISPLAY_NAME"))
	roleKey := strings.TrimSpace(os.Getenv("V2_SUPPORT_AGENT_ROLE"))
	if roleKey == "" {
		roleKey = "support_agent"
	}
	if username == "" || password == "" ||
		(roleKey != "support_agent" && roleKey != "support_supervisor") {
		logger.Error(
			"V2_SUPPORT_AGENT_USERNAME, V2_SUPPORT_AGENT_PASSWORD and a valid V2_SUPPORT_AGENT_ROLE are required",
		)
		os.Exit(1)
	}
	if displayName == "" {
		displayName = username
	}
	passwordHash, err := adminauth.HashPassword(password)
	if err != nil {
		logger.Error("validate support agent password", "error", err)
		os.Exit(1)
	}
	agentNo, err := idgen.New()
	if err != nil {
		logger.Error("create support agent number", "error", err)
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
			logger.Error("support agent already exists; bootstrap never overwrites passwords", "username", username)
		} else {
			logger.Error("create support agent login", "error", err)
		}
		os.Exit(1)
	}
	adminID, err := result.LastInsertId()
	if err != nil {
		logger.Error("read support agent id", "error", err)
		os.Exit(1)
	}
	roleValue := 1
	if roleKey == "support_supervisor" {
		roleValue = 2
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO admin_user_roles(admin_user_id,role_id)
		SELECT ?,id FROM admin_roles WHERE role_key=? AND status=1`,
		adminID, roleKey,
	); err != nil {
		logger.Error("assign support role", "error", err)
		os.Exit(1)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO support_agents
			(admin_user_id,agent_no,agent_role,status,presence,max_active,support_only)
		VALUES(?,?,?,1,0,8,1)`,
		adminID, agentNo, roleValue,
	); err != nil {
		logger.Error("create support agent profile", "error", err)
		os.Exit(1)
	}
	if err = tx.Commit(); err != nil {
		logger.Error("commit support agent", "error", err)
		os.Exit(1)
	}
	logger.Info(
		"support agent created",
		"username", username, "display_name", displayName,
		"role", roleKey, "id", adminID, "agent_no", agentNo,
	)
}
