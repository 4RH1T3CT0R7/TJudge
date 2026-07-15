// Package errors — ошибки приложения с http кодами
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError ошибка с http кодом и сообщением для юзера
type AppError struct {
	Code    int    // http код
	Message string // что показать пользователю
	Err     error  // внутренняя ошибка, наружу не отдаём
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap чтобы работал errors.As/Is по цепочке
func (e *AppError) Unwrap() error {
	return e.Err
}

func New(code int, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Wrap оборачивает ошибку с текстом. обязательно через %w -
// иначе выше по стеку не достанем http код из обёрнутого сентинела
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

var (
	// аутентификация
	ErrUnauthorized       = New(http.StatusUnauthorized, "Unauthorized", nil)
	ErrInvalidToken       = New(http.StatusUnauthorized, "Invalid token", nil)
	ErrTokenExpired       = New(http.StatusUnauthorized, "Token expired", nil)
	ErrInvalidCredentials = New(http.StatusUnauthorized, "Invalid credentials", nil)

	// валидация
	ErrValidation   = New(http.StatusBadRequest, "Validation failed", nil)
	ErrInvalidInput = New(http.StatusBadRequest, "Invalid input", nil)
	ErrBadRequest   = New(http.StatusBadRequest, "Bad request", nil)

	// ресурсы
	ErrNotFound      = New(http.StatusNotFound, "Resource not found", nil)
	ErrAlreadyExists = New(http.StatusConflict, "Resource already exists", nil)
	ErrConflict      = New(http.StatusConflict, "Conflict", nil)

	ErrForbidden = New(http.StatusForbidden, "Forbidden", nil)

	ErrRateLimitExceeded = New(http.StatusTooManyRequests, "Rate limit exceeded", nil)

	ErrInternal = New(http.StatusInternalServerError, "Internal server error", nil)

	// бизнес-логика
	ErrTournamentFull    = New(http.StatusConflict, "Tournament is full", nil)
	ErrTournamentStarted = New(http.StatusConflict, "Tournament already started", nil)
	ErrProgramNotFound   = New(http.StatusNotFound, "Program not found", nil)
	ErrConcurrentUpdate  = New(http.StatusConflict, "Concurrent update detected", nil)
)

// WithMessage новая ошибка с тем же кодом но другим текстом.
// именно НОВАЯ - сентинелы это глобалы, мутировать их нельзя
func (e *AppError) WithMessage(msg string) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: msg,
		Err:     e.Err,
	}
}

// WithError подкладывает внутреннюю ошибку (тоже копией)
func (e *AppError) WithError(err error) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: e.Message,
		Err:     err,
	}
}

func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

func GetAppError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}

// ToAppError достаёт AppError из ошибки, а если это не наша ошибка -
// заворачивает в 500
func ToAppError(err error) *AppError {
	if err == nil {
		return nil
	}

	if appErr := GetAppError(err); appErr != nil {
		return appErr
	}

	return ErrInternal.WithError(err)
}

func IsNotFound(err error) bool {
	appErr := GetAppError(err)
	if appErr != nil {
		return appErr.Code == http.StatusNotFound
	}
	return false
}
