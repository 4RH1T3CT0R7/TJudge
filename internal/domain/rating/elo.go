package rating

import (
	"math"
)

// EloCalculator считает рейтинг по эло
type EloCalculator struct {
	kFactor int // насколько сильно меняется рейтинг за матч
}

func NewEloCalculator(kFactor int) *EloCalculator {
	return &EloCalculator{
		kFactor: kFactor,
	}
}

func NewDefaultEloCalculator() *EloCalculator {
	return NewEloCalculator(32)
}

// CalculateExpectedScore - ожидаемый результат A против B, от 0 до 1
func (ec *EloCalculator) CalculateExpectedScore(ratingA, ratingB int) float64 {
	return 1.0 / (1.0 + math.Pow(10, float64(ratingB-ratingA)/400.0))
}

// CalculateNewRating - новый рейтинг после матча.
// score: 1 победа, 0.5 ничья, 0 поражение
func (ec *EloCalculator) CalculateNewRating(currentRating, opponentRating int, score float64) int {
	expectedScore := ec.CalculateExpectedScore(currentRating, opponentRating)
	change := float64(ec.kFactor) * (score - expectedScore)
	newRating := float64(currentRating) + change

	// в минус рейтинг не пускаем
	if newRating < 0 {
		return 0
	}

	return int(math.Round(newRating))
}

func (ec *EloCalculator) CalculateRatingChange(currentRating, opponentRating int, score float64) int {
	newRating := ec.CalculateNewRating(currentRating, opponentRating, score)
	return newRating - currentRating
}

// ProcessMatch считает новые рейтинги обоих игроков по результату матча
func (ec *EloCalculator) ProcessMatch(rating1, rating2 int, winner int) (newRating1, newRating2, change1, change2 int) {
	var score1, score2 float64

	switch winner {
	case 1: // выиграл первый
		score1 = 1.0
		score2 = 0.0
	case 2: // выиграл второй
		score1 = 0.0
		score2 = 1.0
	default: // ничья (winner = 0)
		score1 = 0.5
		score2 = 0.5
	}

	newRating1 = ec.CalculateNewRating(rating1, rating2, score1)
	newRating2 = ec.CalculateNewRating(rating2, rating1, score2)

	change1 = newRating1 - rating1
	change2 = newRating2 - rating2

	return newRating1, newRating2, change1, change2
}

func (ec *EloCalculator) GetKFactor() int {
	return ec.kFactor
}

// SetKFactor - типичные значения: 32 новичкам, 24 середнякам, 16 сильным
func (ec *EloCalculator) SetKFactor(kFactor int) {
	ec.kFactor = kFactor
}
