package models

import "github.com/google/uuid"

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

// StrategyProfile - свойства стратегии команды по Аксельроду в дилемме
// заключённого, посчитанные по транскриптам её матчей. доля nil - в матчах
// не было ни одной подходящей ситуации (соперник ни разу не предал и т.п.)
type StrategyProfile struct {
	TeamID   uuid.UUID `json:"team_id"`
	TeamName string    `json:"team_name"`
	// матчей с транскриптом
	Matches int `json:"matches"`
	// доля ходов-сотрудничеств
	Cooperation *float64 `json:"cooperation"`
	// доля матчей, где команда не предала первой
	Niceness *float64 `json:"niceness"`
	// доля ответов предательством на предательство соперника
	Retaliation *float64 `json:"retaliation"`
	// доля возвратов к сотрудничеству, когда соперник вернулся к нему после предательства
	Forgiveness *float64 `json:"forgiveness"`
	// доля ответов предательством на неспровоцированное предательство
	// соперника: на прошлом ходу команда сотрудничала
	Provocability *float64 `json:"provocability"`
}

// StrategyTally копит по матчам счётчики для StrategyProfile:
// числитель и знаменатель каждой доли
type StrategyTally struct {
	matches                                             int
	coop, nice, retaliation, forgiveness, provocability [2]int
}

// Add учитывает один матч с точки зрения стороны own
func (s *StrategyTally) Add(own, opp []int) {
	n := min(len(own), len(opp))
	if n == 0 {
		return
	}
	s.matches++

	// добрая стратегия не предаёт первой; предательство в один ход с
	// соперником тоже первое
	firstOwn, firstOpp := n, n
	for t := n - 1; t >= 0; t-- {
		if own[t] == MoveDefect {
			firstOwn = t
		}
		if opp[t] == MoveDefect {
			firstOpp = t
		}
	}
	s.nice[1]++
	if firstOwn == n || firstOwn > firstOpp {
		s.nice[0]++
	}

	for t := range n {
		s.coop[1]++
		if own[t] == MoveCooperate {
			s.coop[0]++
		}
		if t+1 == n {
			break
		}
		next := own[t+1]
		if opp[t] == MoveDefect {
			s.retaliation[1]++
			if next == MoveDefect {
				s.retaliation[0]++
			}
			// ход соперника в t - ответ на ход команды в t-1
			if t == 0 || own[t-1] == MoveCooperate {
				s.provocability[1]++
				if next == MoveDefect {
					s.provocability[0]++
				}
			}
		} else if t > 0 && opp[t-1] == MoveDefect {
			s.forgiveness[1]++
			if next == MoveCooperate {
				s.forgiveness[0]++
			}
		}
	}
}

// Profile переводит счётчики в доли
func (s *StrategyTally) Profile(teamID uuid.UUID, teamName string) *StrategyProfile {
	return &StrategyProfile{
		TeamID:        teamID,
		TeamName:      teamName,
		Matches:       s.matches,
		Cooperation:   ratio(s.coop),
		Niceness:      ratio(s.nice),
		Retaliation:   ratio(s.retaliation),
		Forgiveness:   ratio(s.forgiveness),
		Provocability: ratio(s.provocability),
	}
}

func ratio(c [2]int) *float64 {
	if c[1] == 0 {
		return nil
	}
	r := float64(c[0]) / float64(c[1])
	return &r
}
