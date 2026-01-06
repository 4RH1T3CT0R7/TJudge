package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Request представляет один запрос в batch
type Request struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// Response представляет ответ на один запрос в batch
type Response struct {
	ID         string            `json:"id"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       json.RawMessage   `json:"body,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// BatchRequest - запрос на batch обработку
type BatchRequest struct {
	Requests []Request `json:"requests"`
}

// BatchResponse - ответ batch обработки
type BatchResponse struct {
	Responses []Response `json:"responses"`
}

// Config конфигурация batch handler
type Config struct {
	MaxRequests    int           // Максимальное количество запросов в batch
	MaxBodySize    int64         // Максимальный размер body одного запроса
	Timeout        time.Duration // Таймаут на обработку всего batch
	RequestTimeout time.Duration // Таймаут на один запрос
	AllowedMethods []string      // Разрешённые HTTP методы
	AllowedPaths   []string      // Разрешённые пути (prefix match)
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() Config {
	return Config{
		MaxRequests:    10,
		MaxBodySize:    1 << 20, // 1MB
		Timeout:        30 * time.Second,
		RequestTimeout: 10 * time.Second,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowedPaths:   []string{"/api/v1/"},
	}
}

// Handler обрабатывает batch запросы
type Handler struct {
	config  Config
	handler http.Handler
}

// NewHandler создаёт новый batch handler
func NewHandler(handler http.Handler, config Config) *Handler {
	return &Handler{
		config:  config,
		handler: handler,
	}
}

// ServeHTTP обрабатывает batch запрос
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Ограничиваем размер тела запроса
	r.Body = http.MaxBytesReader(w, r.Body, h.config.MaxBodySize*int64(h.config.MaxRequests))

	var batchReq BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&batchReq); err != nil {
		http.Error(w, "Invalid batch request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Проверяем количество запросов
	if len(batchReq.Requests) == 0 {
		http.Error(w, "Empty batch request", http.StatusBadRequest)
		return
	}
	if len(batchReq.Requests) > h.config.MaxRequests {
		http.Error(w, fmt.Sprintf("Too many requests in batch: max %d", h.config.MaxRequests), http.StatusBadRequest)
		return
	}

	// Валидируем все запросы
	for i, req := range batchReq.Requests {
		if err := h.validateRequest(&req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request %d: %s", i, err.Error()), http.StatusBadRequest)
			return
		}
	}

	// Создаём контекст с таймаутом
	ctx, cancel := context.WithTimeout(r.Context(), h.config.Timeout)
	defer cancel()

	// Обрабатываем запросы параллельно
	responses := h.processBatch(ctx, r, batchReq.Requests)

	// Отправляем ответ
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(BatchResponse{Responses: responses})
}

// validateRequest проверяет валидность запроса
func (h *Handler) validateRequest(req *Request) error {
	if req.ID == "" {
		return fmt.Errorf("request ID is required")
	}
	if req.Method == "" {
		return fmt.Errorf("method is required")
	}
	if req.Path == "" {
		return fmt.Errorf("path is required")
	}

	// Проверяем метод
	methodAllowed := false
	for _, m := range h.config.AllowedMethods {
		if m == req.Method {
			methodAllowed = true
			break
		}
	}
	if !methodAllowed {
		return fmt.Errorf("method %s not allowed", req.Method)
	}

	// Проверяем путь
	pathAllowed := false
	for _, p := range h.config.AllowedPaths {
		if len(req.Path) >= len(p) && req.Path[:len(p)] == p {
			pathAllowed = true
			break
		}
	}
	if !pathAllowed {
		return fmt.Errorf("path %s not allowed", req.Path)
	}

	return nil
}

// processBatch обрабатывает все запросы параллельно
func (h *Handler) processBatch(ctx context.Context, originalReq *http.Request, requests []Request) []Response {
	responses := make([]Response, len(requests))
	var wg sync.WaitGroup

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, r Request) {
			defer wg.Done()
			responses[idx] = h.processRequest(ctx, originalReq, r)
		}(i, req)
	}

	wg.Wait()
	return responses
}

// processRequest обрабатывает один запрос
func (h *Handler) processRequest(ctx context.Context, originalReq *http.Request, req Request) Response {
	// Создаём контекст с таймаутом для одного запроса
	reqCtx, cancel := context.WithTimeout(ctx, h.config.RequestTimeout)
	defer cancel()

	// Создаём новый HTTP запрос
	httpReq, err := http.NewRequestWithContext(reqCtx, req.Method, req.Path, nil)
	if err != nil {
		return Response{
			ID:         req.ID,
			StatusCode: http.StatusInternalServerError,
			Error:      "Failed to create request: " + err.Error(),
		}
	}

	// Копируем заголовки из оригинального запроса
	for key, values := range originalReq.Header {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	// Добавляем заголовки из batch запроса
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Устанавливаем body если есть
	if len(req.Body) > 0 {
		httpReq.Body = &bodyReader{data: req.Body}
		httpReq.ContentLength = int64(len(req.Body))
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// Создаём response writer для захвата ответа
	rw := &responseCapture{
		headers: make(http.Header),
	}

	// Обрабатываем запрос
	h.handler.ServeHTTP(rw, httpReq)

	// Формируем ответ
	resp := Response{
		ID:         req.ID,
		StatusCode: rw.statusCode,
		Headers:    make(map[string]string),
	}

	// Копируем заголовки
	for key, values := range rw.headers {
		if len(values) > 0 {
			resp.Headers[key] = values[0]
		}
	}

	// Копируем body
	if len(rw.body) > 0 {
		resp.Body = rw.body
	}

	return resp
}

// bodyReader реализует io.ReadCloser для body
type bodyReader struct {
	data   []byte
	offset int
}

func (b *bodyReader) Read(p []byte) (n int, err error) {
	if b.offset >= len(b.data) {
		return 0, fmt.Errorf("EOF")
	}
	n = copy(p, b.data[b.offset:])
	b.offset += n
	return n, nil
}

func (b *bodyReader) Close() error {
	return nil
}

// responseCapture захватывает HTTP ответ
type responseCapture struct {
	headers    http.Header
	statusCode int
	body       []byte
}

func (r *responseCapture) Header() http.Header {
	return r.headers
}

func (r *responseCapture) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return len(b), nil
}

func (r *responseCapture) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}
