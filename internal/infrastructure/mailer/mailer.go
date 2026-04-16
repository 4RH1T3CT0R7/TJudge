// Package mailer содержит реализации auth.Mailer для отправки email-ов.
//
// P1.11: реальный SMTP-sender добавлен как опциональный; по-умолчанию
// используется LogMailer, который просто пишет reset-ссылку в лог.
// Это позволяет развернуть password reset без SMTP-инфраструктуры, а
// оператор может посмотреть в логе и скопировать ссылку вручную.
package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// LogMailer печатает reset-ссылку в лог. Используется в dev / без-SMTP.
type LogMailer struct {
	log *logger.Logger
}

// NewLogMailer создаёт LogMailer.
func NewLogMailer(log *logger.Logger) *LogMailer {
	return &LogMailer{log: log}
}

// SendPasswordResetEmail пишет reset-URL в info-лог.
func (m *LogMailer) SendPasswordResetEmail(_ context.Context, email, name, resetURL string) error {
	m.log.Info("Password reset email (log-only — no SMTP configured)",
		zap.String("to", email),
		zap.String("name", name),
		zap.String("reset_url", resetURL),
	)
	return nil
}

// SMTPConfig — конфиг SMTP-отправителя (заполняется из env).
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	UseTLS   bool
}

// Enabled возвращает true если настройки присутствуют.
func (c SMTPConfig) Enabled() bool { return c.Host != "" && c.From != "" }

// SMTPMailer отправляет email через SMTP (STARTTLS или обычный).
type SMTPMailer struct {
	cfg SMTPConfig
	log *logger.Logger
}

// NewSMTPMailer создаёт SMTP-mailer.
func NewSMTPMailer(cfg SMTPConfig, log *logger.Logger) *SMTPMailer {
	return &SMTPMailer{cfg: cfg, log: log}
}

// SendPasswordResetEmail отправляет письмо со ссылкой восстановления.
func (m *SMTPMailer) SendPasswordResetEmail(_ context.Context, email, name, resetURL string) error {
	subject := "TJudge — восстановление пароля"
	body := fmt.Sprintf(
		"Здравствуйте, %s!\r\n\r\nДля сброса пароля перейдите по ссылке (действительна 1 час):\r\n%s\r\n\r\nЕсли вы не запрашивали восстановление, просто проигнорируйте это письмо.\r\n",
		name, resetURL,
	)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		m.cfg.From, email, subject, body))

	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	var auth smtp.Auth
	if m.cfg.User != "" {
		auth = smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)
	}

	if m.cfg.UseTLS {
		// Implicit TLS (порт 465).
		tlsCfg := &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("smtp tls dial: %w", err)
		}
		defer conn.Close()
		c, err := smtp.NewClient(conn, m.cfg.Host)
		if err != nil {
			return fmt.Errorf("smtp new client: %w", err)
		}
		defer func() { _ = c.Quit() }()
		if auth != nil {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
		if err := c.Mail(m.cfg.From); err != nil {
			return err
		}
		if err := c.Rcpt(email); err != nil {
			return err
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write(msg); err != nil {
			return err
		}
		return w.Close()
	}

	// STARTTLS / plain (порт 587/25).
	return smtp.SendMail(addr, auth, m.cfg.From, []string{email}, msg)
}
