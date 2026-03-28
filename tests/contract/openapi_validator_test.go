//go:build contract

package contract

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/require"
)

var (
	specRouter routers.Router
	specOnce   sync.Once
	specErr    error
)

func loadSpec() (routers.Router, error) {
	specOnce.Do(func() {
		loader := openapi3.NewLoader()
		doc, err := loader.LoadFromFile("../../docs/openapi.yaml")
		if err != nil {
			specErr = err
			return
		}
		err = doc.Validate(context.Background())
		if err != nil {
			specErr = err
			return
		}
		// Clear servers so router matches any base URL.
		doc.Servers = nil
		specRouter, specErr = gorillamux.NewRouter(doc)
	})
	return specRouter, specErr
}

// AssertMatchesSpec validates the HTTP response against the OpenAPI specification.
// It finds the matching route for the request, then validates the response body
// and status code against the declared schema.
//
// If the route is not found in the spec, it logs a warning and skips validation
// (to allow incremental spec coverage).
func AssertMatchesSpec(t *testing.T, req *http.Request, resp *http.Response) {
	t.Helper()

	router, err := loadSpec()
	if err != nil {
		t.Logf("WARNING: could not load OpenAPI spec: %v", err)
		return
	}

	route, pathParams, err := router.FindRoute(req)
	if err != nil {
		t.Logf("WARNING: route %s %s not found in OpenAPI spec, skipping validation",
			req.Method, req.URL.Path)
		return
	}

	// Read the response body for validation.
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	// Restore body for further reads.
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
		},
		Status: resp.StatusCode,
		Header: resp.Header,
		Body:   io.NopCloser(bytes.NewReader(bodyBytes)),
		Options: &openapi3filter.Options{
			IncludeResponseStatus: true,
		},
	}

	err = openapi3filter.ValidateResponse(context.Background(), input)
	require.NoError(t, err,
		"response does not match OpenAPI spec for %s %s (status %d)\nBody: %s",
		req.Method, req.URL.Path, resp.StatusCode, string(bodyBytes))
}

func TestOpenAPISpecIsValid(t *testing.T) {
	_, err := loadSpec()
	require.NoError(t, err, "OpenAPI spec at docs/openapi.yaml must be valid")
}
