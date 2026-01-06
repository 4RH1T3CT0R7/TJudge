package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CursorType определяет тип курсора
type CursorType string

const (
	CursorTypeID        CursorType = "id"        // Cursor на основе UUID
	CursorTypeTimestamp CursorType = "timestamp" // Cursor на основе времени
	CursorTypeComposite CursorType = "composite" // Cursor с несколькими полями
)

// Cursor представляет позицию в пагинированном списке
type Cursor struct {
	Type      CursorType             `json:"type"`
	ID        *uuid.UUID             `json:"id,omitempty"`
	Timestamp *time.Time             `json:"timestamp,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"` // Для composite курсоров
}

// PageInfo содержит информацию о пагинации
type PageInfo struct {
	HasNextPage     bool    `json:"has_next_page"`
	HasPreviousPage bool    `json:"has_previous_page"`
	StartCursor     *string `json:"start_cursor,omitempty"`
	EndCursor       *string `json:"end_cursor,omitempty"`
}

// PageRequest представляет запрос пагинации
type PageRequest struct {
	First  *int    `json:"first,omitempty"`  // Количество элементов вперёд
	After  *string `json:"after,omitempty"`  // Cursor для следующей страницы
	Last   *int    `json:"last,omitempty"`   // Количество элементов назад
	Before *string `json:"before,omitempty"` // Cursor для предыдущей страницы
}

// NewIDCursor создаёт курсор на основе ID
func NewIDCursor(id uuid.UUID) *Cursor {
	return &Cursor{
		Type: CursorTypeID,
		ID:   &id,
	}
}

// NewTimestampCursor создаёт курсор на основе времени
func NewTimestampCursor(timestamp time.Time) *Cursor {
	return &Cursor{
		Type:      CursorTypeTimestamp,
		Timestamp: &timestamp,
	}
}

// NewCompositeCursor создаёт составной курсор
func NewCompositeCursor(fields map[string]interface{}) *Cursor {
	return &Cursor{
		Type:   CursorTypeComposite,
		Fields: fields,
	}
}

// Encode кодирует курсор в base64 строку
func (c *Cursor) Encode() (string, error) {
	if c == nil {
		return "", nil
	}

	data, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cursor: %w", err)
	}

	return base64.URLEncoding.EncodeToString(data), nil
}

// DecodeCursor декодирует курсор из base64 строки
func DecodeCursor(encoded string) (*Cursor, error) {
	if encoded == "" {
		return nil, nil
	}

	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cursor: %w", err)
	}

	var cursor Cursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cursor: %w", err)
	}

	return &cursor, nil
}

// GetLimit возвращает лимит записей для запроса
func (pr *PageRequest) GetLimit() int {
	if pr.First != nil {
		return *pr.First
	}
	if pr.Last != nil {
		return *pr.Last
	}
	return 20 // Default limit
}

// IsForward проверяет, является ли запрос forward pagination
func (pr *PageRequest) IsForward() bool {
	return pr.First != nil || pr.After != nil
}

// IsBackward проверяет, является ли запрос backward pagination
func (pr *PageRequest) IsBackward() bool {
	return pr.Last != nil || pr.Before != nil
}

// Validate валидирует запрос пагинации
func (pr *PageRequest) Validate() error {
	// Нельзя одновременно использовать first и last
	if pr.First != nil && pr.Last != nil {
		return fmt.Errorf("cannot use both 'first' and 'last' parameters")
	}

	// Нельзя одновременно использовать after и before
	if pr.After != nil && pr.Before != nil {
		return fmt.Errorf("cannot use both 'after' and 'before' parameters")
	}

	// first/last должны быть положительными
	if pr.First != nil && *pr.First <= 0 {
		return fmt.Errorf("'first' must be positive")
	}
	if pr.Last != nil && *pr.Last <= 0 {
		return fmt.Errorf("'last' must be positive")
	}

	// Ограничение максимального размера страницы
	const maxPageSize = 100
	if pr.First != nil && *pr.First > maxPageSize {
		return fmt.Errorf("'first' cannot exceed %d", maxPageSize)
	}
	if pr.Last != nil && *pr.Last > maxPageSize {
		return fmt.Errorf("'last' cannot exceed %d", maxPageSize)
	}

	return nil
}

// GetCursor возвращает декодированный курсор
func (pr *PageRequest) GetCursor() (*Cursor, error) {
	if pr.After != nil {
		return DecodeCursor(*pr.After)
	}
	if pr.Before != nil {
		return DecodeCursor(*pr.Before)
	}
	return nil, nil
}
