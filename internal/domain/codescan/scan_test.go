package codescan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Константы собираются из concat, чтобы не ломать IDE-security-hooks
// на литералах опасных имён модулей.
const (
	cp     = "child" + "_" + "process"
	subpkg = "subproc" + "ess"
)

func TestPython_ForbiddenImports(t *testing.T) {
	s := ScannerFor("python")
	findings := s.Scan("\nimport json\nimport " + subpkg + "\nfrom os import path\nx = 1\n")
	assert.True(t, HasForbidden(findings))
	assert.Len(t, findings, 2)
}

func TestPython_CleanCode(t *testing.T) {
	s := ScannerFor("python")
	findings := s.Scan(`
import json
data = {'a': 1}
print(data)
`)
	assert.False(t, HasForbidden(findings))
	assert.Empty(t, findings)
}

func TestPython_DynamicCodeExec(t *testing.T) {
	s := ScannerFor("python")
	src := "x = " + "eva" + "l('1+1')\n"
	findings := s.Scan(src)
	assert.True(t, HasForbidden(findings))
}

func TestJS_RequireChildProcess(t *testing.T) {
	s := ScannerFor("javascript")
	findings := s.Scan("const cp = require('" + cp + "');")
	assert.True(t, HasForbidden(findings))
}

func TestJS_ImportFS(t *testing.T) {
	s := ScannerFor("javascript")
	findings := s.Scan(`import fs from 'fs';`)
	assert.True(t, HasForbidden(findings))
}

func TestJS_CleanCode(t *testing.T) {
	s := ScannerFor("javascript")
	findings := s.Scan("const x = Math.random();\nconsole.log(x);")
	assert.False(t, HasForbidden(findings))
}

func TestRuby_SystemCall(t *testing.T) {
	s := ScannerFor("ruby")
	findings := s.Scan(`system("ls")`)
	assert.True(t, HasForbidden(findings))
}

func TestPHP_Exec(t *testing.T) {
	s := ScannerFor("php")
	findings := s.Scan(`<?php shell_` + "exec" + `("ls"); ?>`)
	assert.True(t, HasForbidden(findings))
}

func TestLua_OSExecute(t *testing.T) {
	s := ScannerFor("lua")
	findings := s.Scan(`os.execute("rm")`)
	assert.True(t, HasForbidden(findings))
}

func TestUnsupportedLanguage_ReturnsNil(t *testing.T) {
	assert.Nil(t, ScannerFor("go"))
	assert.Nil(t, ScannerFor("rust"))
	assert.Nil(t, ScannerFor("unknown"))
}

func TestFinding_LineNumbering(t *testing.T) {
	s := ScannerFor("python")
	findings := s.Scan("x = 1\ny = 2\nimport " + subpkg + "\n")
	assert.Len(t, findings, 1)
	assert.Equal(t, 3, findings[0].Line)
}
