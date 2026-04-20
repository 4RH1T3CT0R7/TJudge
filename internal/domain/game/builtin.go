package game

// RegisterBuiltinGames регистрирует все 5 встроенных игровых плагинов, поддерживаемых tjudge-cli.
// Паникует при ошибке регистрации (программная ошибка - дубликат имён, nil-плагин и т.п.).
func RegisterBuiltinGames(registry *Registry) {
	must := func(err error) {
		if err != nil {
			panic("builtin game registration failed: " + err.Error())
		}
	}

	must(registry.Register(&GamePlugin{
		Name:              "dilemma",
		DisplayName:       "Дилемма заключённого",
		DefaultIterations: 100,
		ScoreMultiplier:   1.0,
	}))

	must(registry.Register(&GamePlugin{
		Name:              "tug_of_war",
		DisplayName:       "Перетягивание каната",
		DefaultIterations: 100,
		ScoreMultiplier:   10.0,
	}))

	must(registry.Register(&GamePlugin{
		Name:              "travelers_dilemma",
		DisplayName:       "Дилемма путешественника",
		DefaultIterations: 100,
		ScoreMultiplier:   0.05,
	}))

	must(registry.Register(&GamePlugin{
		Name:              "public_goods",
		DisplayName:       "Общественное благо",
		DefaultIterations: 100,
		ScoreMultiplier:   0.1,
	}))

	must(registry.Register(&GamePlugin{
		Name:              "dollar_auction",
		DisplayName:       "Аукцион двойной цены",
		DefaultIterations: 100,
		ScoreMultiplier:   1.0,
	}))
}
