package game

// GamePlugin describes a game type's metadata and configuration.
// Plugins are registered at startup and used to validate game creation.
type GamePlugin struct {
	Name              string
	DisplayName       string
	DefaultRules      string
	DefaultIterations int
	ScoreMultiplier   float64
}
