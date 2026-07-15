package errors

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppError_Error_WithInnerError(t *testing.T) {
	innerErr := fmt.Errorf("inner error")
	appErr := New(http.StatusBadRequest, "outer message", innerErr)

	result := appErr.Error()

	assert.Equal(t, "outer message: inner error", result)
}

func TestAppError_Error_WithoutInnerError(t *testing.T) {
	appErr := New(http.StatusBadRequest, "just message", nil)

	result := appErr.Error()

	assert.Equal(t, "just message", result)
}

func TestAppError_Unwrap(t *testing.T) {
	innerErr := fmt.Errorf("inner error")
	appErr := New(http.StatusBadRequest, "outer", innerErr)

	unwrapped := appErr.Unwrap()

	assert.Equal(t, innerErr, unwrapped)
}

func TestAppError_Unwrap_Nil(t *testing.T) {
	appErr := New(http.StatusBadRequest, "message", nil)

	unwrapped := appErr.Unwrap()

	assert.Nil(t, unwrapped)
}

func TestNew(t *testing.T) {
	err := fmt.Errorf("some error")
	appErr := New(http.StatusNotFound, "not found", err)

	assert.Equal(t, http.StatusNotFound, appErr.Code)
	assert.Equal(t, "not found", appErr.Message)
	assert.Equal(t, err, appErr.Err)
}

func TestWrap_WithError(t *testing.T) {
	innerErr := fmt.Errorf("original error")
	wrapped := Wrap(innerErr, "wrapped")

	assert.NotNil(t, wrapped)
	assert.Contains(t, wrapped.Error(), "wrapped")
	assert.Contains(t, wrapped.Error(), "original error")

	// Должна быть разворачиваемой
	assert.True(t, errors.Is(wrapped, innerErr))
}

func TestWrap_NilError(t *testing.T) {
	wrapped := Wrap(nil, "message")

	assert.Nil(t, wrapped)
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     *AppError
		code    int
		message string
	}{
		{"ErrUnauthorized", ErrUnauthorized, http.StatusUnauthorized, "Unauthorized"},
		{"ErrInvalidToken", ErrInvalidToken, http.StatusUnauthorized, "Invalid token"},
		{"ErrTokenExpired", ErrTokenExpired, http.StatusUnauthorized, "Token expired"},
		{"ErrInvalidCredentials", ErrInvalidCredentials, http.StatusUnauthorized, "Invalid credentials"},
		{"ErrValidation", ErrValidation, http.StatusBadRequest, "Validation failed"},
		{"ErrInvalidInput", ErrInvalidInput, http.StatusBadRequest, "Invalid input"},
		{"ErrNotFound", ErrNotFound, http.StatusNotFound, "Resource not found"},
		{"ErrAlreadyExists", ErrAlreadyExists, http.StatusConflict, "Resource already exists"},
		{"ErrConflict", ErrConflict, http.StatusConflict, "Conflict"},
		{"ErrForbidden", ErrForbidden, http.StatusForbidden, "Forbidden"},
		{"ErrRateLimitExceeded", ErrRateLimitExceeded, http.StatusTooManyRequests, "Rate limit exceeded"},
		{"ErrInternal", ErrInternal, http.StatusInternalServerError, "Internal server error"},
		{"ErrTournamentFull", ErrTournamentFull, http.StatusConflict, "Tournament is full"},
		{"ErrTournamentStarted", ErrTournamentStarted, http.StatusConflict, "Tournament already started"},
		{"ErrProgramNotFound", ErrProgramNotFound, http.StatusNotFound, "Program not found"},
		{"ErrConcurrentUpdate", ErrConcurrentUpdate, http.StatusConflict, "Concurrent update detected"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.code, tc.err.Code)
			assert.Equal(t, tc.message, tc.err.Message)
		})
	}
}

func TestAppError_WithMessage(t *testing.T) {
	original := ErrNotFound
	custom := original.WithMessage("User not found")

	assert.Equal(t, "User not found", custom.Message)
	assert.Equal(t, original.Code, custom.Code)

	// Оригинал не должен измениться
	assert.Equal(t, "Resource not found", original.Message)
}

func TestAppError_WithError(t *testing.T) {
	original := ErrValidation
	innerErr := fmt.Errorf("email is required")
	custom := original.WithError(innerErr)

	assert.Equal(t, innerErr, custom.Err)
	assert.Equal(t, original.Code, custom.Code)
	assert.Equal(t, original.Message, custom.Message)

	// Оригинал не должен измениться
	assert.Nil(t, original.Err)
}

func TestIsAppError_True(t *testing.T) {
	appErr := ErrNotFound

	result := IsAppError(appErr)

	assert.True(t, result)
}

func TestIsAppError_Wrapped(t *testing.T) {
	appErr := ErrNotFound.WithMessage("user not found")
	wrapped := fmt.Errorf("wrapped: %w", appErr)

	result := IsAppError(wrapped)

	assert.True(t, result)
}

func TestIsAppError_False(t *testing.T) {
	regularErr := fmt.Errorf("regular error")

	result := IsAppError(regularErr)

	assert.False(t, result)
}

func TestIsAppError_Nil(t *testing.T) {
	result := IsAppError(nil)

	assert.False(t, result)
}

func TestGetAppError_Direct(t *testing.T) {
	appErr := ErrNotFound

	result := GetAppError(appErr)

	require.NotNil(t, result)
	assert.Equal(t, appErr.Code, result.Code)
}

func TestGetAppError_Wrapped(t *testing.T) {
	appErr := ErrForbidden.WithMessage("access denied")
	wrapped := fmt.Errorf("wrapped: %w", appErr)

	result := GetAppError(wrapped)

	require.NotNil(t, result)
	assert.Equal(t, http.StatusForbidden, result.Code)
	assert.Equal(t, "access denied", result.Message)
}

func TestGetAppError_NotAppError(t *testing.T) {
	regularErr := fmt.Errorf("regular error")

	result := GetAppError(regularErr)

	assert.Nil(t, result)
}

func TestGetAppError_Nil(t *testing.T) {
	result := GetAppError(nil)

	assert.Nil(t, result)
}

func TestToAppError_AlreadyAppError(t *testing.T) {
	appErr := ErrValidation.WithMessage("custom message")

	result := ToAppError(appErr)

	require.NotNil(t, result)
	assert.Equal(t, appErr.Code, result.Code)
	assert.Equal(t, appErr.Message, result.Message)
}

func TestToAppError_WrappedAppError(t *testing.T) {
	appErr := ErrNotFound
	wrapped := fmt.Errorf("context: %w", appErr)

	result := ToAppError(wrapped)

	require.NotNil(t, result)
	assert.Equal(t, http.StatusNotFound, result.Code)
}

func TestToAppError_RegularError(t *testing.T) {
	regularErr := fmt.Errorf("database connection failed")

	result := ToAppError(regularErr)

	require.NotNil(t, result)
	assert.Equal(t, http.StatusInternalServerError, result.Code)
	assert.Contains(t, result.Error(), "database connection failed")
}

func TestToAppError_Nil(t *testing.T) {
	result := ToAppError(nil)

	assert.Nil(t, result)
}

func TestAppError_ErrorChaining(t *testing.T) {
	// Создаём цепочку ошибок
	original := fmt.Errorf("original error")
	appErr := ErrValidation.WithError(original)
	wrapped := fmt.Errorf("context: %w", appErr)

	// errors.Is должен работать по всей цепочке
	assert.True(t, errors.Is(wrapped, original))

	// GetAppError должен найти AppError
	result := GetAppError(wrapped)
	require.NotNil(t, result)
	assert.Equal(t, http.StatusBadRequest, result.Code)
}

func TestAppError_Immutability(t *testing.T) {
	// Проверяем, что методы With* не изменяют оригинал
	original := ErrNotFound

	_ = original.WithMessage("custom")
	_ = original.WithError(fmt.Errorf("inner"))

	// Оригинал не должен измениться
	assert.Equal(t, "Resource not found", original.Message)
	assert.Nil(t, original.Err)
}

// --- IsNotFound ---

func TestIsNotFound_DirectAppError(t *testing.T) {
	assert.True(t, IsNotFound(ErrNotFound))
}

func TestIsNotFound_WrappedAppError(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", ErrNotFound)
	assert.True(t, IsNotFound(wrapped))
}

func TestIsNotFound_WrongCode(t *testing.T) {
	assert.False(t, IsNotFound(ErrForbidden))
}

func TestIsNotFound_Nil(t *testing.T) {
	assert.False(t, IsNotFound(nil))
}

func TestIsNotFound_RegularError(t *testing.T) {
	assert.False(t, IsNotFound(fmt.Errorf("regular error")))
}

func TestIsNotFound_ProgramNotFound(t *testing.T) {
	// ErrProgramNotFound тоже имеет код 404, поэтому IsNotFound должен вернуть true
	assert.True(t, IsNotFound(ErrProgramNotFound))
}

func TestAppError_WithMessage_WithError_Chaining(t *testing.T) {
	innerErr := fmt.Errorf("db error")
	custom := ErrNotFound.WithMessage("user not found").WithError(innerErr)

	assert.Equal(t, "user not found", custom.Message)
	assert.Equal(t, http.StatusNotFound, custom.Code)
	assert.Equal(t, innerErr, custom.Err)
	assert.Contains(t, custom.Error(), "user not found")
	assert.Contains(t, custom.Error(), "db error")
}

func TestWrap_PreservesIs(t *testing.T) {
	sentinel := fmt.Errorf("sentinel error")
	wrapped := Wrap(sentinel, "layer1")

	require.NotNil(t, wrapped)
	assert.True(t, errors.Is(wrapped, sentinel))

	// Двойное оборачивание
	doubleWrapped := Wrap(wrapped, "layer2")
	assert.True(t, errors.Is(doubleWrapped, sentinel))
}
