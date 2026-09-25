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
		// в случае ошибки возвращается заглушка
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Frontend not available", http.StatusServiceUnavailable)
		})
	}
	return spaHandler(fsys)
}

// spaHandler: хешированные ассеты из /assets/ кэшируются навсегда, index.html
// перепроверяется на каждый заход, чтобы после релиза вкладка получила новые чанки
func spaHandler(fsys http.FileSystem) http.Handler {
	fileServer := http.FileServer(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// для API запросов - пропуск
		if strings.HasPrefix(path, "/api/") {
			http.NotFound(w, r)
			return
		}

		isAsset := strings.HasPrefix(path, "/assets/")
		if isFile(fsys, path) {
			if isAsset {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// чанка прошлого релиза нет: 404, а не index.html, иначе lazy-импорт получит html
		if isAsset {
			http.NotFound(w, r)
			return
		}

		// SPA fallback - возвращается index.html для всех остальных путей
		w.Header().Set("Cache-Control", "no-cache")
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

// isFile - существует ли обычный файл (каталоги не отдаются, без листинга /assets/)
func isFile(fsys http.FileSystem, path string) bool {
	f, err := fsys.Open(strings.TrimPrefix(path, "/"))
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	return err == nil && !st.IsDir()
}
