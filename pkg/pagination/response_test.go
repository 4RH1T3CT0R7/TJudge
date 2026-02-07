package pagination

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testNode struct {
	ID   uuid.UUID
	Name string
}

func testGetCursor(n testNode) (*Cursor, error) {
	return NewIDCursor(n.ID), nil
}

func TestNewConnection_Empty(t *testing.T) {
	first := 10
	pr := &PageRequest{First: &first}

	conn, err := NewConnection([]testNode{}, testGetCursor, pr, false)

	require.NoError(t, err)
	assert.Empty(t, conn.Edges)
	assert.False(t, conn.PageInfo.HasNextPage)
	assert.False(t, conn.PageInfo.HasPreviousPage)
	assert.Nil(t, conn.PageInfo.StartCursor)
	assert.Nil(t, conn.PageInfo.EndCursor)
	assert.Nil(t, conn.Total)
}

func TestNewConnection_WithNodes(t *testing.T) {
	nodes := []testNode{
		{ID: uuid.New(), Name: "first"},
		{ID: uuid.New(), Name: "second"},
		{ID: uuid.New(), Name: "third"},
	}
	first := 10
	pr := &PageRequest{First: &first}

	conn, err := NewConnection(nodes, testGetCursor, pr, false)

	require.NoError(t, err)
	assert.Len(t, conn.Edges, 3)
	assert.Equal(t, "first", conn.Edges[0].Node.Name)
	assert.Equal(t, "third", conn.Edges[2].Node.Name)
	assert.NotEmpty(t, conn.Edges[0].Cursor)
	assert.NotEmpty(t, conn.Edges[2].Cursor)
	require.NotNil(t, conn.PageInfo.StartCursor)
	require.NotNil(t, conn.PageInfo.EndCursor)
	assert.Equal(t, conn.Edges[0].Cursor, *conn.PageInfo.StartCursor)
	assert.Equal(t, conn.Edges[2].Cursor, *conn.PageInfo.EndCursor)
}

func TestNewConnection_HasNextPage_Forward(t *testing.T) {
	nodes := []testNode{{ID: uuid.New(), Name: "node"}}
	first := 1
	pr := &PageRequest{First: &first}

	conn, err := NewConnection(nodes, testGetCursor, pr, true)

	require.NoError(t, err)
	assert.True(t, conn.PageInfo.HasNextPage)
	assert.False(t, conn.PageInfo.HasPreviousPage)
}

func TestNewConnection_HasPreviousPage_Backward(t *testing.T) {
	nodes := []testNode{{ID: uuid.New(), Name: "node"}}
	last := 1
	pr := &PageRequest{Last: &last}

	conn, err := NewConnection(nodes, testGetCursor, pr, true)

	require.NoError(t, err)
	assert.False(t, conn.PageInfo.HasNextPage)
	assert.True(t, conn.PageInfo.HasPreviousPage)
}

func TestNewConnection_NoMore(t *testing.T) {
	nodes := []testNode{{ID: uuid.New(), Name: "node"}}
	first := 10
	pr := &PageRequest{First: &first}

	conn, err := NewConnection(nodes, testGetCursor, pr, false)

	require.NoError(t, err)
	assert.False(t, conn.PageInfo.HasNextPage)
	assert.False(t, conn.PageInfo.HasPreviousPage)
}

func TestNewConnectionWithTotal(t *testing.T) {
	nodes := []testNode{
		{ID: uuid.New(), Name: "a"},
		{ID: uuid.New(), Name: "b"},
	}
	first := 10
	pr := &PageRequest{First: &first}

	conn, err := NewConnectionWithTotal(nodes, testGetCursor, pr, true, 42)

	require.NoError(t, err)
	assert.Len(t, conn.Edges, 2)
	require.NotNil(t, conn.Total)
	assert.Equal(t, 42, *conn.Total)
	assert.True(t, conn.PageInfo.HasNextPage)
}

func TestNewConnectionWithTotal_Zero(t *testing.T) {
	first := 10
	pr := &PageRequest{First: &first}

	conn, err := NewConnectionWithTotal([]testNode{}, testGetCursor, pr, false, 0)

	require.NoError(t, err)
	assert.Empty(t, conn.Edges)
	require.NotNil(t, conn.Total)
	assert.Equal(t, 0, *conn.Total)
}
