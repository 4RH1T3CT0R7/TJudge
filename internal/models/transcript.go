package models

// Transcript - ходы матча по итерациям из вывода tjudge-cli -v. в JSON
// хранится в matches.transcript и отдаётся как есть.
// Moves[0] - ходы программы 1, Moves[1] - программы 2. в дилемме 1 -
// сотрудничество, 0 - предательство; в аукционе это ставки по очереди, 0 - пас.
// у упавшего матча ходы обрываются на ошибке, стороны бывают разной длины.
// Points - очки сторон за каждую итерацию, у аукциона их нет: счёт только итоговый
type Transcript struct {
	Moves  [][]int `json:"moves"`
	Points [][]int `json:"points,omitempty"`
}

// ходы дилеммы в Transcript.Moves
const (
	MoveDefect    = 0
	MoveCooperate = 1
)
