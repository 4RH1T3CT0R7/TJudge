package storage

import (
	"context"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
)

type AuditLogRepository struct {
	db *DB
}

func NewAuditLogRepository(db *DB) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

// Insert пишет одну запись, дёргается асинхронно из middleware
func (r *AuditLogRepository) Insert(ctx context.Context, e *domain.AuditLogEntry) error {
	const query = `
		INSERT INTO audit_log (id, actor_id, actor_role, action, target_type, target_id,
		                      method, path, status_code, ip, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.db.ExecContext(ctx, query,
		e.ID, e.ActorID, e.ActorRole, e.Action, e.TargetType, e.TargetID,
		e.Method, e.Path, e.StatusCode, e.IP, e.UserAgent, e.CreatedAt,
	)
	if err != nil {
		return errors.Wrap(err, "failed to insert audit log")
	}
	return nil
}

// List отдаёт последние N записей для /admin/audit
func (r *AuditLogRepository) List(ctx context.Context, limit int) ([]*domain.AuditLogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	const query = `
		SELECT id, actor_id, actor_role, action, target_type, target_id,
		       method, path, status_code, ip, user_agent, created_at
		FROM audit_log
		ORDER BY created_at DESC
		LIMIT $1
	`
	var entries []*domain.AuditLogEntry
	if err := r.db.QueryWithMetrics(ctx, "audit_log_list", &entries, query, limit); err != nil {
		return nil, errors.Wrap(err, "failed to list audit log")
	}
	return entries, nil
}
