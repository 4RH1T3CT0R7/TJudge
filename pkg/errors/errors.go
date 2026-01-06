package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError - кастомная ошибка приложения с HTTP кодом
type AppError struct {
	Code    int    // HTTP код
	Message string // Сообщение для пользователя
	Err     error  // Внутренняя ошибка
}

// Error реализует интерфейс error
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap позволяет использовать errors.Unwrap
func (e *AppError) Unwrap() error {
	return e.Err
}

// New создаёт новую ошибку приложения
func New(code int, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Wrap оборачивает ошибку
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// Предопределённые ошибки

var (
	// Auth errors
	ErrUnauthorized       = New(http.StatusUnauthorized, "Unauthorized", nil)
	ErrInvalidToken       = New(http.StatusUnauthorized, "Invalid token", nil)
	ErrTokenExpired       = New(http.StatusUnauthorized, "Token expired", nil)
	ErrInvalidCredentials = New(http.StatusUnauthorized, "Invalid credentials", nil)

	// Validation errors
	ErrValidation   = New(http.StatusBadRequest, "Validation failed", nil)
	ErrInvalidInput = New(http.StatusBadRequest, "Invalid input", nil)
	ErrBadRequest   = New(http.StatusBadRequest, "Bad request", nil)
	ErrMissingField = New(http.StatusBadRequest, "Missing required field", nil)

	// Resource errors
	ErrNotFound      = New(http.StatusNotFound, "Resource not found", nil)
	ErrAlreadyExists = New(http.StatusConflict, "Resource already exists", nil)
	ErrConflict      = New(http.StatusConflict, "Conflict", nil)

	// Permission errors
	ErrForbidden        = New(http.StatusForbidden, "Forbidden", nil)
	ErrPermissionDenied = New(http.StatusForbidden, "Permission denied", nil)

	// Rate limiting errors
	ErrRateLimitExceeded = New(http.StatusTooManyRequests, "Rate limit exceeded", nil)

	// Server errors
	ErrInternal           = New(http.StatusInternalServerError, "Internal server error", nil)
	ErrServiceUnavailable = New(http.StatusServiceUnavailable, "Service unavailable", nil)
	ErrTimeout            = New(http.StatusGatewayTimeout, "Request timeout", nil)

	// Business logic errors
	ErrTournamentFull       = New(http.StatusConflict, "Tournament is full", nil)
	ErrTournamentStarted    = New(http.StatusConflict, "Tournament already started", nil)
	ErrTournamentNotStarted = New(http.StatusConflict, "Tournament not started yet", nil)
	ErrInvalidGameType      = New(http.StatusBadRequest, "Invalid game type", nil)
	ErrMatchInProgress      = New(http.StatusConflict, "Match is already in progress", nil)
	ErrProgramNotFound      = New(http.StatusNotFound, "Program not found", nil)
	ErrConcurrentUpdate     = New(http.StatusConflict, "Concurrent update detected", nil)
)

// WithMessage создаёт новую ошибку с кастомным сообщением
func (e *AppError) WithMessage(msg string) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: msg,
		Err:     e.Err,
	}
}

// WithError добавляет внутреннюю ошибку
func (e *AppError) WithError(err error) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: e.Message,
		Err:     err,
	}
}

// IsAppError проверяет, является ли ошибка AppError
func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

// GetAppError извлекает AppError из ошибки
func GetAppError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}

// ToAppError преобразует ошибку в AppError
func ToAppError(err error) *AppError {
	if err == nil {
		return nil
	}

	if appErr := GetAppError(err); appErr != nil {
		return appErr
	}

	return ErrInternal.WithError(err)
}

// IsNotFound проверяет, является ли ошибка типом "not found"
func IsNotFound(err error) bool {
	appErr := GetAppError(err)
	if appErr != nil {
		return appErr.Code == http.StatusNotFound
	}
	return false
}
