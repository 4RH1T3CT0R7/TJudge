package game

// GamePlugin — метаданные игрового типа
// плагины регистрируются на старте, по ним валидируется создание игр
type GamePlugin struct {
	Name              string
	DisplayName       string
	DefaultRules      string
	DefaultIterations int
	ScoreMultiplier   float64
}
