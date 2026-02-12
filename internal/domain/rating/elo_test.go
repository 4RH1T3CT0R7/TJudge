package rating

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEloCalculator(t *testing.T) {
	calc := NewEloCalculator(32)

	assert.NotNil(t, calc)
	assert.Equal(t, 32, calc.kFactor)
}

func TestNewDefaultEloCalculator(t *testing.T) {
	calc := NewDefaultEloCalculator()

	assert.NotNil(t, calc)
	assert.Equal(t, 32, calc.kFactor)
}

func TestEloCalculator_CalculateExpectedScore_EqualRatings(t *testing.T) {
	calc := NewDefaultEloCalculator()

	// Equal ratings should give 0.5 expected score
	expected := calc.CalculateExpectedScore(1500, 1500)

	assert.InDelta(t, 0.5, expected, 0.001)
}

func TestEloCalculator_CalculateExpectedScore_HigherRating(t *testing.T) {
	calc := NewDefaultEloCalculator()

	// Higher rated player should have > 0.5 expected score
	expected := calc.CalculateExpectedScore(1700, 1500)

	assert.Greater(t, expected, 0.5)
	assert.Less(t, expected, 1.0)
}

func TestEloCalculator_CalculateExpectedScore_LowerRating(t *testing.T) {
	calc := NewDefaultEloCalculator()

	// Lower rated player should have < 0.5 expected score
	expected := calc.CalculateExpectedScore(1300, 1500)

	assert.Less(t, expected, 0.5)
	assert.Greater(t, expected, 0.0)
}

func TestEloCalculator_CalculateExpectedScore_400Difference(t *testing.T) {
	calc := NewDefaultEloCalculator()

	// 400 point difference should give ~0.9 expected score for higher rated
	expectedHigher := calc.CalculateExpectedScore(1900, 1500)
	expectedLower := calc.CalculateExpectedScore(1500, 1900)

	// Expected scores should be complementary (sum to 1)
	assert.InDelta(t, 1.0, expectedHigher+expectedLower, 0.001)

	// Higher rated should have ~0.9
	assert.InDelta(t, 0.909, expectedHigher, 0.01)
}

func TestEloCalculator_CalculateExpectedScore_Symmetry(t *testing.T) {
	calc := NewDefaultEloCalculator()

	// Expected scores should sum to 1
	e1 := calc.CalculateExpectedScore(1600, 1400)
	e2 := calc.CalculateExpectedScore(1400, 1600)

	assert.InDelta(t, 1.0, e1+e2, 0.001)
}

func TestEloCalculator_CalculateNewRating_Win(t *testing.T) {
	calc := NewEloCalculator(32)

	// Win against equal opponent
	newRating := calc.CalculateNewRating(1500, 1500, 1.0)

	// Should gain 16 points (K/2 for expected 0.5)
	assert.Equal(t, 1516, newRating)
}

func TestEloCalculator_CalculateNewRating_Loss(t *testing.T) {
	calc := NewEloCalculator(32)

	// Loss against equal opponent
	newRating := calc.CalculateNewRating(1500, 1500, 0.0)

	// Should lose 16 points
	assert.Equal(t, 1484, newRating)
}

func TestEloCalculator_CalculateNewRating_Draw(t *testing.T) {
	calc := NewEloCalculator(32)

	// Draw against equal opponent
	newRating := calc.CalculateNewRating(1500, 1500, 0.5)

	// Should not change rating
	assert.Equal(t, 1500, newRating)
}

func TestEloCalculator_CalculateNewRating_UpsetWin(t *testing.T) {
	calc := NewEloCalculator(32)

	// Lower rated player wins against higher rated
	newRating := calc.CalculateNewRating(1300, 1700, 1.0)

	// Should gain more points for upset
	change := newRating - 1300
	assert.Greater(t, change, 16) // More than K/2
}

func TestEloCalculator_CalculateNewRating_ExpectedWin(t *testing.T) {
	calc := NewEloCalculator(32)

	// Higher rated player wins (expected)
	newRating := calc.CalculateNewRating(1700, 1300, 1.0)

	// Should gain fewer points for expected win
	change := newRating - 1700
	assert.Less(t, change, 16) // Less than K/2
}

func TestEloCalculator_CalculateRatingChange(t *testing.T) {
	calc := NewEloCalculator(32)

	change := calc.CalculateRatingChange(1500, 1500, 1.0)

	assert.Equal(t, 16, change)
}

func TestEloCalculator_CalculateRatingChange_Negative(t *testing.T) {
	calc := NewEloCalculator(32)

	change := calc.CalculateRatingChange(1500, 1500, 0.0)

	assert.Equal(t, -16, change)
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

	// Changes should be zero-sum
	_, _, change1, change2 := calc.ProcessMatch(1500, 1600, 1)

	// Рейтинги должны меняться примерно на одну величину (но в разные стороны)
	// Из-за округления может быть небольшая разница
	assert.InDelta(t, -change2, change1, 1)
}

func TestEloCalculator_ProcessMatch_DifferentRatings(t *testing.T) {
	calc := NewEloCalculator(32)

	// Higher rated wins (expected)
	_, _, change1, change2 := calc.ProcessMatch(1700, 1300, 1)

	// Changes should be smaller due to expected outcome
	assert.Greater(t, change1, 0)
	assert.Less(t, change2, 0)
	assert.Less(t, change1, 16) // Less than half K-factor
}

func TestEloCalculator_ProcessMatch_Upset(t *testing.T) {
	calc := NewEloCalculator(32)

	// Lower rated wins (upset)
	newRating1, _, change1, change2 := calc.ProcessMatch(1300, 1700, 1)

	// Changes should be larger due to upset
	assert.Greater(t, change1, 16) // More than half K-factor
	assert.Less(t, change2, -16)
	assert.Greater(t, newRating1, 1316)
}

func TestEloCalculator_GetKFactor(t *testing.T) {
	calc := NewEloCalculator(24)

	assert.Equal(t, 24, calc.GetKFactor())
}

func TestEloCalculator_SetKFactor(t *testing.T) {
	calc := NewEloCalculator(32)
	calc.SetKFactor(16)

	assert.Equal(t, 16, calc.GetKFactor())
}

func TestGetAdaptiveKFactor_Beginner(t *testing.T) {
	kFactor := GetAdaptiveKFactor(1000)
	assert.Equal(t, 40, kFactor)
}

func TestGetAdaptiveKFactor_Intermediate(t *testing.T) {
	kFactor := GetAdaptiveKFactor(1500)
	assert.Equal(t, 32, kFactor)
}

func TestGetAdaptiveKFactor_Advanced(t *testing.T) {
	kFactor := GetAdaptiveKFactor(2000)
	assert.Equal(t, 24, kFactor)
}

func TestGetAdaptiveKFactor_Expert(t *testing.T) {
	kFactor := GetAdaptiveKFactor(2500)
	assert.Equal(t, 16, kFactor)
}

func TestGetAdaptiveKFactor_Boundaries(t *testing.T) {
	tests := []struct {
		rating   int
		expected int
	}{
		{1199, 40},
		{1200, 32},
		{1799, 32},
		{1800, 24},
		{2399, 24},
		{2400, 16},
		{3000, 16},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			result := GetAdaptiveKFactor(tc.rating)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEloCalculator_RealisticScenario(t *testing.T) {
	calc := NewDefaultEloCalculator()

	// Start with default rating
	player1Rating := 1500
	player2Rating := 1500

	// Player 1 wins 3 games, loses 2
	results := []int{1, 1, 2, 1, 2}

	for _, winner := range results {
		player1Rating, player2Rating, _, _ = calc.ProcessMatch(player1Rating, player2Rating, winner)
	}

	// Player 1 should be higher rated after winning more
	assert.Greater(t, player1Rating, player2Rating)
}

func TestEloCalculator_RatingFloor(t *testing.T) {
	calc := NewEloCalculator(32)

	// Very low rated player loses
	newRating := calc.CalculateNewRating(100, 1500, 0.0)

	// Rating can go very low (no floor in basic ELO)
	require.NotNil(t, newRating)
	// In production, you might want to implement a rating floor
}

func TestEloCalculator_LargeRatingDifference(t *testing.T) {
	calc := NewDefaultEloCalculator()

	// Huge rating difference
	expected := calc.CalculateExpectedScore(3000, 1000)

	// Should be very close to 1.0
	assert.Greater(t, expected, 0.99)
	assert.Less(t, expected, 1.0)
}

func TestEloCalculator_NegativeRatingDifference(t *testing.T) {
	calc := NewDefaultEloCalculator()

	expected := calc.CalculateExpectedScore(1000, 3000)

	// Should be very close to 0.0
	assert.Greater(t, expected, 0.0)
	assert.Less(t, expected, 0.01)
}

func TestEloCalculator_Precision(t *testing.T) {
	calc := NewEloCalculator(32)

	// Test that ratings are properly rounded
	// Rating 1500 vs 1532 should give slightly less than 0.5 expected
	expected := calc.CalculateExpectedScore(1500, 1532)

	// Verify it's a valid probability
	assert.Greater(t, expected, 0.0)
	assert.Less(t, expected, 0.5)
	assert.False(t, math.IsNaN(expected))
	assert.False(t, math.IsInf(expected, 0))
}

// --- Edge-case tests ---

func TestEloCalculator_ZeroRatingsForBothPlayers(t *testing.T) {
	calc := NewDefaultEloCalculator()

	t.Run("Win", func(t *testing.T) {
		// Both at 0 rating, equal expected score of 0.5
		newRating := calc.CalculateNewRating(0, 0, 1.0)
		assert.Equal(t, 16, newRating, "winner should gain K/2 = 16 from 0")
	})

	t.Run("Loss", func(t *testing.T) {
		newRating := calc.CalculateNewRating(0, 0, 0.0)
		assert.Equal(t, -16, newRating, "loser should drop to -16 from 0")
	})

	t.Run("Draw", func(t *testing.T) {
		newRating := calc.CalculateNewRating(0, 0, 0.5)
		assert.Equal(t, 0, newRating, "draw should not change rating at 0")
	})

	t.Run("ProcessMatch", func(t *testing.T) {
		newR1, newR2, c1, c2 := calc.ProcessMatch(0, 0, 1)
		assert.Equal(t, 16, newR1)
		assert.Equal(t, -16, newR2)
		assert.Equal(t, 16, c1)
		assert.Equal(t, -16, c2)
	})

	t.Run("ExpectedScore", func(t *testing.T) {
		expected := calc.CalculateExpectedScore(0, 0)
		assert.InDelta(t, 0.5, expected, 0.001)
	})
}

func TestEloCalculator_NegativeRatings(t *testing.T) {
	calc := NewDefaultEloCalculator()

	t.Run("NegativeVsPositive_Win", func(t *testing.T) {
		// -100 vs 100: difference of 200, so expected for -100 is low (~0.24)
		newRating := calc.CalculateNewRating(-100, 100, 1.0)
		change := newRating - (-100)

		// Upset win, should gain more than K/2
		assert.Greater(t, change, 16)
		assert.LessOrEqual(t, change, 32)
	})

	t.Run("NegativeVsPositive_Loss", func(t *testing.T) {
		// -100 vs 100: expected loss, smaller penalty
		newRating := calc.CalculateNewRating(-100, 100, 0.0)
		change := newRating - (-100)

		// Expected loss, should lose less than K/2
		assert.Less(t, change, 0)
		assert.Greater(t, change, -16)
	})

	t.Run("BothNegative", func(t *testing.T) {
		// -200 vs -200: same as equal ratings, expected = 0.5
		expected := calc.CalculateExpectedScore(-200, -200)
		assert.InDelta(t, 0.5, expected, 0.001)

		newRating := calc.CalculateNewRating(-200, -200, 1.0)
		assert.Equal(t, -184, newRating, "should gain K/2 = 16")
	})

	t.Run("NegativeVsPositive_Symmetry", func(t *testing.T) {
		e1 := calc.CalculateExpectedScore(-100, 100)
		e2 := calc.CalculateExpectedScore(100, -100)
		assert.InDelta(t, 1.0, e1+e2, 0.001)
	})

	t.Run("NegativeVsPositive_ProcessMatch", func(t *testing.T) {
		newR1, newR2, c1, c2 := calc.ProcessMatch(-100, 100, 1)
		assert.Greater(t, newR1, -100, "winner rating should increase")
		assert.Less(t, newR2, 100, "loser rating should decrease")
		assert.InDelta(t, -c2, c1, 1, "changes should be approximately zero-sum")
	})
}

func TestEloCalculator_ExtremeKFactors(t *testing.T) {
	t.Run("K=1_MinimalChange", func(t *testing.T) {
		calc := NewEloCalculator(1)

		// Win against equal opponent: change = 1 * (1.0 - 0.5) = 0.5
		// newRating = 1500 + 0.5 = 1500.5, math.Round(1500.5) = 1501
		newRating := calc.CalculateNewRating(1500, 1500, 1.0)
		assert.Equal(t, 1501, newRating)

		// Loss against equal opponent: change = 1 * (0.0 - 0.5) = -0.5
		// newRating = 1500 - 0.5 = 1499.5, math.Round(1499.5) = 1500 (half away from zero)
		newRating = calc.CalculateNewRating(1500, 1500, 0.0)
		assert.Equal(t, 1500, newRating, "1499.5 rounds to 1500 (half away from zero)")
	})

	t.Run("K=128_LargeChange", func(t *testing.T) {
		calc := NewEloCalculator(128)

		// Win against equal opponent: change = 128 * (1.0 - 0.5) = 64
		newRating := calc.CalculateNewRating(1500, 1500, 1.0)
		assert.Equal(t, 1564, newRating)

		// Loss against equal opponent: change = 128 * (0.0 - 0.5) = -64
		newRating = calc.CalculateNewRating(1500, 1500, 0.0)
		assert.Equal(t, 1436, newRating)
	})

	t.Run("K=128_UpsetWin", func(t *testing.T) {
		calc := NewEloCalculator(128)

		// Big upset with large K: rating change should be close to K
		newRating := calc.CalculateNewRating(1000, 2000, 1.0)
		change := newRating - 1000
		// Expected score for 1000 vs 2000 is very low, so change is close to K
		assert.Greater(t, change, 100)
		assert.LessOrEqual(t, change, 128)
	})

	t.Run("K=1_ProcessMatch", func(t *testing.T) {
		calc := NewEloCalculator(1)

		// Player 1 wins equal match: p1 gets 1500.5 -> 1501, p2 gets 1499.5 -> 1500
		// Due to math.Round half-away-from-zero rounding
		newR1, newR2, c1, c2 := calc.ProcessMatch(1500, 1500, 1)
		assert.Equal(t, 1501, newR1)
		assert.Equal(t, 1500, newR2, "1499.5 rounds to 1500 (half away from zero)")
		assert.Equal(t, 1, c1)
		assert.Equal(t, 0, c2, "rounding artifact: loss of 0.5 rounds to no change")
	})
}

func TestEloCalculator_CalculateExpectedScore_IdenticalRatings(t *testing.T) {
	calc := NewDefaultEloCalculator()

	tests := []struct {
		name   string
		rating int
	}{
		{"Zero", 0},
		{"Standard_1500", 1500},
		{"Low_100", 100},
		{"High_3000", 3000},
		{"Negative_-500", -500},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expected := calc.CalculateExpectedScore(tc.rating, tc.rating)
			// For any identical ratings, expected score must be exactly 0.5
			assert.Equal(t, 0.5, expected,
				"expected score should be exactly 0.5 for identical ratings of %d", tc.rating)
		})
	}
}

func TestEloCalculator_ZeroKFactor(t *testing.T) {
	calc := NewEloCalculator(0)

	t.Run("NoRatingChange_Win", func(t *testing.T) {
		newRating := calc.CalculateNewRating(1500, 1500, 1.0)
		assert.Equal(t, 1500, newRating, "K=0 should produce no rating change on win")
	})

	t.Run("NoRatingChange_Loss", func(t *testing.T) {
		newRating := calc.CalculateNewRating(1500, 1500, 0.0)
		assert.Equal(t, 1500, newRating, "K=0 should produce no rating change on loss")
	})

	t.Run("NoRatingChange_Draw", func(t *testing.T) {
		newRating := calc.CalculateNewRating(1500, 1500, 0.5)
		assert.Equal(t, 1500, newRating, "K=0 should produce no rating change on draw")
	})

	t.Run("NoRatingChange_UnequalRatings", func(t *testing.T) {
		newRating := calc.CalculateNewRating(1200, 1800, 1.0)
		assert.Equal(t, 1200, newRating, "K=0 should produce no rating change even with different ratings")
	})

	t.Run("ProcessMatch_NoChanges", func(t *testing.T) {
		newR1, newR2, c1, c2 := calc.ProcessMatch(1500, 1600, 1)
		assert.Equal(t, 1500, newR1)
		assert.Equal(t, 1600, newR2)
		assert.Equal(t, 0, c1)
		assert.Equal(t, 0, c2)
	})

	t.Run("RatingChange_IsZero", func(t *testing.T) {
		change := calc.CalculateRatingChange(1500, 1000, 1.0)
		assert.Equal(t, 0, change)
	})

	t.Run("GetKFactor", func(t *testing.T) {
		assert.Equal(t, 0, calc.GetKFactor())
	})
}

func TestEloCalculator_VeryLargeRatingDifference(t *testing.T) {
	calc := NewDefaultEloCalculator()

	t.Run("2800_vs_400_ExpectedScores", func(t *testing.T) {
		expectedHigh := calc.CalculateExpectedScore(2800, 400)
		expectedLow := calc.CalculateExpectedScore(400, 2800)

		// 2400 point gap: higher rated should have expected very close to 1.0
		assert.Greater(t, expectedHigh, 0.999)
		assert.Less(t, expectedHigh, 1.0, "expected score should never reach 1.0")
		assert.Greater(t, expectedLow, 0.0, "expected score should never reach 0.0")
		assert.Less(t, expectedLow, 0.001)

		// Complementary property must hold
		assert.InDelta(t, 1.0, expectedHigh+expectedLow, 1e-10)
	})

	t.Run("2800_vs_400_HigherWins", func(t *testing.T) {
		// Expected outcome: nearly zero change
		newRating := calc.CalculateNewRating(2800, 400, 1.0)
		change := newRating - 2800
		assert.GreaterOrEqual(t, change, 0, "winning should not decrease rating")
		assert.LessOrEqual(t, change, 1, "change should be minimal for expected win with huge gap")
	})

	t.Run("2800_vs_400_LowerWins_Upset", func(t *testing.T) {
		// Massive upset: lower rated wins
		newRating := calc.CalculateNewRating(400, 2800, 1.0)
		change := newRating - 400
		// With K=32, change should be close to K (nearly 32)
		assert.Greater(t, change, 30, "upset should give nearly full K-factor change")
		assert.LessOrEqual(t, change, 32)
	})

	t.Run("2800_vs_400_ProcessMatch_ZeroSum", func(t *testing.T) {
		_, _, c1, c2 := calc.ProcessMatch(2800, 400, 1)
		// Even with extreme differences, changes should be approximately zero-sum
		assert.InDelta(t, -c2, c1, 1)
	})

	t.Run("2800_vs_400_Draw", func(t *testing.T) {
		// Draw heavily favors the lower-rated player
		newR1, newR2, c1, c2 := calc.ProcessMatch(2800, 400, 0)
		assert.Less(t, c1, 0, "higher rated should lose rating on draw")
		assert.Greater(t, c2, 0, "lower rated should gain rating on draw")
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

func TestGetAdaptiveKFactor_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		rating   int
		expected int
	}{
		{"ZeroRating", 0, 40},
		{"NegativeRating", -500, 40},
		{"ExactlyAt1200", 1200, 32},
		{"ExactlyAt1800", 1800, 24},
		{"ExactlyAt2400", 2400, 16},
		{"VeryHighRating", 5000, 16},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := GetAdaptiveKFactor(tc.rating)
			assert.Equal(t, tc.expected, result)
		})
	}
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
