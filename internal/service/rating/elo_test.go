package rating

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEloCalculator(t *testing.T) {
	// обычный конструктор с заданным k
	calc := NewEloCalculator(32)
	assert.NotNil(t, calc)
	assert.Equal(t, 32, calc.kFactor)

	// дефолтный тоже должен дать 32
	def := NewDefaultEloCalculator()
	assert.Equal(t, 32, def.kFactor)
}

func TestEloCalculator_CalculateExpectedScore_EqualRatings(t *testing.T) {
	calc := NewDefaultEloCalculator()

	// равные рейтинги дают ровно 0.5
	expected := calc.CalculateExpectedScore(1500, 1500)
	assert.InDelta(t, 0.5, expected, 0.001)
}

func TestEloCalculator_CalculateExpectedScore_Asymmetric(t *testing.T) {
	calc := NewDefaultEloCalculator()

	// у кого рейтинг выше - ожидание больше 0.5, у кого ниже - меньше
	higher := calc.CalculateExpectedScore(1700, 1500)
	assert.Greater(t, higher, 0.5)
	assert.Less(t, higher, 1.0)

	lower := calc.CalculateExpectedScore(1300, 1500)
	assert.Less(t, lower, 0.5)
	assert.Greater(t, lower, 0.0)
}

func TestEloCalculator_CalculateExpectedScore_400Difference(t *testing.T) {
	calc := NewDefaultEloCalculator()

	// разница в 400 очков - фаворит имеет ~0.9
	expectedHigher := calc.CalculateExpectedScore(1900, 1500)
	expectedLower := calc.CalculateExpectedScore(1500, 1900)

	// два ожидания в сумме дают 1
	assert.InDelta(t, 1.0, expectedHigher+expectedLower, 0.001)
	assert.InDelta(t, 0.909, expectedHigher, 0.01)
}

func TestEloCalculator_CalculateNewRating_Win(t *testing.T) {
	calc := NewEloCalculator(32)

	// победа над равным - плюс 16 (половина k)
	newRating := calc.CalculateNewRating(1500, 1500, 1.0)
	assert.Equal(t, 1516, newRating)
}

func TestEloCalculator_CalculateNewRating_Loss(t *testing.T) {
	calc := NewEloCalculator(32)

	// поражение от равного - минус 16
	newRating := calc.CalculateNewRating(1500, 1500, 0.0)
	assert.Equal(t, 1484, newRating)
}

func TestEloCalculator_CalculateNewRating_Draw(t *testing.T) {
	calc := NewEloCalculator(32)

	// ничья с равным - рейтимг не меняется
	newRating := calc.CalculateNewRating(1500, 1500, 0.5)
	assert.Equal(t, 1500, newRating)
}

func TestEloCalculator_CalculateNewRating_UpsetWin(t *testing.T) {
	calc := NewEloCalculator(32)

	// слабый обыграл сильного - очков больше половины k
	newRating := calc.CalculateNewRating(1300, 1700, 1.0)
	assert.Greater(t, newRating-1300, 16)
}

func TestEloCalculator_CalculateNewRating_ExpectedWin(t *testing.T) {
	calc := NewEloCalculator(32)

	// сильный обыграл слабого (ожидаемо) - очков меньше половины k
	newRating := calc.CalculateNewRating(1700, 1300, 1.0)
	assert.Less(t, newRating-1700, 16)
}

func TestEloCalculator_CalculateRatingChange(t *testing.T) {
	calc := NewEloCalculator(32)

	// изменение за победу и за поражение над равным
	assert.Equal(t, 16, calc.CalculateRatingChange(1500, 1500, 1.0))
	assert.Equal(t, -16, calc.CalculateRatingChange(1500, 1500, 0.0))
}

func TestEloCalculator_ProcessMatch_Player1Wins(t *testing.T) {
	calc := NewEloCalculator(32)

	newRating1, newRating2, change1, change2 := calc.ProcessMatch(1500, 1500, 1)
	assert.Equal(t, 1516, newRating1)
	assert.Equal(t, 1484, newRating2)
	assert.Equal(t, 16, change1)
	assert.Equal(t, -16, change2)
}

func TestEloCalculator_ProcessMatch_Player2Wins(t *testing.T) {
	calc := NewEloCalculator(32)

	newRating1, newRating2, change1, change2 := calc.ProcessMatch(1500, 1500, 2)
	assert.Equal(t, 1484, newRating1)
	assert.Equal(t, 1516, newRating2)
	assert.Equal(t, -16, change1)
	assert.Equal(t, 16, change2)
}

func TestEloCalculator_ProcessMatch_Draw(t *testing.T) {
	calc := NewEloCalculator(32)

	newRating1, newRating2, change1, change2 := calc.ProcessMatch(1500, 1500, 0)
	assert.Equal(t, 1500, newRating1)
	assert.Equal(t, 1500, newRating2)
	assert.Equal(t, 0, change1)
	assert.Equal(t, 0, change2)
}

func TestEloCalculator_ProcessMatch_ZeroSum(t *testing.T) {
	calc := NewEloCalculator(32)

	// изменения рейтинга зеркальны с точностью до округления
	_, _, change1, change2 := calc.ProcessMatch(1500, 1600, 1)
	assert.InDelta(t, -change2, change1, 1)
}

func TestEloCalculator_ProcessMatch_DifferentRatings(t *testing.T) {
	calc := NewEloCalculator(32)

	// фаворит выигрывает - изменения маленькие
	_, _, change1, change2 := calc.ProcessMatch(1700, 1300, 1)
	assert.Greater(t, change1, 0)
	assert.Less(t, change2, 0)
	assert.Less(t, change1, 16)
}

func TestEloCalculator_ProcessMatch_Upset(t *testing.T) {
	calc := NewEloCalculator(32)

	// сенсация: слабый обыгрывает сильного - изменения большие
	newRating1, _, change1, change2 := calc.ProcessMatch(1300, 1700, 1)
	assert.Greater(t, change1, 16)
	assert.Less(t, change2, -16)
	assert.Greater(t, newRating1, 1316)
}

func TestEloCalculator_GetSetKFactor(t *testing.T) {
	calc := NewEloCalculator(24)
	assert.Equal(t, 24, calc.GetKFactor())

	calc.SetKFactor(16)
	assert.Equal(t, 16, calc.GetKFactor())
}

func TestEloCalculator_RealisticScenario(t *testing.T) {
	calc := NewDefaultEloCalculator()

	player1Rating := 1500
	player2Rating := 1500

	// первый выигрывает 3 из 5
	results := []int{1, 1, 2, 1, 2}
	for _, winner := range results {
		player1Rating, player2Rating, _, _ = calc.ProcessMatch(player1Rating, player2Rating, winner)
	}

	// после большего числа побед первый должен быть выше
	assert.Greater(t, player1Rating, player2Rating)
}

func TestEloCalculator_Precision(t *testing.T) {
	calc := NewEloCalculator(32)

	// 1500 против 1532 - чуть меньше 0.5, но валидная вероятность
	expected := calc.CalculateExpectedScore(1500, 1532)
	assert.Greater(t, expected, 0.0)
	assert.Less(t, expected, 0.5)
	assert.False(t, math.IsNaN(expected))
	assert.False(t, math.IsInf(expected, 0))
}

// --- граничные случаи ---

func TestEloCalculator_ZeroRatingsForBothPlayers(t *testing.T) {
	calc := NewDefaultEloCalculator()

	t.Run("Win", func(t *testing.T) {
		// оба на нуле, ожидание 0.5
		newRating := calc.CalculateNewRating(0, 0, 1.0)
		assert.Equal(t, 16, newRating, "победитель получает k/2 = 16 от нуля")
	})

	t.Run("Loss", func(t *testing.T) {
		newRating := calc.CalculateNewRating(0, 0, 0.0)
		assert.Equal(t, 0, newRating, "проигравший упирается в пол 0")
	})

	t.Run("Draw", func(t *testing.T) {
		newRating := calc.CalculateNewRating(0, 0, 0.5)
		assert.Equal(t, 0, newRating, "ничья на нуле рейтинг не меняет")
	})

	t.Run("ProcessMatch", func(t *testing.T) {
		newR1, newR2, c1, c2 := calc.ProcessMatch(0, 0, 1)
		assert.Equal(t, 16, newR1)
		assert.Equal(t, 0, newR2, "проигравший упирается в пол 0")
		assert.Equal(t, 16, c1)
		assert.Equal(t, 0, c2)
	})

	t.Run("ExpectedScore", func(t *testing.T) {
		expected := calc.CalculateExpectedScore(0, 0)
		assert.InDelta(t, 0.5, expected, 0.001)
	})
}

func TestEloCalculator_NegativeRatings(t *testing.T) {
	calc := NewDefaultEloCalculator()

	t.Run("NegativeVsPositive_Win", func(t *testing.T) {
		// -100 против 100: без пола было бы ~-76, но пол зажимает в 0
		newRating := calc.CalculateNewRating(-100, 100, 1.0)
		assert.Equal(t, 0, newRating, "зажимается в пол 0")
	})

	t.Run("NegativeVsPositive_Loss", func(t *testing.T) {
		newRating := calc.CalculateNewRating(-100, 100, 0.0)
		assert.Equal(t, 0, newRating, "зажимается в пол 0")
	})

	t.Run("BothNegative", func(t *testing.T) {
		// -200 против -200 - как равные, ожидание 0.5
		expected := calc.CalculateExpectedScore(-200, -200)
		assert.InDelta(t, 0.5, expected, 0.001)

		// -200 + 16 = -184, но пол зажимает в 0
		newRating := calc.CalculateNewRating(-200, -200, 1.0)
		assert.Equal(t, 0, newRating, "зажимается в пол 0")
	})

	t.Run("Symmetry", func(t *testing.T) {
		e1 := calc.CalculateExpectedScore(-100, 100)
		e2 := calc.CalculateExpectedScore(100, -100)
		assert.InDelta(t, 1.0, e1+e2, 0.001)
	})

	t.Run("ProcessMatch", func(t *testing.T) {
		newR1, newR2, c1, _ := calc.ProcessMatch(-100, 100, 1)
		assert.Equal(t, 0, newR1, "победитель зажимается в пол 0")
		assert.Less(t, newR2, 100, "проигравший теряет рейтинг")
		assert.Equal(t, 100, c1, "изменение отражает зажим с -100 до 0")
	})
}

func TestEloCalculator_ExtremeKFactors(t *testing.T) {
	t.Run("K=1_MinimalChange", func(t *testing.T) {
		calc := NewEloCalculator(1)

		// победа над равным: change = 1 * (1.0 - 0.5) = 0.5, round(1500.5) = 1501
		newRating := calc.CalculateNewRating(1500, 1500, 1.0)
		assert.Equal(t, 1501, newRating)

		// поражение: change = -0.5, round(1499.5) = 1500 (округление от нуля)
		newRating = calc.CalculateNewRating(1500, 1500, 0.0)
		assert.Equal(t, 1500, newRating, "1499.5 округляется до 1500")
	})

	t.Run("K=128_LargeChange", func(t *testing.T) {
		calc := NewEloCalculator(128)

		// победа над равным: change = 128 * 0.5 = 64
		newRating := calc.CalculateNewRating(1500, 1500, 1.0)
		assert.Equal(t, 1564, newRating)

		// поражение: change = -64
		newRating = calc.CalculateNewRating(1500, 1500, 0.0)
		assert.Equal(t, 1436, newRating)
	})

	t.Run("K=128_UpsetWin", func(t *testing.T) {
		calc := NewEloCalculator(128)

		// большая сенсация с большим k: изменение близко к k
		newRating := calc.CalculateNewRating(1000, 2000, 1.0)
		change := newRating - 1000
		assert.Greater(t, change, 100)
		assert.LessOrEqual(t, change, 128)
	})

	t.Run("K=1_ProcessMatch", func(t *testing.T) {
		calc := NewEloCalculator(1)

		// первый выигрывает: p1 1500.5 -> 1501, p2 1499.5 -> 1500
		newR1, newR2, c1, c2 := calc.ProcessMatch(1500, 1500, 1)
		assert.Equal(t, 1501, newR1)
		assert.Equal(t, 1500, newR2, "1499.5 округляется до 1500")
		assert.Equal(t, 1, c1)
		assert.Equal(t, 0, c2, "артефакт округления: потеря 0.5 даёт 0")
	})
}

func TestEloCalculator_ZeroKFactor(t *testing.T) {
	calc := NewEloCalculator(0)

	t.Run("NoRatingChange", func(t *testing.T) {
		// при k=0 рейтинг не двигается ни при каком исходе
		assert.Equal(t, 1500, calc.CalculateNewRating(1500, 1500, 1.0))
		assert.Equal(t, 1500, calc.CalculateNewRating(1500, 1500, 0.0))
		assert.Equal(t, 1500, calc.CalculateNewRating(1500, 1500, 0.5))
		// даже при разных рейтингах
		assert.Equal(t, 1200, calc.CalculateNewRating(1200, 1800, 1.0))
	})

	t.Run("ProcessMatch", func(t *testing.T) {
		newR1, newR2, c1, c2 := calc.ProcessMatch(1500, 1600, 1)
		assert.Equal(t, 1500, newR1)
		assert.Equal(t, 1600, newR2)
		assert.Equal(t, 0, c1)
		assert.Equal(t, 0, c2)
	})

	t.Run("RatingChangeIsZero", func(t *testing.T) {
		assert.Equal(t, 0, calc.CalculateRatingChange(1500, 1000, 1.0))
	})
}

func TestEloCalculator_VeryLargeRatingDifference(t *testing.T) {
	calc := NewDefaultEloCalculator()

	t.Run("2800_vs_400_ExpectedScores", func(t *testing.T) {
		expectedHigh := calc.CalculateExpectedScore(2800, 400)
		expectedLow := calc.CalculateExpectedScore(400, 2800)

		// разрыв 2400 очков: фаворит почти 1.0, но не ровно
		assert.Greater(t, expectedHigh, 0.999)
		assert.Less(t, expectedHigh, 1.0, "ожидание никогда не доходит до 1.0")
		assert.Greater(t, expectedLow, 0.0, "ожидание никогда не доходит до 0.0")
		assert.Less(t, expectedLow, 0.001)

		// в сумме всё равно 1
		assert.InDelta(t, 1.0, expectedHigh+expectedLow, 1e-10)
	})

	t.Run("2800_vs_400_HigherWins", func(t *testing.T) {
		// ожидаемый исход - почти нулевое изменение
		newRating := calc.CalculateNewRating(2800, 400, 1.0)
		change := newRating - 2800
		assert.GreaterOrEqual(t, change, 0, "победа не должна снижать рейтинг")
		assert.LessOrEqual(t, change, 1, "изменение минимально при огромном разрыве")
	})

	t.Run("2800_vs_400_LowerWins_Upset", func(t *testing.T) {
		// огромная сенсация: слабый выигрывает
		newRating := calc.CalculateNewRating(400, 2800, 1.0)
		change := newRating - 400
		assert.Greater(t, change, 30, "сенсация даёт почти полный k")
		assert.LessOrEqual(t, change, 32)
	})

	t.Run("2800_vs_400_ProcessMatch_ZeroSum", func(t *testing.T) {
		_, _, c1, c2 := calc.ProcessMatch(2800, 400, 1)
		// даже при экстремальной разнице изменения примерно зеркальны
		assert.InDelta(t, -c2, c1, 1)
	})

	t.Run("2800_vs_400_Draw", func(t *testing.T) {
		// ничья сильно на руку слабому
		newR1, newR2, c1, c2 := calc.ProcessMatch(2800, 400, 0)
		assert.Less(t, c1, 0, "фаворит теряет рейтинг на ничье")
		assert.Greater(t, c2, 0, "слабый набирает рейтинг на ничье")
		assert.Less(t, newR1, 2800)
		assert.Greater(t, newR2, 400)
	})

	t.Run("NoNaN_NoInf", func(t *testing.T) {
		expected := calc.CalculateExpectedScore(2800, 400)
		assert.False(t, math.IsNaN(expected))
		assert.False(t, math.IsInf(expected, 0))

		expected = calc.CalculateExpectedScore(400, 2800)
		assert.False(t, math.IsNaN(expected))
		assert.False(t, math.IsInf(expected, 0))
	})
}

func BenchmarkEloCalculator_CalculateExpectedScore(b *testing.B) {
	calc := NewDefaultEloCalculator()

	for i := 0; i < b.N; i++ {
		calc.CalculateExpectedScore(1500, 1600)
	}
}

func BenchmarkEloCalculator_ProcessMatch(b *testing.B) {
	calc := NewDefaultEloCalculator()

	for i := 0; i < b.N; i++ {
		calc.ProcessMatch(1500, 1600, 1)
	}
}
