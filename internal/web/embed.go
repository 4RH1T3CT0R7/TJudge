package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// GetFileSystem возвращает файловую систему для статических файлов
func GetFileSystem() (http.FileSystem, error) {
	subFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}
	return http.FS(subFS), nil
}

// Handler создаёт HTTP handler для статических файлов
// Поддерживает SPA fallback на index.html для клиентской маршрутизации
func Handler() http.Handler {
	fsys, err := GetFileSystem()
	if err != nil {
		// В случае ошибки возвращаем заглушку
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Frontend not available", http.StatusServiceUnavailable)
		})
	}

	fileServer := http.FileServer(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Для API запросов пропускаем
		if strings.HasPrefix(path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Пробуем найти файл
		f, err := fsys.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback - возвращаем index.html для всех остальных путей
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
