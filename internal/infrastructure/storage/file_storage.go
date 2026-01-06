package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// FileStorage управляет хранением файлов программ
type FileStorage struct {
	basePath    string
	maxFileSize int64
	log         *logger.Logger
}

// Config конфигурация для FileStorage
type Config struct {
	BasePath    string
	MaxFileSize int64 // В байтах, по умолчанию 10MB
}

// NewFileStorage создаёт новый FileStorage
func NewFileStorage(cfg Config, log *logger.Logger) (*FileStorage, error) {
	if cfg.BasePath == "" {
		cfg.BasePath = "/data/programs"
	}

	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = 10 * 1024 * 1024 // 10MB
	}

	// Создаём базовую директорию если не существует
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create storage directory")
	}

	return &FileStorage{
		basePath:    cfg.BasePath,
		maxFileSize: cfg.MaxFileSize,
		log:         log,
	}, nil
}

// AllowedExtensions - разрешённые расширения файлов
var AllowedExtensions = map[string]bool{
	".py":   true, // Python
	".go":   true, // Go
	".cpp":  true, // C++
	".c":    true, // C
	".java": true, // Java
	".rs":   true, // Rust
	".js":   true, // JavaScript
	".ts":   true, // TypeScript
	".rb":   true, // Ruby
	".php":  true, // PHP
	".cs":   true, // C#
	".kt":   true, // Kotlin
	".lua":  true, // Lua
	"":      true, // Без расширения (бинарники)
}

// SaveProgram сохраняет файл программы
func (s *FileStorage) SaveProgram(
	ctx context.Context,
	teamID uuid.UUID,
	gameID uuid.UUID,
	version int,
	file multipart.File,
	header *multipart.FileHeader,
) (string, error) {
	// Проверяем размер файла
	if header.Size > s.maxFileSize {
		return "", errors.ErrBadRequest.WithMessage(fmt.Sprintf("file too large, max size is %d bytes", s.maxFileSize))
	}

	// Проверяем расширение
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !AllowedExtensions[ext] {
		return "", errors.ErrBadRequest.WithMessage("file type not allowed")
	}

	// Создаём структуру директорий: basePath/teamID/gameID/
	dir := filepath.Join(s.basePath, teamID.String(), gameID.String())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", errors.Wrap(err, "failed to create program directory")
	}

	// Генерируем имя файла: v{version}_{original_name}
	filename := fmt.Sprintf("v%d_%s", version, sanitizeFilename(header.Filename))
	filePath := filepath.Join(dir, filename)

	// Создаём файл
	dst, err := os.Create(filePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to create file")
	}
	defer dst.Close()

	// Копируем содержимое
	written, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(filePath) // Удаляем частично записанный файл
		return "", errors.Wrap(err, "failed to write file")
	}

	// Делаем файл исполняемым
	if err := os.Chmod(filePath, 0755); err != nil {
		s.log.Error("Failed to make file executable", zap.Error(err), zap.String("path", filePath))
	}

	s.log.Info("Program file saved",
		zap.String("team_id", teamID.String()),
		zap.String("game_id", gameID.String()),
		zap.Int("version", version),
		zap.String("filename", filename),
		zap.Int64("size", written),
	)

	return filePath, nil
}

// GetProgramPath возвращает путь к программе
func (s *FileStorage) GetProgramPath(teamID, gameID uuid.UUID, version int, originalFilename string) string {
	filename := fmt.Sprintf("v%d_%s", version, sanitizeFilename(originalFilename))
	return filepath.Join(s.basePath, teamID.String(), gameID.String(), filename)
}

// DeleteProgram удаляет файл программы
func (s *FileStorage) DeleteProgram(ctx context.Context, path string) error {
	// Проверяем что путь внутри basePath для безопасности
	absPath, err := filepath.Abs(path)
	if err != nil {
		return errors.Wrap(err, "failed to get absolute path")
	}

	absBasePath, err := filepath.Abs(s.basePath)
	if err != nil {
		return errors.Wrap(err, "failed to get absolute base path")
	}

	if !strings.HasPrefix(absPath, absBasePath) {
		return errors.ErrForbidden.WithMessage("path outside storage directory")
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil // Файл уже удалён
		}
		return errors.Wrap(err, "failed to delete file")
	}

	s.log.Info("Program file deleted", zap.String("path", path))

	return nil
}

// DeleteTeamPrograms удаляет все программы команды для игры
func (s *FileStorage) DeleteTeamPrograms(ctx context.Context, teamID, gameID uuid.UUID) error {
	dir := filepath.Join(s.basePath, teamID.String(), gameID.String())

	if err := os.RemoveAll(dir); err != nil {
		return errors.Wrap(err, "failed to delete team programs directory")
	}

	s.log.Info("Team programs deleted", zap.String("team_id", teamID.String()), zap.String("game_id", gameID.String()))

	return nil
}

// GetLatestProgramPath находит последнюю версию программы
func (s *FileStorage) GetLatestProgramPath(teamID, gameID uuid.UUID) (string, int, error) {
	dir := filepath.Join(s.basePath, teamID.String(), gameID.String())

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, errors.ErrNotFound.WithMessage("no programs found")
		}
		return "", 0, errors.Wrap(err, "failed to read programs directory")
	}

	var latestVersion int
	var latestPath string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Парсим версию из имени файла (v{version}_...)
		var version int
		if _, err := fmt.Sscanf(name, "v%d_", &version); err == nil {
			if version > latestVersion {
				latestVersion = version
				latestPath = filepath.Join(dir, name)
			}
		}
	}

	if latestPath == "" {
		return "", 0, errors.ErrNotFound.WithMessage("no programs found")
	}

	return latestPath, latestVersion, nil
}

// FileExists проверяет существует ли файл
func (s *FileStorage) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetFileSize возвращает размер файла
func (s *FileStorage) GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get file info")
	}
	return info.Size(), nil
}

// sanitizeFilename очищает имя файла от опасных символов
func sanitizeFilename(filename string) string {
	// Оставляем только базовое имя файла
	filename = filepath.Base(filename)

	// Заменяем опасные символы
	replacer := strings.NewReplacer(
		"..", "_",
		"/", "_",
		"\\", "_",
		"\x00", "_",
		" ", "_",
	)

	return replacer.Replace(filename)
}

// GetBasePath возвращает базовый путь хранилища
func (s *FileStorage) GetBasePath() string {
	return s.basePath
}
