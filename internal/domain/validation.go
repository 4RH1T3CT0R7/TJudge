package domain

import (
	"github.com/bmstu-itstech/tjudge/pkg/validator"
)

// Validate валидирует User
func (u *User) Validate() error {
	errs := validator.ValidationErrors{}

	if err := validator.ValidateUsername(u.Username); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	}

	if err := validator.ValidateEmail(u.Email); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// ValidatePassword валидирует пароль при регистрации
func ValidatePassword(password string) error {
	return validator.ValidatePassword(password)
}

// Validate валидирует Program
func (p *Program) Validate() error {
	errs := validator.ValidationErrors{}

	if err := validator.ValidateRequired("name", p.Name); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	} else if err := validator.ValidateLength("name", p.Name, 1, 100); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	}

	if err := validator.ValidateRequired("game_type", p.GameType); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	} else if err := validator.ValidateLength("game_type", p.GameType, 1, 50); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	}

	if err := validator.ValidateRequired("code_path", p.CodePath); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	}

	if err := validator.ValidateRequired("language", p.Language); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	} else if err := validator.ValidateLength("language", p.Language, 1, 50); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// Validate валидирует Tournament
func (t *Tournament) Validate() error {
	errs := validator.ValidationErrors{}

	if err := validator.ValidateRequired("name", t.Name); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	} else if err := validator.ValidateLength("name", t.Name, 1, 255); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	}

	if err := validator.ValidateRequired("game_type", t.GameType); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	} else if err := validator.ValidateLength("game_type", t.GameType, 1, 50); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	}

	// Валидация статуса
	validStatuses := []string{
		string(TournamentPending),
		string(TournamentActive),
		string(TournamentCompleted),
		string(TournamentCancelled),
	}
	if err := validator.ValidateEnum("status", string(t.Status), validStatuses); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	}

	// Валидация max_participants
	if t.MaxParticipants != nil && *t.MaxParticipants <= 0 {
		errs.Add("max_participants", "max_participants must be positive")
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// Validate валидирует Match
func (m *Match) Validate() error {
	errs := validator.ValidationErrors{}

	// Валидация статуса
	validStatuses := []string{
		string(MatchPending),
		string(MatchRunning),
		string(MatchCompleted),
		string(MatchFailed),
	}
	if err := validator.ValidateEnum("status", string(m.Status), validStatuses); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	}

	// Валидация приоритета
	validPriorities := []string{
		string(PriorityHigh),
		string(PriorityMedium),
		string(PriorityLow),
	}
	if err := validator.ValidateEnum("priority", string(m.Priority), validPriorities); err != nil {
		errs = append(errs, err.(*validator.ValidationError))
	}

	// Программы не должны быть одинаковыми
	if m.Program1ID == m.Program2ID {
		errs.Add("program2_id", "program1 and program2 must be different")
	}

	// Валидация winner
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
	errs := validator.ValidationErrors{}

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
