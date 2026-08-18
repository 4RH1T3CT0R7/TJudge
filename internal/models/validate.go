package models

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// TODO: вынести magic-числа лимитов в конфиг

var (
	// email regex помягче, чем rfc. получем большинство нормальных адресов:
	// локальная часть с буквами/цифрами/точками/подчёркиваниями/процентами/плюсами/дефисами,
	// домен с буквами/цифрами/дефисами, tld из 2+ букв, поддомены
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*\.[a-zA-Z]{2,}$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,50}$`)
)

// ValidationError - ошибка валидации одного поля
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors - пачка ошибок валидации
type ValidationErrors []*ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var msg strings.Builder
	msg.WriteString("validation errors:")
	for _, err := range e {
		fmt.Fprintf(&msg, "\n  - %s", err.Error())
	}
	return msg.String()
}

func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

func (e *ValidationErrors) Add(field, message string) {
	*e = append(*e, &ValidationError{
		Field:   field,
		Message: message,
	})
}

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

	// пароль должен содержать буквы разного регистра и цифру
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

func ValidateRequired(field, value string) error {
	if value == "" {
		return &ValidationError{Field: field, Message: fmt.Sprintf("%s is required", field)}
	}
	return nil
}

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

func ValidateEnum(field, value string, allowedValues []string) error {
	if slices.Contains(allowedValues, value) {
		return nil
	}
	return &ValidationError{
		Field:   field,
		Message: fmt.Sprintf("%s must be one of: %v", field, allowedValues),
	}
}
