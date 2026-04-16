package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockAuditReader struct{ mock.Mock }

func (m *mockAuditReader) List(ctx context.Context, limit int) ([]*domain.AuditLogEntry, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLogEntry), args.Error(1)
}

func newAuditHandlerFixture(t *testing.T) (*AuditHandler, *mockAuditReader) {
	t.Helper()
	log, _ := logger.New("error", "json")
	repo := new(mockAuditReader)
	return NewAuditHandler(repo, log), repo
}

func TestAuditHandler_List_DefaultLimit(t *testing.T) {
	h, repo := newAuditHandlerFixture(t)
	entry := &domain.AuditLogEntry{ID: uuid.New(), Action: "POST /tournaments", CreatedAt: time.Now()}
	repo.On("List", mock.Anything, 100).Return([]*domain.AuditLogEntry{entry}, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var envelope struct {
		Data []domain.AuditLogEntry `json:"data"`
	}
	assert.NoError(t, json.NewDecoder(rec.Body).Decode(&envelope))
	assert.Len(t, envelope.Data, 1)
	assert.Equal(t, "POST /tournaments", envelope.Data[0].Action)
}

func TestAuditHandler_List_CustomLimit(t *testing.T) {
	h, repo := newAuditHandlerFixture(t)
	repo.On("List", mock.Anything, 50).Return([]*domain.AuditLogEntry{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit?limit=50", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	repo.AssertCalled(t, "List", mock.Anything, 50)
}

func TestAuditHandler_List_CapsLimitAt500(t *testing.T) {
	h, repo := newAuditHandlerFixture(t)
	repo.On("List", mock.Anything, 500).Return([]*domain.AuditLogEntry{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit?limit=99999", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	repo.AssertCalled(t, "List", mock.Anything, 500)
}

func TestAuditHandler_List_IgnoresInvalidLimit(t *testing.T) {
	h, repo := newAuditHandlerFixture(t)
	repo.On("List", mock.Anything, 100).Return([]*domain.AuditLogEntry{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit?limit=garbage", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	repo.AssertCalled(t, "List", mock.Anything, 100)
}

func TestAuditHandler_List_RepoError_500(t *testing.T) {
	h, repo := newAuditHandlerFixture(t)
	repo.On("List", mock.Anything, 100).Return(nil, errors.New("db down"))

	req := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
