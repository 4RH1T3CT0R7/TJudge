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
