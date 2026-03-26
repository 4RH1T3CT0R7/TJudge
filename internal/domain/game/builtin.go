package game

// RegisterBuiltinGames registers all 5 built-in game plugins supported by tjudge-cli.
func RegisterBuiltinGames(registry *Registry) {
	// Дилемма заключённого (Prisoner's Dilemma)
	// Note: the DB name was renamed from "prisoners_dilemma" to "dilemma" in migration 000022.
	_ = registry.Register(&GamePlugin{
		Name:              "dilemma",
		DisplayName:       "Дилемма заключённого",
		DefaultIterations: 100,
		ScoreMultiplier:   1.0,
	})

	// Перетягивание каната (Tug of War)
	_ = registry.Register(&GamePlugin{
		Name:              "tug_of_war",
		DisplayName:       "Перетягивание каната",
		DefaultIterations: 100,
		ScoreMultiplier:   10.0,
	})

	// Дилемма путешественника (Traveler's Dilemma)
	_ = registry.Register(&GamePlugin{
		Name:              "travelers_dilemma",
		DisplayName:       "Дилемма путешественника",
		DefaultIterations: 100,
		ScoreMultiplier:   0.05,
	})

	// Общественное благо (Public Goods Game)
	_ = registry.Register(&GamePlugin{
		Name:              "public_goods",
		DisplayName:       "Общественное благо",
		DefaultIterations: 100,
		ScoreMultiplier:   0.1,
	})

	// Аукцион двойной цены (Dollar Auction)
	_ = registry.Register(&GamePlugin{
		Name:              "dollar_auction",
		DisplayName:       "Аукцион двойной цены",
		DefaultIterations: 100,
		ScoreMultiplier:   1.0,
	})
}
