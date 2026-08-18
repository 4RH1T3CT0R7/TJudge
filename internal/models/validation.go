package models

// проверяет юзера перед сохранением
func (u *User) Validate() error {
	errs := ValidationErrors{}

	if err := ValidateUsername(u.Username); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	}

	if err := ValidateEmail(u.Email); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// Validate валидирует Program
func (p *Program) Validate() error {
	errs := ValidationErrors{}

	// TODO: одинаковые лимиты длины раскиданы по всем Validate, вынести бы в константы
	if err := ValidateRequired("name", p.Name); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	} else if err := ValidateLength("name", p.Name, 1, 100); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	}

	if err := ValidateRequired("game_type", p.GameType); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	} else if err := ValidateLength("game_type", p.GameType, 1, 50); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	}

	if err := ValidateRequired("code_path", p.CodePath); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	}

	if err := ValidateRequired("language", p.Language); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	} else if err := ValidateLength("language", p.Language, 1, 50); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// Validate валидирует Tournament
func (t *Tournament) Validate() error {
	errs := ValidationErrors{}

	if err := ValidateRequired("name", t.Name); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	} else if err := ValidateLength("name", t.Name, 1, 255); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	}

	if err := ValidateRequired("game_type", t.GameType); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	} else if err := ValidateLength("game_type", t.GameType, 1, 50); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	}

	// проверка статуса
	validStatuses := []string{
		string(TournamentPending),
		string(TournamentActive),
		string(TournamentCompleted),
		string(TournamentCancelled),
	}
	if err := ValidateEnum("status", string(t.Status), validStatuses); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	}

	// min 2, тк турниру нужно хотя бы 2 участника
	if t.MaxParticipants != nil && *t.MaxParticipants < 2 {
		errs.Add("max_participants", "max_participants must be at least 2")
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// Validate валидирует Match.
// правила тут дёргает планировщик, так что руками не трогаем поведение
func (m *Match) Validate() error {
	errs := ValidationErrors{}

	// проверка статуса
	validStatuses := []string{
		string(MatchPending),
		string(MatchRunning),
		string(MatchCompleted),
		string(MatchFailed),
		string(MatchCancelled),
	}
	if err := ValidateEnum("status", string(m.Status), validStatuses); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	}

	// проверка приоритета
	validPriorities := []string{
		string(PriorityHigh),
		string(PriorityMedium),
		string(PriorityLow),
	}
	if err := ValidateEnum("priority", string(m.Priority), validPriorities); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			errs = append(errs, ve)
		}
	}

	// программы не должны совпадать
	if m.Program1ID == m.Program2ID {
		errs.Add("program2_id", "program1 and program2 must be different")
	}

	// winner: 0 ничья, 1 или 2 - победитель
	if m.Winner != nil && (*m.Winner < 0 || *m.Winner > 2) {
		errs.Add("winner", "winner must be 0 (draw), 1 (program1) or 2 (program2)")
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// Validate валидирует TournamentParticipant
func (tp *TournamentParticipant) Validate() error {
	errs := ValidationErrors{}

	if tp.Rating < 0 {
		errs.Add("rating", "rating cannot be negative")
	}

	if tp.Wins < 0 {
		errs.Add("wins", "wins cannot be negative")
	}

	if tp.Losses < 0 {
		errs.Add("losses", "losses cannot be negative")
	}

	if tp.Draws < 0 {
		errs.Add("draws", "draws cannot be negative")
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}
