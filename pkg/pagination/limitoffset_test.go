package pagination

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newRequest(query string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/test?"+query, nil)
}

func TestParseLimitOffset_DefaultValues(t *testing.T) {
	r := newRequest("")
	lo := ParseLimitOffset(r, 20, 100)

	assert.Equal(t, 20, lo.Limit)
	assert.Equal(t, 0, lo.Offset)
}

func TestParseLimitOffset_ValidLimitAndOffset(t *testing.T) {
	r := newRequest("limit=50&offset=10")
	lo := ParseLimitOffset(r, 20, 100)

	assert.Equal(t, 50, lo.Limit)
	assert.Equal(t, 10, lo.Offset)
}

func TestParseLimitOffset_LimitExceedsMax(t *testing.T) {
	r := newRequest("limit=200")
	lo := ParseLimitOffset(r, 20, 100)

	assert.Equal(t, 20, lo.Limit) // откатывается к значению по умолчанию
}

func TestParseLimitOffset_NegativeLimit(t *testing.T) {
	r := newRequest("limit=-5")
	lo := ParseLimitOffset(r, 20, 100)

	assert.Equal(t, 20, lo.Limit)
}

func TestParseLimitOffset_ZeroLimit(t *testing.T) {
	r := newRequest("limit=0")
	lo := ParseLimitOffset(r, 20, 100)

	assert.Equal(t, 20, lo.Limit) // 0 не > 0
}

func TestParseLimitOffset_NonNumericLimit(t *testing.T) {
	r := newRequest("limit=abc")
	lo := ParseLimitOffset(r, 20, 100)

	assert.Equal(t, 20, lo.Limit)
}

func TestParseLimitOffset_NegativeOffset(t *testing.T) {
	r := newRequest("offset=-1")
	lo := ParseLimitOffset(r, 20, 100)

	assert.Equal(t, 0, lo.Offset) // отрицательные игнорируются
}

func TestParseLimitOffset_NonNumericOffset(t *testing.T) {
	r := newRequest("offset=xyz")
	lo := ParseLimitOffset(r, 20, 100)

	assert.Equal(t, 0, lo.Offset)
}

func TestParseLimitOffset_ZeroMaxLimit_UsesDefault(t *testing.T) {
	r := newRequest("limit=50")
	lo := ParseLimitOffset(r, 20, 0)

	// maxLimit=0 означает DefaultMaxLimit=100, поэтому 50 принимается
	assert.Equal(t, 50, lo.Limit)
}

func TestParseLimitOffset_NegativeMaxLimit_UsesDefault(t *testing.T) {
	r := newRequest("limit=50")
	lo := ParseLimitOffset(r, 20, -1)

	assert.Equal(t, 50, lo.Limit)
}

func TestParseLimitOffset_LimitEqualsMax(t *testing.T) {
	r := newRequest("limit=100")
	lo := ParseLimitOffset(r, 20, 100)

	assert.Equal(t, 100, lo.Limit)
}

func TestParseLimitOffset_ZeroOffset(t *testing.T) {
	r := newRequest("offset=0")
	lo := ParseLimitOffset(r, 20, 100)

	assert.Equal(t, 0, lo.Offset)
}
