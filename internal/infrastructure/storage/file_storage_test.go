package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStorage(t *testing.T) *FileStorage {
	t.Helper()
	log, _ := logger.New("error", "json")
	basePath := t.TempDir()

	fs, err := NewFileStorage(Config{BasePath: basePath, MaxFileSize: 1024}, log)
	require.NoError(t, err)
	return fs
}

// --- sanitizeFilename ---

func TestSanitizeFilename_PathTraversal(t *testing.T) {
	result := sanitizeFilename("../../etc/passwd")
	assert.NotContains(t, result, "..")
	assert.NotContains(t, result, "/")
}

func TestSanitizeFilename_Slashes(t *testing.T) {
	result := sanitizeFilename("path/to/file.py")
	// filepath.Base extracts "file.py"
	assert.Equal(t, "file.py", result)
}

func TestSanitizeFilename_NullBytes(t *testing.T) {
	result := sanitizeFilename("file\x00name.py")
	assert.NotContains(t, result, "\x00")
}

func TestSanitizeFilename_Spaces(t *testing.T) {
	result := sanitizeFilename("my program.py")
	assert.Equal(t, "my_program.py", result)
}

func TestSanitizeFilename_NormalName(t *testing.T) {
	result := sanitizeFilename("solution.py")
	assert.Equal(t, "solution.py", result)
}

func TestSanitizeFilename_Backslashes(t *testing.T) {
	result := sanitizeFilename("path\\to\\file.go")
	// filepath.Base on unix treats backslash as part of name
	assert.NotContains(t, result, "\\")
}

// --- AllowedExtensions ---

func TestAllowedExtensions_Allowed(t *testing.T) {
	allowed := []string{".py", ".go", ".cpp", ".c", ".java", ".rs", ".js", ".ts", ".rb", ".php", ".cs", ".kt", ".lua", ""}
	for _, ext := range allowed {
		assert.True(t, AllowedExtensions[ext], "expected %q to be allowed", ext)
	}
}

func TestAllowedExtensions_NotAllowed(t *testing.T) {
	notAllowed := []string{".exe", ".sh", ".bat", ".dll", ".so"}
	for _, ext := range notAllowed {
		assert.False(t, AllowedExtensions[ext], "expected %q to not be allowed", ext)
	}
}

// --- GetProgramPath ---

func TestGetProgramPath(t *testing.T) {
	fs := newTestStorage(t)
	teamID := uuid.New()
	gameID := uuid.New()

	path := fs.GetProgramPath(teamID, gameID, 3, "solution.py")

	expected := filepath.Join(fs.basePath, teamID.String(), gameID.String(), "v3_solution.py")
	assert.Equal(t, expected, path)
}

// --- NewFileStorage ---

func TestNewFileStorage_CreatesDirectory(t *testing.T) {
	log, _ := logger.New("error", "json")
	basePath := filepath.Join(t.TempDir(), "subdir", "programs")

	fs, err := NewFileStorage(Config{BasePath: basePath}, log)

	require.NoError(t, err)
	assert.NotNil(t, fs)
	// Directory should exist
	info, err := os.Stat(basePath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestNewFileStorage_DefaultValues(t *testing.T) {
	log, _ := logger.New("error", "json")
	basePath := t.TempDir()

	fs, err := NewFileStorage(Config{BasePath: basePath}, log)

	require.NoError(t, err)
	assert.Equal(t, int64(10*1024*1024), fs.maxFileSize)
}

// --- DeleteProgram ---

func TestDeleteProgram_Success(t *testing.T) {
	fs := newTestStorage(t)
	ctx := context.Background()

	// Create a file to delete
	filePath := filepath.Join(fs.basePath, "test_file.py")
	require.NoError(t, os.WriteFile(filePath, []byte("print('hello')"), 0644))

	err := fs.DeleteProgram(ctx, filePath)
	assert.NoError(t, err)

	// File should be gone
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err))
}

func TestDeleteProgram_PathTraversal(t *testing.T) {
	fs := newTestStorage(t)
	ctx := context.Background()

	// Try to delete outside basePath
	err := fs.DeleteProgram(ctx, "/tmp/should_not_delete")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside storage directory")
}

func TestDeleteProgram_NonExistent(t *testing.T) {
	fs := newTestStorage(t)
	ctx := context.Background()

	err := fs.DeleteProgram(ctx, filepath.Join(fs.basePath, "nonexistent.py"))
	assert.NoError(t, err) // Not an error if file doesn't exist
}

// --- DeleteTeamPrograms ---

func TestDeleteTeamPrograms_Success(t *testing.T) {
	fs := newTestStorage(t)
	ctx := context.Background()
	teamID := uuid.New()
	gameID := uuid.New()

	// Create the directory structure with a file
	dir := filepath.Join(fs.basePath, teamID.String(), gameID.String())
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v1_solution.py"), []byte("code"), 0644))

	err := fs.DeleteTeamPrograms(ctx, teamID, gameID)
	assert.NoError(t, err)

	// Directory should be gone
	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err))
}

// --- GetLatestProgramPath ---

func TestGetLatestProgramPath_MultipleVersions(t *testing.T) {
	fs := newTestStorage(t)
	teamID := uuid.New()
	gameID := uuid.New()

	dir := filepath.Join(fs.basePath, teamID.String(), gameID.String())
	require.NoError(t, os.MkdirAll(dir, 0755))

	// Create multiple versions
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v1_solution.py"), []byte("v1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v3_solution.py"), []byte("v3"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v2_solution.py"), []byte("v2"), 0644))

	path, version, err := fs.GetLatestProgramPath(teamID, gameID)

	require.NoError(t, err)
	assert.Equal(t, 3, version)
	assert.Contains(t, path, "v3_solution.py")
}

func TestGetLatestProgramPath_EmptyDir(t *testing.T) {
	fs := newTestStorage(t)
	teamID := uuid.New()
	gameID := uuid.New()

	dir := filepath.Join(fs.basePath, teamID.String(), gameID.String())
	require.NoError(t, os.MkdirAll(dir, 0755))

	_, _, err := fs.GetLatestProgramPath(teamID, gameID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no programs found")
}

func TestGetLatestProgramPath_NoDirectory(t *testing.T) {
	fs := newTestStorage(t)

	_, _, err := fs.GetLatestProgramPath(uuid.New(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no programs found")
}

// --- FileExists ---

func TestFileExists_True(t *testing.T) {
	fs := newTestStorage(t)
	filePath := filepath.Join(fs.basePath, "existing.py")
	require.NoError(t, os.WriteFile(filePath, []byte("code"), 0644))

	assert.True(t, fs.FileExists(filePath))
}

func TestFileExists_False(t *testing.T) {
	fs := newTestStorage(t)

	assert.False(t, fs.FileExists(filepath.Join(fs.basePath, "nonexistent.py")))
}

// --- GetFileSize ---

func TestGetFileSize_Success(t *testing.T) {
	fs := newTestStorage(t)
	content := []byte("hello world")
	filePath := filepath.Join(fs.basePath, "file.py")
	require.NoError(t, os.WriteFile(filePath, content, 0644))

	size, err := fs.GetFileSize(filePath)
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), size)
}

func TestGetFileSize_NonExistent(t *testing.T) {
	fs := newTestStorage(t)

	_, err := fs.GetFileSize(filepath.Join(fs.basePath, "nonexistent.py"))
	assert.Error(t, err)
}

// --- GetBasePath ---

func TestGetBasePath(t *testing.T) {
	fs := newTestStorage(t)
	assert.NotEmpty(t, fs.GetBasePath())
	assert.Equal(t, fs.basePath, fs.GetBasePath())
}
