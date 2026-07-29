package im

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Dispatcher struct {
	db     *sql.DB
	redis  *redis.Client
	logger *slog.Logger
}

type outboxEvent struct {
	ID          int64
	AggregateID string
	Payload     []byte
	Attempts    int
}

func NewDispatcher(db *sql.DB, redisClient *redis.Client, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{db: db, redis: redisClient, logger: logger}
}

func (d *Dispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	d.drain(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			d.drain(ctx)
		}
	}
}

func (d *Dispatcher) drain(ctx context.Context) {
	lock, err := d.redis.SetNX(
		ctx, "im:v2:outbox:lock", strconv.FormatInt(time.Now().UnixNano(), 10), 2*time.Second,
	).Result()
	if err != nil || !lock {
		return
	}
	defer d.redis.Del(context.Background(), "im:v2:outbox:lock") //nolint:errcheck
	for iteration := 0; iteration < 50; iteration++ {
		processed, processErr := d.processOne(ctx)
		if processErr != nil {
			d.logger.Error("dispatch IM outbox", "error", processErr)
			return
		}
		if !processed {
			return
		}
	}
}

func (d *Dispatcher) processOne(ctx context.Context) (bool, error) {
	tx, err := d.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback() //nolint:errcheck
	var event outboxEvent
	err = tx.QueryRowContext(ctx, `
		SELECT id,aggregate_id,payload,attempts
		FROM outbox_events
		WHERE event_type='im.message.created'
		  AND (
		    (status=0 AND available_at<=CURRENT_TIMESTAMP(3))
		    OR
		    (status=1 AND (
		      processing_started_at IS NULL
		      OR processing_started_at<=CURRENT_TIMESTAMP(3)-INTERVAL 30 SECOND
		    ))
		  )
		ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`,
	).Scan(&event.ID, &event.AggregateID, &event.Payload, &event.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE outbox_events
		SET status=1,attempts=attempts+1,processing_started_at=CURRENT_TIMESTAMP(3)
		WHERE id=?`,
		event.ID,
	); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}

	err = d.publish(ctx, event)
	if err == nil {
		_, updateErr := d.db.ExecContext(ctx, `
			UPDATE outbox_events
			SET status=2,processed_at=CURRENT_TIMESTAMP(3),processing_started_at=NULL,last_error=''
			WHERE id=?`,
			event.ID,
		)
		return true, updateErr
	}
	nextStatus := 0
	if event.Attempts+1 >= 10 {
		nextStatus = 3
	}
	delaySeconds := 1 << min(event.Attempts, 8)
	_, updateErr := d.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET status=?,available_at=CURRENT_TIMESTAMP(3)+INTERVAL ? SECOND,
		    processing_started_at=NULL,last_error=?
		WHERE id=?`,
		nextStatus, delaySeconds, truncateOutboxError(err.Error(), 1000), event.ID,
	)
	if updateErr != nil {
		return true, updateErr
	}
	return true, err
}

func (d *Dispatcher) publish(ctx context.Context, event outboxEvent) error {
	if !json.Valid(event.Payload) {
		return errors.New("invalid IM outbox payload")
	}
	envelope := append([]byte(`{"type":"message","data":`), event.Payload...)
	envelope = append(envelope, '}')
	rows, err := d.db.QueryContext(ctx, `
		SELECT user_id FROM im_conversation_members
		WHERE conversation_id=? AND member_status=1`,
		event.AggregateID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	pipe := d.redis.Pipeline()
	for rows.Next() {
		var userID int64
		if err = rows.Scan(&userID); err != nil {
			return err
		}
		pipe.Publish(ctx, "im:v2:user:"+strconv.FormatInt(userID, 10), envelope)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if _, err = pipe.Exec(ctx); err != nil {
		return fmt.Errorf("publish IM event: %w", err)
	}
	return nil
}

func truncateOutboxError(value string, length int) string {
	if len(value) <= length {
		return value
	}
	return value[:length]
}
