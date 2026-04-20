package domain

import (
	"time"

	"github.com/google/uuid"
)

// AuditLogEntry описывает одну запись audit-лога.
// Хранит admin-действия для последующего расследования инцидентов.
type AuditLogEntry struct {
	ID         uuid.UUID `db:"id"          json:"id"`
	ActorID    uuid.UUID `db:"actor_id"    json:"actor_id"`
	ActorRole  string    `db:"actor_role"  json:"actor_role"`
	Action     string    `db:"action"      json:"action"`
	TargetType string    `db:"target_type" json:"target_type,omitempty"`
	TargetID   string    `db:"target_id"   json:"target_id,omitempty"`
	Method     string    `db:"method"      json:"method,omitempty"`
	Path       string    `db:"path"        json:"path,omitempty"`
	StatusCode int       `db:"status_code" json:"status_code,omitempty"`
	IP         string    `db:"ip"          json:"ip,omitempty"`
	UserAgent  string    `db:"user_agent"  json:"user_agent,omitempty"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
}
