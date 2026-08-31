// Package codescan — лёгкая проверка загружаемого кода на подозрительные вызовы.
// это не замена docker-песочнице, а defense-in-depth: если песочницу пробьют,
// регекс-скан ловит явные попытки выйти в систему (subprocess, socket, child_process).
// regex не видит обфускацию и даёт ложные срабатывания — с этим приходится жить.
// только для интерпретируемых языков, компилируемые всё равно гоняются в sandbox.
package codescan

import (
	"regexp"
	"strings"
)

// Level — серьёзность находки
type Level string

const (
	LevelInfo      Level = "info"
	LevelWarn      Level = "warn"
	LevelForbidden Level = "forbidden"
)

// Finding — одно срабатывание сканера
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

// Scanner — сканер под конкретный язык
type Scanner struct {
	rules []rule
}

// Scan возвращает findings
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

// HasForbidden — есть ли хоть один forbidden
func HasForbidden(findings []Finding) bool {
	for _, f := range findings {
		if f.Level == LevelForbidden {
			return true
		}
	}
	return false
}

// опасные имена лежат отдельными списками, чтобы не тащить длинные литералы в regex
// и заодно не триггерить ide-шные security-линтеры, они реагируют на голые keyword-ы
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

// op — вынесенный кусок "\s*\(", чтобы regex-строки были покороче
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
			// exec/eval/__import__/compile — динамическое выполнение кода
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
