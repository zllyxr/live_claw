package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
)

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func auditAdmin(
	ctx context.Context,
	executor sqlExecutor,
	r *http.Request,
	action string,
	resourceType string,
	resourceID any,
	before any,
	after any,
) error {
	adminUser, _ := adminFromRequest(r)
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = executor.ExecContext(ctx, `
		INSERT INTO audit_logs
			(request_id,actor_type,actor_id,action,resource_type,resource_id,
			 before_data,after_data,ip,user_agent)
		VALUES(?,1,?,?,?,?,CAST(? AS JSON),CAST(? AS JSON),?,?)`,
		httpx.RequestID(ctx), adminUser.ID, action, resourceType, resourceID,
		string(beforeJSON), string(afterJSON), clientIP(r), r.UserAgent(),
	)
	return err
}
