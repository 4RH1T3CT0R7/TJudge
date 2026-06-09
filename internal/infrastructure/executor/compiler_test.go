package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJavaClassNameRegex блокирует shell-метасимволы в Java class name
// (защита от инъекций в wrapper-скрипт).
func TestJavaClassNameRegex(t *testing.T) {
	okNames := []string{"Main", "Solution1", "_Hidden", "$Dollar", "A1_b2"}
	for _, n := range okNames {
		assert.True(t, javaClassNameRe.MatchString(n), "valid identifier rejected: %q", n)
	}

	badNames := []string{
		"Main;rm -rf /",
		"A|B",
		"A&B",
		"A`cmd`",
		"A$(cmd)",
		"A B",
		"A\nB",
		"1Leading",
		"",
		"../etc",
	}
	for _, n := range badNames {
		assert.False(t, javaClassNameRe.MatchString(n), "unsafe name accepted: %q", n)
	}
}

// TestExtractJavaClassName проверяет извлечение declared class name из Java-source.
func TestExtractJavaClassName(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"simple public class", `public class Main { }`, "Main"},
		{"with package", "package foo;\npublic class Solution {}", "Solution"},
		{"non-public class", "class Helper { }", "Helper"},
		{"with line comment above", "// my file\nclass Foo {}", "Foo"},
		{"with block comment", "/* hdr */\npublic class Bar {}", "Bar"},
		{"final class", "final class Fin {}", "Fin"},
		{"abstract class", "abstract class Abs {}", "Abs"},
		{"class keyword in comment does not match", "// class Ghost\npublic class Real {}", "Real"},
		{"no class declaration", "public interface I {}", ""},
		{"empty", "", ""},
		{"multi-class: takes first", "class A {}\nclass B {}", "A"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, extractJavaClassName(c.src))
		})
	}
}

func TestBuildCompilePlan_CompiledLanguages(t *testing.T) {
	cases := []struct {
		language string
		source   string
		artifact string
		first    string
	}{
		{"c", "main.c", "out", "gcc"},
		{"cpp", "main.cpp", "out", "g++"},
		{"go", "main.go", "out", "go"},
		{"rust", "main.rs", "out", "rustc"},
	}
	for _, c := range cases {
		t.Run(c.language, func(t *testing.T) {
			plan, err := buildCompilePlan(c.language, "")
			require.NoError(t, err)
			assert.Equal(t, c.source, plan.SourceName)
			assert.Equal(t, c.artifact, plan.ArtifactName)
			assert.Equal(t, c.first, plan.Cmd[0])
			// Все пути в команде должны лежать внутри /build.
			for _, arg := range plan.Cmd[1:] {
				if arg[0] == '/' {
					assert.Contains(t, arg, buildContainerPath+"/")
				}
			}
		})
	}
}

func TestBuildCompilePlan_InterpretedLanguages(t *testing.T) {
	// Интерпретируемые языки: только проверка синтаксиса, артефакта нет.
	for lang, first := range map[string]string{
		"python":     "python3",
		"javascript": "node",
		"ruby":       "ruby",
		"php":        "php",
		"lua":        "luac",
	} {
		t.Run(lang, func(t *testing.T) {
			plan, err := buildCompilePlan(lang, "")
			require.NoError(t, err)
			assert.Empty(t, plan.ArtifactName)
			assert.Equal(t, first, plan.Cmd[0])
		})
	}
}

func TestBuildCompilePlan_Java(t *testing.T) {
	plan, err := buildCompilePlan("java", "Main")
	require.NoError(t, err)
	assert.Equal(t, "Main.java", plan.SourceName)
	assert.Equal(t, "Main.class", plan.ArtifactName)
	assert.Equal(t, []string{"javac", "/build/Main.java"}, plan.Cmd)

	// Без class-декларации - ошибка программы (не инфраструктуры).
	_, err = buildCompilePlan("java", "")
	assert.Error(t, err)

	// Невалидное имя класса блокируется allowlist-регэкспом.
	_, err = buildCompilePlan("java", "Evil;rm -rf /")
	assert.Error(t, err)
}

func TestBuildCompilePlan_UnsupportedLanguage(t *testing.T) {
	_, err := buildCompilePlan("brainfuck", "")
	assert.Error(t, err)
}

func TestStripDockerLogHeaders(t *testing.T) {
	// Фрейм: [stream(1) 0 0 0 size(4)] payload
	frame := append([]byte{1, 0, 0, 0, 0, 0, 0, 5}, []byte("hello")...)
	frame = append(frame, append([]byte{2, 0, 0, 0, 0, 0, 0, 6}, []byte(" world")...)...)
	assert.Equal(t, "hello world", stripDockerLogHeaders(frame))

	// Пустой и неполный ввод не паникуют.
	assert.Equal(t, "", stripDockerLogHeaders(nil))
	assert.Equal(t, "", stripDockerLogHeaders([]byte{1, 0, 0}))
}
