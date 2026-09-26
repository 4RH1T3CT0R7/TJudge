package executor

import (
	"strings"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestExecutor(t *testing.T) *Executor {
	t.Helper()
	log, _ := logger.New("error", "json")
	return &Executor{
		config:           config.ExecutorConfig{},
		programsPath:     "/data/programs",
		hostProgramsPath: "/host/programs",
		containerPath:    "/programs",
		log:              log,
	}
}

// --- sanitizeForDB ---

func TestSanitizeForDB_NullBytes(t *testing.T) {
	result := sanitizeForDB("hello\x00world")
	assert.Equal(t, "helloworld", result)
}

func TestSanitizeForDB_Clean(t *testing.T) {
	result := sanitizeForDB("clean string")
	assert.Equal(t, "clean string", result)
}

func TestSanitizeForDB_Empty(t *testing.T) {
	result := sanitizeForDB("")
	assert.Equal(t, "", result)
}

func TestSanitizeForDB_MultipleNullBytes(t *testing.T) {
	result := sanitizeForDB("\x00start\x00middle\x00end\x00")
	assert.Equal(t, "startmiddleend", result)
}

// --- buildCommand ---

func TestBuildCommand_Basic(t *testing.T) {
	e := newTestExecutor(t)

	cmd := e.buildCommand("dilemma", "/programs/p1.py", "/programs/p2.py")

	assert.Equal(t, []string{"dilemma", "/programs/p1.py", "/programs/p2.py"}, cmd)
}

func TestBuildCommand_WithIterations(t *testing.T) {
	e := newTestExecutor(t)
	e.config.DefaultIterations = 100

	cmd := e.buildCommand("tug_of_war", "/programs/p1.py", "/programs/p2.py")

	assert.Equal(t, []string{"tug_of_war", "-i", "100", "/programs/p1.py", "/programs/p2.py"}, cmd)
}

func TestBuildCommand_WithVerbose(t *testing.T) {
	e := newTestExecutor(t)
	e.config.Verbose = true

	cmd := e.buildCommand("dilemma", "/programs/p1.py", "/programs/p2.py")

	assert.Equal(t, []string{"dilemma", "-v", "/programs/p1.py", "/programs/p2.py"}, cmd)
}

func TestBuildCommand_WithIterationsAndVerbose(t *testing.T) {
	e := newTestExecutor(t)
	e.config.DefaultIterations = 50
	e.config.Verbose = true

	cmd := e.buildCommand("dilemma", "/programs/p1.py", "/programs/p2.py")

	assert.Equal(t, []string{"dilemma", "-i", "50", "-v", "/programs/p1.py", "/programs/p2.py"}, cmd)
}

// --- hostToContainerPath ---

func TestHostToContainerPath_MatchingPrefix(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.hostToContainerPath("/data/programs/team1/game1/v1_solution.py")

	require.NoError(t, err)
	assert.Equal(t, "/programs/team1/game1/v1_solution.py", result)
}

func TestHostToContainerPath_NonMatchingPrefix(t *testing.T) {
	e := newTestExecutor(t)

	_, err := e.hostToContainerPath("/other/path/solution.py")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside programs directory")
}

// --- parseResult ---

func TestParseResult_ExitCode0_Player1Wins(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(0, "15 10", "")

	require.NoError(t, err)
	assert.Equal(t, 15, result.Score1)
	assert.Equal(t, 10, result.Score2)
	assert.Equal(t, 1, result.Winner)
	assert.Equal(t, 0, result.ErrorCode)
}

func TestParseResult_ExitCode0_Player2Wins(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(0, "10 15", "")

	require.NoError(t, err)
	assert.Equal(t, 10, result.Score1)
	assert.Equal(t, 15, result.Score2)
	assert.Equal(t, 2, result.Winner)
}

func TestParseResult_ExitCode0_Draw(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(0, "10 10", "")

	require.NoError(t, err)
	assert.Equal(t, 10, result.Score1)
	assert.Equal(t, 10, result.Score2)
	assert.Equal(t, 0, result.Winner) // ничья
}

func TestParseResult_ExitCode1_Program1Error(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(1, "", "runtime error")

	require.NoError(t, err)
	assert.Equal(t, 2, result.Winner) // побеждает программа 2
	assert.Equal(t, 1, result.ErrorCode)
	assert.NotEmpty(t, result.ErrorMessage)
}

func TestParseResult_ExitCode2_Program2Error(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(2, "", "timeout")

	require.NoError(t, err)
	assert.Equal(t, 1, result.Winner) // побеждает программа 1
	assert.Equal(t, 2, result.ErrorCode)
	assert.NotEmpty(t, result.ErrorMessage)
}

func TestParseResult_InvalidOutput_OneNumber(t *testing.T) {
	e := newTestExecutor(t)

	_, err := e.parseResult(0, "42", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected 2 scores")
}

func TestParseResult_InvalidOutput_NonNumeric(t *testing.T) {
	e := newTestExecutor(t)

	_, err := e.parseResult(0, "abc def", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid score1")
}

func TestParseResult_InvalidOutput_SecondNonNumeric(t *testing.T) {
	e := newTestExecutor(t)

	_, err := e.parseResult(0, "10 abc", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid score2")
}

func TestParseResult_InvalidOutput_Empty(t *testing.T) {
	e := newTestExecutor(t)

	_, err := e.parseResult(0, "", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected 2 scores")
}

// --- parseResult (дополнительно) ---

func TestParseResult_LargeScores(t *testing.T) {
	e := newTestExecutor(t)

	// очки в допустимых границах [0, 100000]
	result, err := e.parseResult(0, "99999 88888", "")

	require.NoError(t, err)
	assert.Equal(t, 99999, result.Score1)
	assert.Equal(t, 88888, result.Score2)
	assert.Equal(t, 1, result.Winner)
}

func TestParseResult_ScoresOutOfBounds(t *testing.T) {
	e := newTestExecutor(t)

	// конфигурация по умолчанию (0 итераций) даёт нижний порог 100_000
	_, err := e.parseResult(0, "999999 888888", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scores out of bounds")
}

func TestParseResult_HighIterationsAcceptsLargeScores(t *testing.T) {
	e := newTestExecutor(t)
	e.config.DefaultIterations = 500 // maxScore = 500*1000 = 500_000

	result, err := e.parseResult(0, "450000 300000", "")

	require.NoError(t, err)
	assert.Equal(t, 450000, result.Score1)
	assert.Equal(t, 300000, result.Score2)
	assert.Equal(t, 1, result.Winner)
}

func TestParseResult_HighIterationsStillRejectsExtremeScores(t *testing.T) {
	e := newTestExecutor(t)
	e.config.DefaultIterations = 500 // maxScore = 500_000

	_, err := e.parseResult(0, "999999 888888", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scores out of bounds")
}

func TestParseResult_NegativeScore(t *testing.T) {
	e := newTestExecutor(t)

	// небольшие отрицательные очки допустимы (например, проигравший в dollar_auction получает -bid)
	result, err := e.parseResult(0, "-1 50", "")

	require.NoError(t, err)
	assert.Equal(t, -1, result.Score1)
	assert.Equal(t, 50, result.Score2)
}

func TestParseResult_ExtremeNegativeScore(t *testing.T) {
	e := newTestExecutor(t)

	// экстремально отрицательные очки за пределами -maxScore отклоняются
	_, err := e.parseResult(0, "-999999 50", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scores out of bounds")
}

func TestParseResult_ZeroScores(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(0, "0 0", "")

	require.NoError(t, err)
	assert.Equal(t, 0, result.Score1)
	assert.Equal(t, 0, result.Score2)
	assert.Equal(t, 0, result.Winner) // ничья
}

func TestParseResult_WhitespaceOutput(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(0, "  15   10  \n", "")

	require.NoError(t, err)
	assert.Equal(t, 15, result.Score1)
	assert.Equal(t, 10, result.Score2)
	assert.Equal(t, 1, result.Winner)
}

func TestParseResult_ThreeNumbers(t *testing.T) {
	e := newTestExecutor(t)

	_, err := e.parseResult(0, "10 15 20", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected 2 scores")
}

func TestParseResult_ExitCodeGeneric(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(3, "", "unknown error")

	require.NoError(t, err)
	assert.Equal(t, 3, result.ErrorCode)
	assert.Contains(t, result.ErrorMessage, "код 3")
}

// сбой подготовки песочницы - окружение, а не программы: матч повторяется
func TestParseResult_SandboxSetupFailedIsInfra(t *testing.T) {
	e := newTestExecutor(t)

	_, err := e.parseResult(sandboxSetupFailed, "", "sandbox: line 47: command failed")

	require.Error(t, err)
	assert.True(t, IsInfraError(err))
	assert.Contains(t, err.Error(), "line 47")
}

func TestParseResult_ErrorWithStdoutAndStderr(t *testing.T) {
	e := newTestExecutor(t)

	// stdout должен быть длиннее 20 символов или без пробелов, чтобы пройти фильтр parseResult
	longStdout := "traceback in main function call"
	result, err := e.parseResult(1, longStdout, "error msg")

	require.NoError(t, err)
	assert.Equal(t, 2, result.Winner)
	assert.Equal(t, 1, result.ErrorCode)
	assert.Contains(t, result.ErrorMessage, "stderr")
	assert.Contains(t, result.ErrorMessage, "error msg")
	assert.Contains(t, result.ErrorMessage, "stdout")
	assert.Contains(t, result.ErrorMessage, longStdout)
}

func TestParseResult_NullBytesInError(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(1, "", "error\x00message")

	require.NoError(t, err)
	assert.Equal(t, 1, result.ErrorCode)
	assert.NotContains(t, result.ErrorMessage, "\x00")
	assert.Contains(t, result.ErrorMessage, "errormessage")
}

// --- buildCommand (дополнительно) ---

func TestBuildCommand_ZeroIterations(t *testing.T) {
	e := newTestExecutor(t)
	e.config.DefaultIterations = 0

	cmd := e.buildCommand("dilemma", "/programs/p1.py", "/programs/p2.py")

	assert.Equal(t, []string{"dilemma", "/programs/p1.py", "/programs/p2.py"}, cmd)
	assert.NotContains(t, cmd, "-i")
}

func TestBuildCommand_NegativeIterations(t *testing.T) {
	e := newTestExecutor(t)
	e.config.DefaultIterations = -1

	cmd := e.buildCommand("dilemma", "/programs/p1.py", "/programs/p2.py")

	assert.Equal(t, []string{"dilemma", "/programs/p1.py", "/programs/p2.py"}, cmd)
	assert.NotContains(t, cmd, "-i")
}

func TestBuildCommand_EmptyGameType(t *testing.T) {
	e := newTestExecutor(t)

	cmd := e.buildCommand("", "/programs/p1.py", "/programs/p2.py")

	assert.Equal(t, "", cmd[0])
}

// --- hostToContainerPath (дополнительно) ---

// сам каталог программ в матч не отдаётся - там программы всех команд
func TestHostToContainerPath_ProgramsRootRejected(t *testing.T) {
	e := newTestExecutor(t)

	_, err := e.hostToContainerPath("/data/programs/")

	assert.Error(t, err)
}

func TestHostToContainerPath_TraversalNormalized(t *testing.T) {
	e := newTestExecutor(t)

	// путь с ".." нормализуется filepath.Clean до проверки префикса,
	// так что "/data/programs/../programs/evil" превращается в "/data/programs/evil"
	// и по-прежнему корректно отображается внутри пути контейнера
	result, err := e.hostToContainerPath("/data/programs/../programs/evil")
	require.NoError(t, err)
	assert.Equal(t, "/programs/evil", result)

	// путь, пытающийся выйти за programsPath, теперь возвращает ошибку
	_, err = e.hostToContainerPath("/data/programs/../../etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside programs directory")
}

func TestHostToContainerPath_DotSegments(t *testing.T) {
	e := newTestExecutor(t)

	// избыточные dot-сегменты вычищаются
	result, err := e.hostToContainerPath("/data/programs/./team1/../team1/solution.py")
	require.NoError(t, err)
	assert.Equal(t, "/programs/team1/solution.py", result)
}

func TestHostToContainerPath_SiblingDirectory(t *testing.T) {
	e := newTestExecutor(t)

	// "/data/programs-evil" начинается с "/data/programs", но не является подкаталогом;
	// должна вернуться ошибка, а не тихий проброс
	_, err := e.hostToContainerPath("/data/programs-evil/secret.py")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside programs directory")
}

// --- Close ---

func TestClose_NilDockerClient(t *testing.T) {
	e := newTestExecutor(t)
	e.dockerClient = nil

	err := e.Close()
	assert.NoError(t, err)
}

// --- parseResult: детали сообщения об ошибке ---

func TestParseResult_ExitCode1_NoStderr(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(1, "", "")

	require.NoError(t, err)
	assert.Equal(t, 2, result.Winner)
	assert.Equal(t, 1, result.ErrorCode)
	assert.Contains(t, result.ErrorMessage, "Программа 1")
}

func TestParseResult_ExitCode2_StderrOnly(t *testing.T) {
	e := newTestExecutor(t)

	result, err := e.parseResult(2, "", "some error details")

	require.NoError(t, err)
	assert.Equal(t, 1, result.Winner)
	assert.Contains(t, result.ErrorMessage, "stderr")
	assert.Contains(t, result.ErrorMessage, "some error details")
}

func TestParseResult_ExitCode1_ShortStdout_Filtered(t *testing.T) {
	e := newTestExecutor(t)

	// короткий stdout с пробелом (например, "10 15") должен отсеиваться
	result, err := e.parseResult(1, "10 15", "error")

	require.NoError(t, err)
	assert.NotContains(t, result.ErrorMessage, "stdout")
}

func TestHostToContainerPath_TrailingSlashInput(t *testing.T) {
	e := newTestExecutor(t)

	// filepath.Clean убирает завершающий слэш, совпадение должно сохраниться
	result, err := e.hostToContainerPath("/data/programs/team1/")
	require.NoError(t, err)
	assert.Equal(t, "/programs/team1", result)
}

// --- sanitizeStderr ---

func TestSanitizeStderr_Empty(t *testing.T) {
	assert.Equal(t, "", sanitizeStderr(""))
}

func TestSanitizeStderr_Clean(t *testing.T) {
	assert.Equal(t, "runtime error: index out of range", sanitizeStderr("runtime error: index out of range"))
}

func TestSanitizeStderr_StripANSI(t *testing.T) {
	input := "\x1b[31mERROR\x1b[0m: something failed\x1b[1;33m!"
	expected := "ERROR: something failed!"
	assert.Equal(t, expected, sanitizeStderr(input))
}

func TestSanitizeStderr_TruncateLongOutput(t *testing.T) {
	// строка длиннее 4KB
	long := strings.Repeat("x", 5000)
	result := sanitizeStderr(long)

	assert.LessOrEqual(t, len(result), maxStderrSize)
	assert.True(t, strings.HasSuffix(result, "...(truncated)"))
}

func TestSanitizeStderr_ExactlyAtLimit(t *testing.T) {
	// строка ровно в 4096 байт не усекается
	exact := strings.Repeat("a", maxStderrSize)
	result := sanitizeStderr(exact)

	assert.Equal(t, maxStderrSize, len(result))
	assert.False(t, strings.HasSuffix(result, "...(truncated)"))
}

func TestSanitizeStderr_OneBeyondLimit(t *testing.T) {
	// строка на один байт больше лимита усекается
	input := strings.Repeat("b", maxStderrSize+1)
	result := sanitizeStderr(input)

	assert.LessOrEqual(t, len(result), maxStderrSize)
	assert.True(t, strings.HasSuffix(result, "...(truncated)"))
}

func TestSanitizeStderr_ANSIStrippedBeforeTruncation(t *testing.T) {
	// ANSI-коды снимаются первыми, поэтому полезный объём
	// меряется без них;
	// контент, который превысил бы 4KB с ANSI, но без них помещается
	content := strings.Repeat("x", maxStderrSize-10)
	ansiPadding := strings.Repeat("\x1b[0m", 100) // добавляет 400 байт ANSI
	input := ansiPadding + content

	result := sanitizeStderr(input)

	// после снятия ANSI контент помещается в лимит
	assert.Equal(t, content, result)
	assert.False(t, strings.HasSuffix(result, "...(truncated)"))
}

func TestSanitizeStderr_MultipleANSICodes(t *testing.T) {
	input := "\x1b[1m\x1b[31mBold Red\x1b[0m \x1b[32mGreen\x1b[0m"
	expected := "Bold Red Green"
	assert.Equal(t, expected, sanitizeStderr(input))
}

// --- limitWriter ---

// TestLimitWriter_UnderLimit_WritesFully проверяет, что запись меньше лимита
// проходит без усечения и возвращает правильный счётчик
func TestLimitWriter_UnderLimit_WritesFully(t *testing.T) {
	var buf strings.Builder
	lw := &limitWriter{w: &buf, n: 100}
	n, err := lw.Write([]byte("hello world"))
	assert.NoError(t, err)
	assert.Equal(t, 11, n)
	assert.Equal(t, "hello world", buf.String())
	assert.Equal(t, 89, lw.n)
}

// TestLimitWriter_OverLimit_TruncatesButReportsFullWrite проверяет важное
// для stdcopy поведение: даже когда лимит достигнут, Write должен вернуть
// len(p), иначе stdcopy интерпретирует это как short-write и прервёт
// чтение второго потока
func TestLimitWriter_OverLimit_TruncatesButReportsFullWrite(t *testing.T) {
	var buf strings.Builder
	lw := &limitWriter{w: &buf, n: 5}
	n, err := lw.Write([]byte("hello world"))
	assert.NoError(t, err)
	assert.Equal(t, 11, n, "must report full length for stdcopy compatibility")
	assert.Equal(t, "hello", buf.String())
	assert.Equal(t, 0, lw.n)
}

// TestLimitWriter_AfterLimit_DropsSilently: после исчерпания бюджета
// последующие записи тихо отбрасываются, буфер не растёт
func TestLimitWriter_AfterLimit_DropsSilently(t *testing.T) {
	var buf strings.Builder
	lw := &limitWriter{w: &buf, n: 3}
	_, _ = lw.Write([]byte("abc"))
	n, err := lw.Write([]byte("def"))
	assert.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, "abc", buf.String())
}

// TestLimitWriter_IndependentBudgets: два writer'а имеют независимые лимиты,
// большой stdout не ворует бюджет stderr
func TestLimitWriter_IndependentBudgets(t *testing.T) {
	var outBuf, errBuf strings.Builder
	out := &limitWriter{w: &outBuf, n: 5}
	errW := &limitWriter{w: &errBuf, n: 5}

	_, _ = out.Write([]byte("very long stdout spam"))
	_, _ = errW.Write([]byte("ERR!"))

	assert.Equal(t, "very ", outBuf.String())
	assert.Equal(t, "ERR!", errBuf.String(), "stderr не должен пострадать от большого stdout")
}
