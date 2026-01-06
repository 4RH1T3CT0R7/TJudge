package validator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "email",
		Message: "is required",
	}

	result := err.Error()

	assert.Equal(t, "email: is required", result)
}

func TestValidationErrors_Error_Empty(t *testing.T) {
	var errs ValidationErrors

	result := errs.Error()

	assert.Equal(t, "", result)
}

func TestValidationErrors_Error_Multiple(t *testing.T) {
	errs := ValidationErrors{
		{Field: "email", Message: "is required"},
		{Field: "password", Message: "too short"},
	}

	result := errs.Error()

	assert.Contains(t, result, "validation errors:")
	assert.Contains(t, result, "email: is required")
	assert.Contains(t, result, "password: too short")
}

func TestValidationErrors_HasErrors(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var errs ValidationErrors
		assert.False(t, errs.HasErrors())
	})

	t.Run("with errors", func(t *testing.T) {
		errs := ValidationErrors{
			{Field: "test", Message: "error"},
		}
		assert.True(t, errs.HasErrors())
	})
}

func TestValidationErrors_Add(t *testing.T) {
	var errs ValidationErrors

	errs.Add("email", "is required")
	errs.Add("password", "too short")

	require.Len(t, errs, 2)
	assert.Equal(t, "email", errs[0].Field)
	assert.Equal(t, "password", errs[1].Field)
}

func TestValidateEmail_Valid(t *testing.T) {
	validEmails := []string{
		"test@example.com",
		"user.name@domain.org",
		"user+tag@example.co.uk",
		"user123@test-domain.com",
		"a@b.io",
	}

	for _, email := range validEmails {
		t.Run(email, func(t *testing.T) {
			err := ValidateEmail(email)
			assert.NoError(t, err)
		})
	}
}

func TestValidateEmail_Empty(t *testing.T) {
	err := ValidateEmail("")

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "email", validationErr.Field)
	assert.Contains(t, validationErr.Message, "required")
}

func TestValidateEmail_TooLong(t *testing.T) {
	longEmail := strings.Repeat("a", 250) + "@b.com"

	err := ValidateEmail(longEmail)

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "too long")
}

func TestValidateEmail_Invalid(t *testing.T) {
	invalidEmails := []string{
		"notanemail",
		"missing@tld",
		"@nolocal.com",
		"spaces in@email.com",
		"missing.at.sign.com",
	}

	for _, email := range invalidEmails {
		t.Run(email, func(t *testing.T) {
			err := ValidateEmail(email)
			require.Error(t, err)
			var validationErr *ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Contains(t, validationErr.Message, "invalid email format")
		})
	}
}

func TestValidateUsername_Valid(t *testing.T) {
	validUsernames := []string{
		"user",
		"user123",
		"user_name",
		"user-name",
		"UserName",
		"abc",
		strings.Repeat("a", 50),
	}

	for _, username := range validUsernames {
		t.Run(username, func(t *testing.T) {
			err := ValidateUsername(username)
			assert.NoError(t, err)
		})
	}
}

func TestValidateUsername_Empty(t *testing.T) {
	err := ValidateUsername("")

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "required")
}

func TestValidateUsername_TooShort(t *testing.T) {
	err := ValidateUsername("ab")

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "at least 3 characters")
}

func TestValidateUsername_TooLong(t *testing.T) {
	err := ValidateUsername(strings.Repeat("a", 51))

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "too long")
}

func TestValidateUsername_InvalidCharacters(t *testing.T) {
	invalidUsernames := []string{
		"user name",    // space
		"user@name",    // special char
		"user.name",    // dot
		"пользователь", // cyrillic
	}

	for _, username := range invalidUsernames {
		t.Run(username, func(t *testing.T) {
			err := ValidateUsername(username)
			require.Error(t, err)
			var validationErr *ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Contains(t, validationErr.Message, "only contain")
		})
	}
}

func TestValidatePassword_Valid(t *testing.T) {
	validPasswords := []string{
		"Password1",
		"SecurePass123",
		"MyP@ssw0rd!",
		"Ab1" + strings.Repeat("x", 5),
	}

	for _, password := range validPasswords {
		t.Run(password, func(t *testing.T) {
			err := ValidatePassword(password)
			assert.NoError(t, err)
		})
	}
}

func TestValidatePassword_Empty(t *testing.T) {
	err := ValidatePassword("")

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "required")
}

func TestValidatePassword_TooShort(t *testing.T) {
	err := ValidatePassword("Pass1")

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "at least 8 characters")
}

func TestValidatePassword_TooLong(t *testing.T) {
	err := ValidatePassword(strings.Repeat("Aa1", 50))

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "too long")
}

func TestValidatePassword_NoUppercase(t *testing.T) {
	err := ValidatePassword("password123")

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "uppercase")
}

func TestValidatePassword_NoLowercase(t *testing.T) {
	err := ValidatePassword("PASSWORD123")

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "lowercase")
}

func TestValidatePassword_NoDigit(t *testing.T) {
	err := ValidatePassword("PasswordOnly")

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "digit")
}

func TestValidateRequired_Valid(t *testing.T) {
	err := ValidateRequired("name", "John")

	assert.NoError(t, err)
}

func TestValidateRequired_Empty(t *testing.T) {
	err := ValidateRequired("name", "")

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "name", validationErr.Field)
	assert.Contains(t, validationErr.Message, "required")
}

func TestValidateLength_Valid(t *testing.T) {
	tests := []struct {
		name  string
		value string
		min   int
		max   int
	}{
		{"exact min", "abc", 3, 10},
		{"exact max", "abcdefghij", 3, 10},
		{"in range", "abcde", 3, 10},
		{"no max", "abcde", 3, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLength("field", tc.value, tc.min, tc.max)
			assert.NoError(t, err)
		})
	}
}

func TestValidateLength_TooShort(t *testing.T) {
	err := ValidateLength("name", "ab", 3, 10)

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "at least 3 characters")
}

func TestValidateLength_TooLong(t *testing.T) {
	err := ValidateLength("name", "abcdefghijk", 3, 10)

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "too long")
}

func TestValidateRange_Valid(t *testing.T) {
	tests := []struct {
		name  string
		value int
		min   int
		max   int
	}{
		{"exact min", 1, 1, 10},
		{"exact max", 10, 1, 10},
		{"in range", 5, 1, 10},
		{"no max", 100, 1, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRange("field", tc.value, tc.min, tc.max)
			assert.NoError(t, err)
		})
	}
}

func TestValidateRange_TooSmall(t *testing.T) {
	err := ValidateRange("age", 0, 1, 100)

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "at least 1")
}

func TestValidateRange_TooLarge(t *testing.T) {
	err := ValidateRange("age", 101, 1, 100)

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "at most 100")
}

func TestValidateEnum_Valid(t *testing.T) {
	allowed := []string{"active", "inactive", "pending"}

	err := ValidateEnum("status", "active", allowed)

	assert.NoError(t, err)
}

func TestValidateEnum_Invalid(t *testing.T) {
	allowed := []string{"active", "inactive", "pending"}

	err := ValidateEnum("status", "unknown", allowed)

	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Message, "must be one of")
}

func TestValidateEnum_EmptyAllowed(t *testing.T) {
	err := ValidateEnum("status", "any", []string{})

	require.Error(t, err)
}

func BenchmarkValidateEmail(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ValidateEmail("test@example.com")
	}
}

func BenchmarkValidatePassword(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ValidatePassword("SecurePass123!")
	}
}

func BenchmarkValidateUsername(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ValidateUsername("testuser123")
	}
}
