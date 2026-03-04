package pagination

import (
	"net/http"
	"strconv"
)

// DefaultMaxLimit максимальный лимит по умолчанию
const DefaultMaxLimit = 100

// LimitOffset содержит распарсенные параметры пагинации
type LimitOffset struct {
	Limit  int
	Offset int
}

// ParseLimitOffset парсит limit и offset из query параметров запроса.
// defaultLimit используется, если параметр не указан.
// maxLimit ограничивает максимальное значение (0 = DefaultMaxLimit).
func ParseLimitOffset(r *http.Request, defaultLimit int, maxLimit int) LimitOffset {
	if maxLimit <= 0 {
		maxLimit = DefaultMaxLimit
	}

	limit := defaultLimit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= maxLimit {
			limit = l
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	return LimitOffset{Limit: limit, Offset: offset}
}
