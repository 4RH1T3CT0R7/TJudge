package validator

import (
	"strings"
	"testing"
)

func FuzzValidateUsername(f *testing.F) {
	// Начальный корпус: корректные, пустые, граничные, спецсимволы
	f.Add("validuser")
	f.Add("")
	f.Add("a")
	f.Add("ab")
	f.Add("abc")
	f.Add(strings.Repeat("x", 50))
	f.Add(strings.Repeat("x", 51))
	f.Add(strings.Repeat("x", 1000))
	f.Add("user@name")
	f.Add("user name")
	f.Add("user\x00name")
	f.Add("user\nname")
	f.Add("user\tname")
	f.Add("___")
	f.Add("---")
	f.Add("a-b_c")
	f.Add("123")
	f.Add("ALLCAPS")
	f.Add("MiXeD_CaSe-123")
	f.Add("юникод")
	f.Add("\xff\xfe\xfd")

	f.Fuzz(func(t *testing.T, input string) {
		// Не должен паниковать, только вернуть nil или ошибку
		_ = ValidateUsername(input)
	})
}

func FuzzValidateEmail(f *testing.F) {
	// Начальный корпус: корректные email, некорректные форматы, граничные случаи
	f.Add("user@example.com")
	f.Add("test.user@domain.org")
	f.Add("a+b@c.de")
	f.Add("user%tag@host.co.uk")
	f.Add("")
	f.Add("@")
	f.Add("@domain.com")
	f.Add("user@")
	f.Add("user@.com")
	f.Add("user@domain.")
	f.Add("user@@domain.com")
	f.Add("user @domain.com")
	f.Add("user@domain .com")
	f.Add(strings.Repeat("a", 255) + "@example.com")
	f.Add(strings.Repeat("a", 1000))
	f.Add("user\x00@example.com")
	f.Add("user\n@example.com")
	f.Add("user@\x00.com")
	f.Add(".user@domain.com")
	f.Add("user.@domain.com")
	f.Add("user..name@domain.com")
	f.Add("very.long.email.address.with.many.dots@sub.domain.example.com")
	f.Add("\xff\xfe@example.com")
	f.Add("user@юникод.com")

	f.Fuzz(func(t *testing.T, input string) {
		// Не должен паниковать, только вернуть nil или ошибку
		_ = ValidateEmail(input)
	})
}

func FuzzValidatePassword(f *testing.F) {
	// Начальный корпус: корректные, слишком короткие, слишком длинные, отсутствующие классы символов
	f.Add("ValidPass1")
	f.Add("Str0ngP@ss!")
	f.Add("")
	f.Add("a")
	f.Add("abcdefgh")
	f.Add("ABCDEFGH")
	f.Add("12345678")
	f.Add("abcdEFGH")
	f.Add("abcd1234")
	f.Add("ABCD1234")
	f.Add("aB1")
	f.Add("aB1cD2eF")
	f.Add(strings.Repeat("aB1", 50))
	f.Add(strings.Repeat("x", 128))
	f.Add(strings.Repeat("x", 129))
	f.Add(strings.Repeat("x", 1000))
	f.Add("pass\x00word")
	f.Add("pass\nWord1")
	f.Add("Пароль123")
	f.Add("\xff\xfe\xfdAa1bbbb")
	f.Add("Aa1!@#$%^&*()")

	f.Fuzz(func(t *testing.T, input string) {
		// Не должен паниковать, только вернуть nil или ошибку
		_ = ValidatePassword(input)
	})
}

func FuzzValidateLength(f *testing.F) {
	// Начальный корпус: различные строки, проверяемые с фиксированными min=1, max=255
	f.Add("")
	f.Add("a")
	f.Add("hello world")
	f.Add(strings.Repeat("x", 255))
	f.Add(strings.Repeat("x", 256))
	f.Add(strings.Repeat("x", 1000))
	f.Add("\x00")
	f.Add("\n\t\r")
	f.Add("юникод строка")
	f.Add("\xff\xfe\xfd")

	f.Fuzz(func(t *testing.T, input string) {
		// Проверка с фиксированными границами min/max
		_ = ValidateLength("field", input, 1, 255)
		// Также с нулевым max (без верхнего ограничения)
		_ = ValidateLength("field", input, 0, 0)
		// С min=0 (всегда проходит проверку min)
		_ = ValidateLength("field", input, 0, 100)
	})
}

func FuzzValidateRequired(f *testing.F) {
	f.Add("")
	f.Add("value")
	f.Add(" ")
	f.Add("\t")
	f.Add("\n")
	f.Add("\x00")
	f.Add(strings.Repeat("x", 10000))
	f.Add("юникод")

	f.Fuzz(func(t *testing.T, input string) {
		// Не должен паниковать, только вернуть nil или ошибку
		_ = ValidateRequired("field", input)
	})
}

func FuzzValidateEnum(f *testing.F) {
	f.Add("")
	f.Add("active")
	f.Add("inactive")
	f.Add("unknown")
	f.Add("ACTIVE")
	f.Add("\x00")
	f.Add(strings.Repeat("x", 1000))
	f.Add("юникод")

	allowed := []string{"active", "inactive", "pending"}

	f.Fuzz(func(t *testing.T, input string) {
		// Не должен паниковать, только вернуть nil или ошибку
		_ = ValidateEnum("status", input, allowed)
		// Также с пустым списком допустимых значений
		_ = ValidateEnum("status", input, []string{})
		// С nil в качестве списка допустимых значений
		_ = ValidateEnum("status", input, nil)
	})
}

func FuzzValidateRange(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(-1)
	f.Add(100)
	f.Add(-100)
	f.Add(2147483647)  // MaxInt32
	f.Add(-2147483648) // MinInt32

	f.Fuzz(func(t *testing.T, input int) {
		// Проверка с фиксированными границами min/max
		_ = ValidateRange("field", input, 1, 100)
		// С нулевым max (без верхнего ограничения)
		_ = ValidateRange("field", input, 0, 0)
		// С отрицательным min
		_ = ValidateRange("field", input, -10, 10)
	})
}
