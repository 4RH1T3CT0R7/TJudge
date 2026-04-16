package mailer

import (
	"context"
	"testing"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/stretchr/testify/assert"
)

// TestLogMailer_DoesNotError — LogMailer только логирует и не шлёт реально,
// поэтому SendPasswordResetEmail всегда возвращает nil.
func TestLogMailer_DoesNotError(t *testing.T) {
	log, _ := logger.New("error", "json")
	m := NewLogMailer(log)
	err := m.SendPasswordResetEmail(context.Background(),
		"user@example.com", "User", "https://tjudge.example/reset?token=abc")
	assert.NoError(t, err)
}

func TestSMTPConfig_EnabledFlag(t *testing.T) {
	assert.False(t, SMTPConfig{}.Enabled(), "пустой конфиг не enabled")
	assert.False(t, SMTPConfig{Host: "smtp.example.com"}.Enabled(), "без From не enabled")
	assert.False(t, SMTPConfig{From: "a@b.c"}.Enabled(), "без Host не enabled")
	assert.True(t, SMTPConfig{Host: "smtp.example.com", From: "a@b.c"}.Enabled())
}

// TestSMTPMailer_ConstructsCorrectly — smoke test, что конструктор не паникует.
func TestSMTPMailer_ConstructsCorrectly(t *testing.T) {
	log, _ := logger.New("error", "json")
	m := NewSMTPMailer(SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
		From: "noreply@example.com",
	}, log)
	assert.NotNil(t, m)
	assert.Equal(t, "smtp.example.com", m.cfg.Host)
}
