package rating

import (
	"math"
)

// EloCalculator - калькулятор рейтинга по системе ELO
type EloCalculator struct {
	kFactor int // K-фактор определяет скорость изменения рейтинга
}

// NewEloCalculator создаёт новый калькулятор ELO
func NewEloCalculator(kFactor int) *EloCalculator {
	return &EloCalculator{
		kFactor: kFactor,
	}
}

// NewDefaultEloCalculator создаёт калькулятор с дефолтным K-фактором
func NewDefaultEloCalculator() *EloCalculator {
	return NewEloCalculator(32)
}

// CalculateExpectedScore вычисляет ожидаемый результат для игрока A против B
// Возвращает значение от 0 до 1 (вероятность победы)
func (ec *EloCalculator) CalculateExpectedScore(ratingA, ratingB int) float64 {
	return 1.0 / (1.0 + math.Pow(10, float64(ratingB-ratingA)/400.0))
}

// CalculateNewRating вычисляет новый рейтинг после матча
// score: 1.0 = победа, 0.5 = ничья, 0.0 = поражение
func (ec *EloCalculator) CalculateNewRating(currentRating, opponentRating int, score float64) int {
	expectedScore := ec.CalculateExpectedScore(currentRating, opponentRating)
	change := float64(ec.kFactor) * (score - expectedScore)
	newRating := float64(currentRating) + change

	return int(math.Round(newRating))
}

// CalculateRatingChange вычисляет изменение рейтинга
func (ec *EloCalculator) CalculateRatingChange(currentRating, opponentRating int, score float64) int {
	newRating := ec.CalculateNewRating(currentRating, opponentRating, score)
	return newRating - currentRating
}

// ProcessMatch обрабатывает результат матча и возвращает новые рейтинги обоих игроков
func (ec *EloCalculator) ProcessMatch(rating1, rating2 int, winner int) (newRating1, newRating2, change1, change2 int) {
	var score1, score2 float64

	switch winner {
	case 1: // Победа первого игрока
		score1 = 1.0
		score2 = 0.0
	case 2: // Победа второго игрока
		score1 = 0.0
		score2 = 1.0
	default: // Ничья (winner = 0)
		score1 = 0.5
		score2 = 0.5
	}

	newRating1 = ec.CalculateNewRating(rating1, rating2, score1)
	newRating2 = ec.CalculateNewRating(rating2, rating1, score2)

	change1 = newRating1 - rating1
	change2 = newRating2 - rating2

	return
}

// GetKFactor возвращает текущий K-фактор
func (ec *EloCalculator) GetKFactor() int {
	return ec.kFactor
}

// SetKFactor устанавливает K-фактор
// Типичные значения:
// - 32 для новых игроков
// - 24 для игроков среднего уровня
// - 16 для сильных игроков
func (ec *EloCalculator) SetKFactor(kFactor int) {
	ec.kFactor = kFactor
}

// GetAdaptiveKFactor возвращает адаптивный K-фактор на основе рейтинга
// Более высокий K для низких рейтингов (быстрее растут)
// Более низкий K для высоких рейтингов (стабильнее)
func GetAdaptiveKFactor(rating int) int {
	switch {
	case rating < 1200:
		return 40 // Новички растут быстро
	case rating < 1800:
		return 32 // Обычная скорость
	case rating < 2400:
		return 24 // Замедленный рост
	default:
		return 16 // Топ игроки - очень стабильно
	}
}
