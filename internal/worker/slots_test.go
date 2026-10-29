package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchSlots(t *testing.T) {
	assert.Equal(t, 4, MatchSlots(100000, 4), "квота в ядро - матч на ядро")
	assert.Equal(t, 8, MatchSlots(50000, 4), "пол-ядра - два матча на ядро")
	assert.Equal(t, 2, MatchSlots(200000, 4))
	assert.Equal(t, 4, MatchSlots(0, 4), "без квоты матч считается ядром")
	assert.Equal(t, 1, MatchSlots(400000, 2), "хотя бы один")
}
