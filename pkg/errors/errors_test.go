package errors

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppError_Error(t *testing.T) {
	withInner := New(http.StatusBadRequest, "outer message", fmt.Errorf("inner error"))
	assert.Equal(t, "outer message: inner error", withInner.Error())

	noInner := New(http.StatusBadRequest, "just message", nil)
	assert.Equal(t, "just message", noInner.Error())
}

func TestNew(t *testing.T) {
	err := fmt.Errorf("some error")
	appErr := New(http.StatusNotFound, "not found", err)

	assert.Equal(t, http.StatusNotFound, appErr.Code)
	assert.Equal(t, "not found", appErr.Message)
	assert.Equal(t, err, appErr.Err)
}

func TestWrap(t *testing.T) {
	inner := fmt.Errorf("original error")
	wrapped := Wrap(inner, "wrapped")

	require.NotNil(t, wrapped)
	assert.Contains(t, wrapped.Error(), "wrapped")
	assert.Contains(t, wrapped.Error(), "original error")
	// главное - остаётся разворачиваемой, на этом висит деривация http кода
	assert.True(t, errors.Is(wrapped, inner))

	// двойная обёртка тоже разворачивается
	double := Wrap(wrapped, "layer2")
	assert.True(t, errors.Is(double, inner))

	assert.Nil(t, Wrap(nil, "message"))
}

// пары (код, сообщение) уходят на клиента, это контракт - проверяем все
func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		err     *AppError
		code    int
		message string
	}{
		{ErrUnauthorized, http.StatusUnauthorized, "Unauthorized"},
		{ErrInvalidToken, http.StatusUnauthorized, "Invalid token"},
		{ErrTokenExpired, http.StatusUnauthorized, "Token expired"},
		{ErrInvalidCredentials, http.StatusUnauthorized, "Invalid credentials"},
		{ErrValidation, http.StatusBadRequest, "Validation failed"},
		{ErrInvalidInput, http.StatusBadRequest, "Invalid input"},
		{ErrBadRequest, http.StatusBadRequest, "Bad request"},
		{ErrNotFound, http.StatusNotFound, "Resource not found"},
		{ErrAlreadyExists, http.StatusConflict, "Resource already exists"},
		{ErrConflict, http.StatusConflict, "Conflict"},
		{ErrForbidden, http.StatusForbidden, "Forbidden"},
		{ErrRateLimitExceeded, http.StatusTooManyRequests, "Rate limit exceeded"},
		{ErrInternal, http.StatusInternalServerError, "Internal server error"},
		{ErrTournamentFull, http.StatusConflict, "Tournament is full"},
		{ErrTournamentStarted, http.StatusConflict, "Tournament already started"},
		{ErrProgramNotFound, http.StatusNotFound, "Program not found"},
		{ErrConcurrentUpdate, http.StatusConflict, "Concurrent update detected"},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.code, tc.err.Code)
		assert.Equal(t, tc.message, tc.err.Message)
	}
}

func TestWithMessageAndWithError_DontMutateOriginal(t *testing.T) {
	custom := ErrNotFound.WithMessage("User not found")
	assert.Equal(t, "User not found", custom.Message)
	assert.Equal(t, ErrNotFound.Code, custom.Code)

	inner := fmt.Errorf("email is required")
	custom2 := ErrValidation.WithError(inner)
	assert.Equal(t, inner, custom2.Err)
	assert.Equal(t, ErrValidation.Message, custom2.Message)

	// сентинелы - глобалы, они не должны были поменяться
	assert.Equal(t, "Resource not found", ErrNotFound.Message)
	assert.Nil(t, ErrValidation.Err)
}

func TestWithMessage_WithError_Chaining(t *testing.T) {
	inner := fmt.Errorf("db error")
	custom := ErrNotFound.WithMessage("user not found").WithError(inner)

	assert.Equal(t, "user not found", custom.Message)
	assert.Equal(t, http.StatusNotFound, custom.Code)
	assert.Equal(t, inner, custom.Err)
	assert.Contains(t, custom.Error(), "db error")
}

func TestIsAppError(t *testing.T) {
	assert.True(t, IsAppError(ErrNotFound))
	assert.True(t, IsAppError(fmt.Errorf("wrapped: %w", ErrNotFound.WithMessage("x"))))
	assert.False(t, IsAppError(fmt.Errorf("regular error")))
	assert.False(t, IsAppError(nil))
}

func TestGetAppError(t *testing.T) {
	wrapped := fmt.Errorf("wrapped: %w", ErrForbidden.WithMessage("access denied"))
	res := GetAppError(wrapped)
	require.NotNil(t, res)
	assert.Equal(t, http.StatusForbidden, res.Code)
	assert.Equal(t, "access denied", res.Message)

	assert.Nil(t, GetAppError(fmt.Errorf("regular error")))
	assert.Nil(t, GetAppError(nil))
}

func TestToAppError(t *testing.T) {
	// уже наша ошибка - отдаём как есть
	appErr := ErrValidation.WithMessage("custom message")
	assert.Equal(t, appErr.Code, ToAppError(appErr).Code)

	// обёрнутая - достаётся из цепочки
	wrapped := ToAppError(fmt.Errorf("context: %w", ErrNotFound))
	assert.Equal(t, http.StatusNotFound, wrapped.Code)

	// чужая ошибка - заворачивается в 500
	other := ToAppError(fmt.Errorf("database connection failed"))
	require.NotNil(t, other)
	assert.Equal(t, http.StatusInternalServerError, other.Code)
	assert.Contains(t, other.Error(), "database connection failed")

	assert.Nil(t, ToAppError(nil))
}

func TestIsNotFound(t *testing.T) {
	assert.True(t, IsNotFound(ErrNotFound))
	assert.True(t, IsNotFound(fmt.Errorf("context: %w", ErrNotFound)))
	assert.True(t, IsNotFound(ErrProgramNotFound)) // тоже 404
	assert.False(t, IsNotFound(ErrForbidden))
	assert.False(t, IsNotFound(fmt.Errorf("regular error")))
	assert.False(t, IsNotFound(nil))
}
