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

	// Should be unwrappable
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
		{"ErrMissingField", ErrMissingField, http.StatusBadRequest, "Missing required field"},
		{"ErrNotFound", ErrNotFound, http.StatusNotFound, "Resource not found"},
		{"ErrAlreadyExists", ErrAlreadyExists, http.StatusConflict, "Resource already exists"},
		{"ErrConflict", ErrConflict, http.StatusConflict, "Conflict"},
		{"ErrForbidden", ErrForbidden, http.StatusForbidden, "Forbidden"},
		{"ErrPermissionDenied", ErrPermissionDenied, http.StatusForbidden, "Permission denied"},
		{"ErrRateLimitExceeded", ErrRateLimitExceeded, http.StatusTooManyRequests, "Rate limit exceeded"},
		{"ErrInternal", ErrInternal, http.StatusInternalServerError, "Internal server error"},
		{"ErrServiceUnavailable", ErrServiceUnavailable, http.StatusServiceUnavailable, "Service unavailable"},
		{"ErrTimeout", ErrTimeout, http.StatusGatewayTimeout, "Request timeout"},
		{"ErrTournamentFull", ErrTournamentFull, http.StatusConflict, "Tournament is full"},
		{"ErrTournamentStarted", ErrTournamentStarted, http.StatusConflict, "Tournament already started"},
		{"ErrTournamentNotStarted", ErrTournamentNotStarted, http.StatusConflict, "Tournament not started yet"},
		{"ErrInvalidGameType", ErrInvalidGameType, http.StatusBadRequest, "Invalid game type"},
		{"ErrMatchInProgress", ErrMatchInProgress, http.StatusConflict, "Match is already in progress"},
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

	// Original should be unchanged
	assert.Equal(t, "Resource not found", original.Message)
}

func TestAppError_WithError(t *testing.T) {
	original := ErrValidation
	innerErr := fmt.Errorf("email is required")
	custom := original.WithError(innerErr)

	assert.Equal(t, innerErr, custom.Err)
	assert.Equal(t, original.Code, custom.Code)
	assert.Equal(t, original.Message, custom.Message)

	// Original should be unchanged
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
	// Create a chain of errors
	original := fmt.Errorf("original error")
	appErr := ErrValidation.WithError(original)
	wrapped := fmt.Errorf("context: %w", appErr)

	// errors.Is should work through the chain
	assert.True(t, errors.Is(wrapped, original))

	// GetAppError should find the AppError
	result := GetAppError(wrapped)
	require.NotNil(t, result)
	assert.Equal(t, http.StatusBadRequest, result.Code)
}

func TestAppError_Immutability(t *testing.T) {
	// Verify that With* methods don't modify the original
	original := ErrNotFound

	_ = original.WithMessage("custom")
	_ = original.WithError(fmt.Errorf("inner"))

	// Original should be unchanged
	assert.Equal(t, "Resource not found", original.Message)
	assert.Nil(t, original.Err)
}
