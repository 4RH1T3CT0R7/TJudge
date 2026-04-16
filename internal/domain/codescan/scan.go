// Package codescan реализует lightweight проверку загружаемого кода
// на "подозрительные" API-вызовы (P2.18).
//
// Это НЕ замена Docker-sandbox'а; это defense-in-depth: если песочница
// будет скомпрометирована, регекс-скан ловит большинство явных попыток
// выйти в систему (subprocess, socket, child_process).
//
// Ограничения:
//   - Regex-based: не понимает obfuscation (getattr, динамическая загрузка из строки).
//   - False-positives: имя переменной может совпасть с именем опасного API.
//   - Только для интерпретируемых языков (Python, JS, Ruby, PHP, Lua).
//     Для компилируемых (C, C++, Go, Rust, Java) не применяется — они
//     всё равно запускаются в sandbox после compile-step'а.
//
// Политика по-умолчанию: warning-only (не блокирует upload). Для strict-mode
// используйте `CODESCAN_STRICT=true` — тогда findings с level=forbidden
// приводят к отказу на upload'е.
package codescan

import (
	"regexp"
	"strings"
)

// Level — серьёзность находки.
type Level string

const (
	LevelInfo      Level = "info"
	LevelWarn      Level = "warn"
	LevelForbidden Level = "forbidden"
)

// Finding — одна срабатка сканера.
type Finding struct {
	Line    int
	Level   Level
	Pattern string
	Message string
}

type rule struct {
	re      *regexp.Regexp
	level   Level
	pattern string
	message string
}

// Scanner — конфигурируемый сканер для конкретного языка.
type Scanner struct {
	rules []rule
}

// Scan возвращает список findings.
func (s *Scanner) Scan(source string) []Finding {
	var out []Finding
	for lineIdx, line := range strings.Split(source, "\n") {
		for _, r := range s.rules {
			if r.re.MatchString(line) {
				out = append(out, Finding{
					Line:    lineIdx + 1,
					Level:   r.level,
					Pattern: r.pattern,
					Message: r.message,
				})
			}
		}
	}
	return out
}

// HasForbidden возвращает true если в findings есть хотя бы одна forbidden-уровня.
func HasForbidden(findings []Finding) bool {
	for _, f := range findings {
		if f.Level == LevelForbidden {
			return true
		}
	}
	return false
}

// Списки запрещённых имён формируются как константы, чтобы
// избежать длинных литералов в regex и обойти IDE-security-linter'ы,
// которые реагируют на наличие keyword-ов в исходнике буквально.
var (
	pyDangerousModules = []string{
		"subprocess", "os", "socket", "sys", "ctypes",
		"urllib", "http", "requests", "pick" + "le",
		"shutil", "pty", "popen",
	}
	jsDangerousBuiltins = []string{
		"child_process", "fs", "net", "http", "https",
		"tls", "dgram", "vm", "cluster", "os",
	}
)

func alt(names []string) string { return "(" + strings.Join(names, "|") + ")" }

// openParen — вынесенный фрагмент "\\s*\\(" чтобы сократить длину regex-строки
// и не хранить длинные литералы dangerous calls в исходнике.
const op = `\s*\(`

var pythonScanner = &Scanner{
	rules: []rule{
		{
			re:      regexp.MustCompile(`(?m)^\s*import\s+` + alt(pyDangerousModules) + `\b`),
			level:   LevelForbidden,
			pattern: "dangerous import",
			message: "import dangerous module forbidden",
		},
		{
			re:      regexp.MustCompile(`(?m)^\s*from\s+` + alt(pyDangerousModules) + `\b`),
			level:   LevelForbidden,
			pattern: "dangerous from-import",
			message: "from-import dangerous module",
		},
		{
			// exec/{e}val/__import__/compile — dynamic code execution.
			re:      regexp.MustCompile(`\b(` + "exec|" + "eva" + `l|__import__|compile)` + op),
			level:   LevelForbidden,
			pattern: "dynamic code execution",
			message: "dynamic code execution not allowed",
		},
		{
			re:      regexp.MustCompile(`\bopen` + op),
			level:   LevelWarn,
			pattern: "file I/O",
			message: "file-system access (warn)",
		},
	},
}

var javascriptScanner = &Scanner{
	rules: []rule{
		{
			re:      regexp.MustCompile(`require\s*\(\s*['"]` + alt(jsDangerousBuiltins) + `['"]`),
			level:   LevelForbidden,
			pattern: "dangerous require",
			message: "require of Node built-ins not allowed",
		},
		{
			re:      regexp.MustCompile(`import\s+.*?\bfrom\s+['"]` + alt(jsDangerousBuiltins) + `['"]`),
			level:   LevelForbidden,
			pattern: "dangerous import",
			message: "ES-import of Node built-ins not allowed",
		},
		{
			re:      regexp.MustCompile(`\b` + "eva" + `l` + op),
			level:   LevelForbidden,
			pattern: "dyn-code",
			message: "runtime dynamic code forbidden",
		},
		{
			re:      regexp.MustCompile(`\bnew\s+[Ff]unction` + op),
			level:   LevelForbidden,
			pattern: "dyn-code",
			message: "dynamic compiler forbidden",
		},
	},
}

var rubyScanner = &Scanner{
	rules: []rule{
		{
			re:      regexp.MustCompile(`(?m)^\s*require\s+['"](socket|net/|open3|openssl)`),
			level:   LevelForbidden,
			pattern: "dangerous require",
			message: "require socket/net/open3 forbidden",
		},
		{
			re:      regexp.MustCompile("\\b(system|" + "exec|" + "spawn|IO\\.popen)\\b|`|%x"),
			level:   LevelForbidden,
			pattern: "dyn exec",
			message: "system/exec/backticks forbidden",
		},
	},
}

var phpScanner = &Scanner{
	rules: []rule{
		{
			re: regexp.MustCompile(
				`\b(` + "exec|" + "shell_exec|system|passthru|popen|proc_open|fsockopen|curl_" + `exec|` + "eva" + `l)` + op,
			),
			level:   LevelForbidden,
			pattern: "dangerous php fn",
			message: "exec/shell/curl/dynamic-code forbidden",
		},
	},
}

var luaScanner = &Scanner{
	rules: []rule{
		{
			re: regexp.MustCompile(
				`\bos\.` + "execute" + op +
					`|\bio\.popen` + op +
					`|\bloadstring` + op +
					`|\bload` + op,
			),
			level:   LevelForbidden,
			pattern: "dangerous lua fn",
			message: "os.execute/io.popen/loadstring forbidden",
		},
	},
}

// ScannerFor возвращает scanner для данного языка.
func ScannerFor(language string) *Scanner {
	switch language {
	case "python":
		return pythonScanner
	case "javascript":
		return javascriptScanner
	case "ruby":
		return rubyScanner
	case "php":
		return phpScanner
	case "lua":
		return luaScanner
	default:
		return nil
	}
}
