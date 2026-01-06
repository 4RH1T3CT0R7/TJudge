package validator

import (
	"fmt"
	"regexp"
	"unicode"
)

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,50}$`)
)

// ValidationError представляет ошибку валидации
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors список ошибок валидации
type ValidationErrors []*ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	msg := "validation errors:"
	for _, err := range e {
		msg += fmt.Sprintf("\n  - %s", err.Error())
	}
	return msg
}

// HasErrors проверяет наличие ошибок
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Add добавляет ошибку валидации
func (e *ValidationErrors) Add(field, message string) {
	*e = append(*e, &ValidationError{
		Field:   field,
		Message: message,
	})
}

// ValidateEmail проверяет email
func ValidateEmail(email string) error {
	if email == "" {
		return &ValidationError{Field: "email", Message: "email is required"}
	}
	if len(email) > 255 {
		return &ValidationError{Field: "email", Message: "email is too long (max 255 characters)"}
	}
	if !emailRegex.MatchString(email) {
		return &ValidationError{Field: "email", Message: "invalid email format"}
	}
	return nil
}

// ValidateUsername проверяет username
func ValidateUsername(username string) error {
	if username == "" {
		return &ValidationError{Field: "username", Message: "username is required"}
	}
	if len(username) < 3 {
		return &ValidationError{Field: "username", Message: "username must be at least 3 characters"}
	}
	if len(username) > 50 {
		return &ValidationError{Field: "username", Message: "username is too long (max 50 characters)"}
	}
	if !usernameRegex.MatchString(username) {
		return &ValidationError{Field: "username", Message: "username can only contain letters, numbers, underscore and hyphen"}
	}
	return nil
}

// ValidatePassword проверяет пароль
func ValidatePassword(password string) error {
	if password == "" {
		return &ValidationError{Field: "password", Message: "password is required"}
	}
	if len(password) < 8 {
		return &ValidationError{Field: "password", Message: "password must be at least 8 characters"}
	}
	if len(password) > 128 {
		return &ValidationError{Field: "password", Message: "password is too long (max 128 characters)"}
	}

	// Проверка на наличие букв, цифр и спецсимволов
	var hasUpper, hasLower, hasDigit bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit {
		return &ValidationError{
			Field:   "password",
			Message: "password must contain uppercase, lowercase and digit",
		}
	}

	return nil
}

// ValidateRequired проверяет обязательное поле
func ValidateRequired(field, value string) error {
	if value == "" {
		return &ValidationError{Field: field, Message: fmt.Sprintf("%s is required", field)}
	}
	return nil
}

// ValidateLength проверяет длину строки
func ValidateLength(field, value string, min, max int) error {
	length := len(value)
	if length < min {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s must be at least %d characters", field, min),
		}
	}
	if max > 0 && length > max {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s is too long (max %d characters)", field, max),
		}
	}
	return nil
}

// ValidateRange проверяет числовой диапазон
func ValidateRange(field string, value, min, max int) error {
	if value < min {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s must be at least %d", field, min),
		}
	}
	if max > 0 && value > max {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s must be at most %d", field, max),
		}
	}
	return nil
}

// ValidateEnum проверяет значение из списка
func ValidateEnum(field, value string, allowedValues []string) error {
	for _, allowed := range allowedValues {
		if value == allowed {
			return nil
		}
	}
	return &ValidationError{
		Field:   field,
		Message: fmt.Sprintf("%s must be one of: %v", field, allowedValues),
	}
}
