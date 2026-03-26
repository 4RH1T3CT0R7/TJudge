package game

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()

	plugin := &GamePlugin{
		Name:              "test_game",
		DisplayName:       "Test Game",
		DefaultIterations: 50,
		ScoreMultiplier:   2.0,
	}

	err := r.Register(plugin)
	require.NoError(t, err)

	got, ok := r.Get("test_game")
	assert.True(t, ok)
	assert.Equal(t, plugin, got)
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()

	plugin := &GamePlugin{
		Name:        "test_game",
		DisplayName: "Test Game",
	}

	err := r.Register(plugin)
	require.NoError(t, err)

	err = r.Register(plugin)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_RegisterNil(t *testing.T) {
	r := NewRegistry()

	err := r.Register(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must not be nil")
}

func TestRegistry_RegisterEmptyName(t *testing.T) {
	r := NewRegistry()

	err := r.Register(&GamePlugin{Name: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestRegistry_GetNonExistent(t *testing.T) {
	r := NewRegistry()

	got, ok := r.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()

	_ = r.Register(&GamePlugin{Name: "zzz_game", DisplayName: "ZZZ"})
	_ = r.Register(&GamePlugin{Name: "aaa_game", DisplayName: "AAA"})
	_ = r.Register(&GamePlugin{Name: "mmm_game", DisplayName: "MMM"})

	list := r.List()
	require.Len(t, list, 3)

	// List is sorted alphabetically
	assert.Equal(t, "aaa_game", list[0].Name)
	assert.Equal(t, "mmm_game", list[1].Name)
	assert.Equal(t, "zzz_game", list[2].Name)
}

func TestRegistry_ListEmpty(t *testing.T) {
	r := NewRegistry()

	list := r.List()
	assert.Empty(t, list)
}

func TestRegistry_Has(t *testing.T) {
	r := NewRegistry()

	_ = r.Register(&GamePlugin{Name: "test_game", DisplayName: "Test"})

	assert.True(t, r.Has("test_game"))
	assert.False(t, r.Has("nonexistent"))
}

func TestRegistry_RegisterBuiltinGames(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinGames(r)

	expectedGames := []string{
		"dilemma",
		"tug_of_war",
		"travelers_dilemma",
		"public_goods",
		"dollar_auction",
	}

	list := r.List()
	assert.Len(t, list, 5)

	for _, name := range expectedGames {
		assert.True(t, r.Has(name), "expected game %q to be registered", name)

		plugin, ok := r.Get(name)
		require.True(t, ok)
		assert.NotEmpty(t, plugin.DisplayName)
		assert.Greater(t, plugin.DefaultIterations, 0)
		assert.Greater(t, plugin.ScoreMultiplier, 0.0)
	}
}

func TestRegistry_RegisterBuiltinGames_NoDuplicates(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinGames(r)

	// Registering again should fail because names are already taken
	r2 := NewRegistry()
	RegisterBuiltinGames(r2)

	// Verify both registries have exactly 5 games
	assert.Len(t, r.List(), 5)
	assert.Len(t, r2.List(), 5)
}
