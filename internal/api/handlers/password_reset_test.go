package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/domain/auth"
	apperrors "github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockPasswordResetService struct{ mock.Mock }

func (m *mockPasswordResetService) Request(ctx context.Context, req auth.PasswordResetRequest, ip string) error {
	return m.Called(ctx, req, ip).Error(0)
}
func (m *mockPasswordResetService) Confirm(ctx context.Context, req auth.PasswordResetConfirm) error {
	return m.Called(ctx, req).Error(0)
}

func newPWHandler(t *testing.T) (*PasswordResetHandler, *mockPasswordResetService) {
	t.Helper()
	log, _ := logger.New("error", "json")
	svc := new(mockPasswordResetService)
	return NewPasswordResetHandler(svc, log), svc
}

func TestPasswordResetHandler_Request_OK(t *testing.T) {
	h, svc := newPWHandler(t)
	svc.On("Request", mock.Anything, auth.PasswordResetRequest{Email: "u@e.com"}, mock.Anything).
		Return(nil)

	body, _ := json.Marshal(map[string]string{"email": "u@e.com"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.RemoteAddr = "1.2.3.4:55"
	rec := httptest.NewRecorder()
	h.Request(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPasswordResetHandler_Request_InvalidBody(t *testing.T) {
	h, _ := newPWHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("{not json")))
	rec := httptest.NewRecorder()
	h.Request(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPasswordResetHandler_Request_ServiceError(t *testing.T) {
	h, svc := newPWHandler(t)
	svc.On("Request", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("db down"))

	body, _ := json.Marshal(map[string]string{"email": "u@e.com"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Request(rec, req)
	// Generic-ошибка возвращает 500.
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestPasswordResetHandler_Confirm_OK(t *testing.T) {
	h, svc := newPWHandler(t)
	svc.On("Confirm", mock.Anything, auth.PasswordResetConfirm{Token: "t", NewPassword: "longpass"}).
		Return(nil)

	body, _ := json.Marshal(map[string]string{"token": "t", "new_password": "longpass"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Confirm(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPasswordResetHandler_Confirm_InvalidToken(t *testing.T) {
	h, svc := newPWHandler(t)
	svc.On("Confirm", mock.Anything, mock.Anything).
		Return(apperrors.ErrInvalidCredentials.WithMessage("bad token"))

	body, _ := json.Marshal(map[string]string{"token": "x", "new_password": "longpass"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Confirm(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestClientIPFromRequest_StripsPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:54321"
	assert.Equal(t, "192.168.1.1", clientIPFromRequest(req))
}

func TestClientIPFromRequest_NoPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "plain-host-no-port"
	assert.Equal(t, "plain-host-no-port", clientIPFromRequest(req))
}
