package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func TestSpaHandler(t *testing.T) {
	h := spaHandler(http.FS(fstest.MapFS{
		"index.html":        {Data: []byte("<html>app</html>")},
		"favicon.svg":       {Data: []byte("<svg/>")},
		"assets/app-abc.js": {Data: []byte("console.log(1)")},
	}))

	tests := []struct {
		path         string
		code         int
		cacheControl string
		body         string
	}{
		{"/assets/app-abc.js", http.StatusOK, "public, max-age=31536000, immutable", "console.log(1)"},
		{"/assets/old-123.js", http.StatusNotFound, "", ""},
		{"/assets/", http.StatusNotFound, "", ""},
		{"/tournaments/42", http.StatusOK, "no-cache", "<html>app</html>"},
		{"/", http.StatusOK, "no-cache", "<html>app</html>"},
		{"/favicon.svg", http.StatusOK, "", "<svg/>"},
		{"/api/v1/unknown", http.StatusNotFound, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			assert.Equal(t, tt.code, rec.Code)
			assert.Equal(t, tt.cacheControl, rec.Header().Get("Cache-Control"))
			if tt.body != "" {
				assert.Equal(t, tt.body, rec.Body.String())
			}
		})
	}
}
