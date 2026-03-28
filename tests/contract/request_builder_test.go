//go:build contract

package contract

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// RequestBuilder provides a fluent API for constructing and executing HTTP
// requests against the test harness server.
type RequestBuilder struct {
	t       *testing.T
	client  *http.Client
	method  string
	url     string
	body    io.Reader
	headers map[string]string
}

func (h *TestHarness) newRequest(method, path string) *RequestBuilder {
	return &RequestBuilder{
		t:       h.t,
		client:  h.Client,
		method:  method,
		url:     h.URL + path,
		headers: make(map[string]string),
	}
}

// GET creates a RequestBuilder for a GET request.
func (h *TestHarness) GET(path string) *RequestBuilder { return h.newRequest(http.MethodGet, path) }

// POST creates a RequestBuilder for a POST request.
func (h *TestHarness) POST(path string) *RequestBuilder { return h.newRequest(http.MethodPost, path) }

// PUT creates a RequestBuilder for a PUT request.
func (h *TestHarness) PUT(path string) *RequestBuilder { return h.newRequest(http.MethodPut, path) }

// DELETE creates a RequestBuilder for a DELETE request.
func (h *TestHarness) DELETE(path string) *RequestBuilder {
	return h.newRequest(http.MethodDelete, path)
}

// PATCH creates a RequestBuilder for a PATCH request.
func (h *TestHarness) PATCH(path string) *RequestBuilder {
	return h.newRequest(http.MethodPatch, path)
}

// WithAuth adds a Bearer token to the Authorization header.
func (rb *RequestBuilder) WithAuth(token string) *RequestBuilder {
	rb.headers["Authorization"] = "Bearer " + token
	return rb
}

// WithJSON marshals the given value to JSON and sets the Content-Type header.
func (rb *RequestBuilder) WithJSON(v interface{}) *RequestBuilder {
	rb.t.Helper()
	data, err := json.Marshal(v)
	require.NoError(rb.t, err, "failed to marshal request body to JSON")
	rb.body = bytes.NewReader(data)
	rb.headers["Content-Type"] = "application/json"
	return rb
}

// WithBody sets a raw byte slice as the request body.
func (rb *RequestBuilder) WithBody(data []byte) *RequestBuilder {
	rb.body = bytes.NewReader(data)
	return rb
}

// WithHeader sets a custom header on the request.
func (rb *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	rb.headers[key] = value
	return rb
}

// WithMultipartFile builds a multipart/form-data body with the given file
// field and optional extra string fields.
func (rb *RequestBuilder) WithMultipartFile(fieldName, filename string, content []byte, extraFields map[string]string) *RequestBuilder {
	rb.t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Write the file part.
	part, err := writer.CreateFormFile(fieldName, filename)
	require.NoError(rb.t, err, "failed to create form file part")
	_, err = part.Write(content)
	require.NoError(rb.t, err, "failed to write file content")

	// Write extra string fields.
	for k, v := range extraFields {
		err = writer.WriteField(k, v)
		require.NoError(rb.t, err, "failed to write form field %s", k)
	}

	err = writer.Close()
	require.NoError(rb.t, err, "failed to close multipart writer")

	rb.body = &buf
	rb.headers["Content-Type"] = writer.FormDataContentType()
	return rb
}

// Do builds the http.Request, executes it, and returns the response.
// The response body is automatically closed via t.Cleanup.
func (rb *RequestBuilder) Do() *http.Response {
	rb.t.Helper()

	var body io.Reader
	if rb.body != nil {
		body = rb.body
	}

	req, err := http.NewRequest(rb.method, rb.url, body)
	require.NoError(rb.t, err, "failed to build HTTP request")

	for k, v := range rb.headers {
		req.Header.Set(k, v)
	}

	resp, err := rb.client.Do(req)
	require.NoError(rb.t, err, "failed to execute HTTP request")

	rb.t.Cleanup(func() { resp.Body.Close() })

	return resp
}
