package rating

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
)

// boundedRating is a custom type that generates ratings in [0, 3000].
type boundedRating int

func (boundedRating) Generate(r *rand.Rand, _ int) reflect.Value {
	return reflect.ValueOf(boundedRating(r.Intn(3001))) // [0, 3000]
}

// boundedWinner generates winner values in {0, 1, 2}.
type boundedWinner int

func (boundedWinner) Generate(r *rand.Rand, _ int) reflect.Value {
	return reflect.ValueOf(boundedWinner(r.Intn(3))) // 0, 1, or 2
}

var quickCfg = &quick.Config{MaxCount: 1000}

// TestPropertyZeroSum verifies that for any two ratings in [0, 3000] and any
// winner, the sum of rating changes is approximately zero (within +/-1 due to
// rounding), EXCEPT when the rating floor at 0 clips a result.
func TestPropertyZeroSum(t *testing.T) {
	calc := NewDefaultEloCalculator()

	f := func(r1, r2 boundedRating, w boundedWinner) bool {
		rating1, rating2, winner := int(r1), int(r2), int(w)

		newR1, newR2, change1, change2 := calc.ProcessMatch(rating1, rating2, winner)

		// Verify changes are consistent with new ratings.
		if newR1 != rating1+change1 || newR2 != rating2+change2 {
			t.Logf("inconsistent changes: rating1=%d newR1=%d change1=%d, rating2=%d newR2=%d change2=%d",
				rating1, newR1, change1, rating2, newR2, change2)
			return false
		}

		// If the floor kicked in for either player, skip the zero-sum check.
		if newR1 == 0 || newR2 == 0 {
			return true
		}

		// When floor has NOT been reached, changes should be approximately zero-sum.
		// Since each rating is computed independently with math.Round, the sum can
		// be off by at most 1.
		sum := change1 + change2
		if sum < -1 || sum > 1 {
			t.Logf("zero-sum violated: r1=%d r2=%d winner=%d change1=%d change2=%d sum=%d",
				rating1, rating2, winner, change1, change2, sum)
			return false
		}
		return true
	}

	if err := quick.Check(f, quickCfg); err != nil {
		t.Errorf("zero-sum property failed: %v", err)
	}
}

// TestPropertyRatingFloor verifies that new ratings are never negative
// for any input ratings and any winner.
func TestPropertyRatingFloor(t *testing.T) {
	calc := NewDefaultEloCalculator()

	f := func(r1, r2 boundedRating, w boundedWinner) bool {
		rating1, rating2, winner := int(r1), int(r2), int(w)

		newR1, newR2, _, _ := calc.ProcessMatch(rating1, rating2, winner)

		if newR1 < 0 {
			t.Logf("floor violated for player 1: r1=%d r2=%d winner=%d newR1=%d",
				rating1, rating2, winner, newR1)
			return false
		}
		if newR2 < 0 {
			t.Logf("floor violated for player 2: r1=%d r2=%d winner=%d newR2=%d",
				rating1, rating2, winner, newR2)
			return false
		}
		return true
	}

	if err := quick.Check(f, quickCfg); err != nil {
		t.Errorf("rating floor property failed: %v", err)
	}
}

// TestPropertySymmetry verifies that ProcessMatch(r1, r2, 1) produces
// mirrored results compared to ProcessMatch(r2, r1, 2).
// Specifically: if ProcessMatch(r1, r2, 1) = (newA1, newA2, cA1, cA2),
// then ProcessMatch(r2, r1, 2) should give (newB1, newB2, cB1, cB2)
// where newA1 == newB2 and newA2 == newB1.
func TestPropertySymmetry(t *testing.T) {
	calc := NewDefaultEloCalculator()

	f := func(r1, r2 boundedRating) bool {
		rating1, rating2 := int(r1), int(r2)

		// Player 1 wins
		newA1, newA2, cA1, cA2 := calc.ProcessMatch(rating1, rating2, 1)
		// Swap players and player 2 wins (equivalent scenario)
		newB1, newB2, cB1, cB2 := calc.ProcessMatch(rating2, rating1, 2)

		// The winning player's new rating should be the same in both cases.
		// In the first call, player 1 wins => newA1 is the winner's new rating.
		// In the second call, player 2 wins => newB2 is the winner's new rating.
		if newA1 != newB2 {
			t.Logf("symmetry violated (winner): r1=%d r2=%d newA1=%d newB2=%d",
				rating1, rating2, newA1, newB2)
			return false
		}
		if newA2 != newB1 {
			t.Logf("symmetry violated (loser): r1=%d r2=%d newA2=%d newB1=%d",
				rating1, rating2, newA2, newB1)
			return false
		}
		if cA1 != cB2 {
			t.Logf("symmetry violated (winner change): r1=%d r2=%d cA1=%d cB2=%d",
				rating1, rating2, cA1, cB2)
			return false
		}
		if cA2 != cB1 {
			t.Logf("symmetry violated (loser change): r1=%d r2=%d cA2=%d cB1=%d",
				rating1, rating2, cA2, cB1)
			return false
		}
		return true
	}

	if err := quick.Check(f, quickCfg); err != nil {
		t.Errorf("symmetry property failed: %v", err)
	}
}

// TestPropertyWinnerGains verifies that:
//   - The winner's rating change is >= 0.
//   - The loser's rating change is <= 0.
//   - For draws between unequal ratings, the lower-rated player gains and the
//     higher-rated player loses; for equal ratings, changes are 0.
func TestPropertyWinnerGains(t *testing.T) {
	calc := NewDefaultEloCalculator()

	t.Run("Winner=1", func(t *testing.T) {
		f := func(r1, r2 boundedRating) bool {
			rating1, rating2 := int(r1), int(r2)
			_, _, change1, change2 := calc.ProcessMatch(rating1, rating2, 1)

			if change1 < 0 {
				t.Logf("winner lost rating: r1=%d r2=%d change1=%d", rating1, rating2, change1)
				return false
			}
			if change2 > 0 {
				t.Logf("loser gained rating: r1=%d r2=%d change2=%d", rating1, rating2, change2)
				return false
			}
			return true
		}
		if err := quick.Check(f, quickCfg); err != nil {
			t.Errorf("winner gains (player 1) property failed: %v", err)
		}
	})

	t.Run("Winner=2", func(t *testing.T) {
		f := func(r1, r2 boundedRating) bool {
			rating1, rating2 := int(r1), int(r2)
			_, _, change1, change2 := calc.ProcessMatch(rating1, rating2, 2)

			if change2 < 0 {
				t.Logf("winner lost rating: r1=%d r2=%d change2=%d", rating1, rating2, change2)
				return false
			}
			if change1 > 0 {
				t.Logf("loser gained rating: r1=%d r2=%d change1=%d", rating1, rating2, change1)
				return false
			}
			return true
		}
		if err := quick.Check(f, quickCfg); err != nil {
			t.Errorf("winner gains (player 2) property failed: %v", err)
		}
	})

	t.Run("Draw", func(t *testing.T) {
		f := func(r1, r2 boundedRating) bool {
			rating1, rating2 := int(r1), int(r2)
			_, _, change1, change2 := calc.ProcessMatch(rating1, rating2, 0)

			// For draws, changes should be bounded by K-factor.
			kFactor := calc.GetKFactor()
			if change1 > kFactor || change1 < -kFactor {
				t.Logf("draw change1 out of bounds: r1=%d r2=%d change1=%d K=%d",
					rating1, rating2, change1, kFactor)
				return false
			}
			if change2 > kFactor || change2 < -kFactor {
				t.Logf("draw change2 out of bounds: r1=%d r2=%d change2=%d K=%d",
					rating1, rating2, change2, kFactor)
				return false
			}

			// Higher-rated player should lose or stay (change <= 0).
			// Lower-rated player should gain or stay (change >= 0).
			if rating1 > rating2 {
				if change1 > 0 {
					t.Logf("higher-rated gained on draw: r1=%d r2=%d change1=%d",
						rating1, rating2, change1)
					return false
				}
				if change2 < 0 {
					t.Logf("lower-rated lost on draw: r1=%d r2=%d change2=%d",
						rating1, rating2, change2)
					return false
				}
			} else if rating2 > rating1 {
				if change2 > 0 {
					t.Logf("higher-rated gained on draw: r1=%d r2=%d change2=%d",
						rating1, rating2, change2)
					return false
				}
				if change1 < 0 {
					t.Logf("lower-rated lost on draw: r1=%d r2=%d change1=%d",
						rating1, rating2, change1)
					return false
				}
			} else {
				// Equal ratings: changes should be 0.
				if change1 != 0 || change2 != 0 {
					t.Logf("equal ratings draw produced non-zero change: r1=%d r2=%d c1=%d c2=%d",
						rating1, rating2, change1, change2)
					return false
				}
			}
			return true
		}
		if err := quick.Check(f, quickCfg); err != nil {
			t.Errorf("draw property failed: %v", err)
		}
	})
}

// TestPropertyMonotonicity verifies that if ratingA > ratingB, then
// CalculateExpectedScore(ratingA, opponentRating) > CalculateExpectedScore(ratingB, opponentRating)
// for any fixed opponent rating. In other words, higher-rated players always
// have a higher expected score against the same opponent.
func TestPropertyMonotonicity(t *testing.T) {
	calc := NewDefaultEloCalculator()

	f := func(rA, rB, rOpp boundedRating) bool {
		ratingA, ratingB, opponent := int(rA), int(rB), int(rOpp)

		if ratingA == ratingB {
			// Skip equal ratings -- nothing to test.
			return true
		}

		expectedA := calc.CalculateExpectedScore(ratingA, opponent)
		expectedB := calc.CalculateExpectedScore(ratingB, opponent)

		if ratingA > ratingB {
			if expectedA <= expectedB {
				t.Logf("monotonicity violated: rA=%d > rB=%d but expectedA=%.6f <= expectedB=%.6f (opp=%d)",
					ratingA, ratingB, expectedA, expectedB, opponent)
				return false
			}
		} else {
			if expectedB <= expectedA {
				t.Logf("monotonicity violated: rB=%d > rA=%d but expectedB=%.6f <= expectedA=%.6f (opp=%d)",
					ratingB, ratingA, expectedB, expectedA, opponent)
				return false
			}
		}
		return true
	}

	if err := quick.Check(f, quickCfg); err != nil {
		t.Errorf("monotonicity property failed: %v", err)
	}
}

// TestPropertyExpectedScoreHigherRatedAboveHalf verifies that if ratingA > ratingB,
// then CalculateExpectedScore(ratingA, ratingB) > 0.5.
func TestPropertyExpectedScoreHigherRatedAboveHalf(t *testing.T) {
	calc := NewDefaultEloCalculator()

	f := func(r1, r2 boundedRating) bool {
		ratingA, ratingB := int(r1), int(r2)

		if ratingA == ratingB {
			// Equal ratings should give exactly 0.5.
			expected := calc.CalculateExpectedScore(ratingA, ratingB)
			if expected != 0.5 {
				t.Logf("equal ratings did not give 0.5: r=%d expected=%.6f", ratingA, expected)
				return false
			}
			return true
		}

		higher, lower := ratingA, ratingB
		if ratingB > ratingA {
			higher, lower = ratingB, ratingA
		}

		expected := calc.CalculateExpectedScore(higher, lower)
		if expected <= 0.5 {
			t.Logf("higher rated expected <= 0.5: higher=%d lower=%d expected=%.6f",
				higher, lower, expected)
			return false
		}
		return true
	}

	if err := quick.Check(f, quickCfg); err != nil {
		t.Errorf("expected score > 0.5 for higher-rated property failed: %v", err)
	}
}

// TestPropertyChangeConsistency verifies that CalculateRatingChange returns
// the same value as CalculateNewRating minus the current rating.
func TestPropertyChangeConsistency(t *testing.T) {
	calc := NewDefaultEloCalculator()

	scores := []float64{0.0, 0.5, 1.0}

	f := func(r1, r2 boundedRating) bool {
		rating, opponent := int(r1), int(r2)

		for _, score := range scores {
			newRating := calc.CalculateNewRating(rating, opponent, score)
			change := calc.CalculateRatingChange(rating, opponent, score)

			if newRating-rating != change {
				t.Logf("change inconsistency: r=%d opp=%d score=%.1f newRating=%d change=%d expected=%d",
					rating, opponent, score, newRating, change, newRating-rating)
				return false
			}
		}
		return true
	}

	if err := quick.Check(f, quickCfg); err != nil {
		t.Errorf("change consistency property failed: %v", err)
	}
}

// TestPropertyExpectedScoreComplementary verifies that for any two ratings,
// CalculateExpectedScore(a, b) + CalculateExpectedScore(b, a) == 1.0
// (within floating-point tolerance).
func TestPropertyExpectedScoreComplementary(t *testing.T) {
	calc := NewDefaultEloCalculator()

	f := func(r1, r2 boundedRating) bool {
		ratingA, ratingB := int(r1), int(r2)

		eAB := calc.CalculateExpectedScore(ratingA, ratingB)
		eBA := calc.CalculateExpectedScore(ratingB, ratingA)

		sum := eAB + eBA
		if sum < 0.9999999 || sum > 1.0000001 {
			t.Logf("complementary violated: rA=%d rB=%d eAB=%.10f eBA=%.10f sum=%.10f",
				ratingA, ratingB, eAB, eBA, sum)
			return false
		}
		return true
	}

	if err := quick.Check(f, quickCfg); err != nil {
		t.Errorf("complementary expected score property failed: %v", err)
	}
}

// TestPropertyChangeBoundedByKFactor verifies that the absolute rating change
// is always bounded by the K-factor.
func TestPropertyChangeBoundedByKFactor(t *testing.T) {
	calc := NewDefaultEloCalculator()
	kFactor := calc.GetKFactor()

	f := func(r1, r2 boundedRating, w boundedWinner) bool {
		rating1, rating2, winner := int(r1), int(r2), int(w)

		newR1, newR2, change1, change2 := calc.ProcessMatch(rating1, rating2, winner)

		// Normal (non-floor) changes should be bounded by K-factor.
		// When the floor activates the reported change can differ from the raw
		// ELO change because the rating is clamped to 0.
		abs1 := change1
		if abs1 < 0 {
			abs1 = -abs1
		}
		abs2 := change2
		if abs2 < 0 {
			abs2 = -abs2
		}

		// If the floor did NOT activate, the change must be <= K.
		if newR1 > 0 && abs1 > kFactor {
			t.Logf("change1 exceeds K: r1=%d r2=%d winner=%d change1=%d K=%d",
				rating1, rating2, winner, change1, kFactor)
			return false
		}
		if newR2 > 0 && abs2 > kFactor {
			t.Logf("change2 exceeds K: r1=%d r2=%d winner=%d change2=%d K=%d",
				rating1, rating2, winner, change2, kFactor)
			return false
		}

		// When the floor DID activate, the change equals -(original rating)
		// which is bounded by the original rating, not K.
		if newR1 == 0 && abs1 > rating1 {
			t.Logf("floor change1 exceeds original: r1=%d change1=%d", rating1, change1)
			return false
		}
		if newR2 == 0 && abs2 > rating2 {
			t.Logf("floor change2 exceeds original: r2=%d change2=%d", rating2, change2)
			return false
		}

		return true
	}

	if err := quick.Check(f, quickCfg); err != nil {
		t.Errorf("change bounded by K-factor property failed: %v", err)
	}
}

// TestPropertyDrawSymmetry verifies that for a draw between equal-rated
// players, both changes are exactly 0.
func TestPropertyDrawSymmetry(t *testing.T) {
	calc := NewDefaultEloCalculator()

	f := func(r boundedRating) bool {
		rating := int(r)
		_, _, change1, change2 := calc.ProcessMatch(rating, rating, 0)

		if change1 != 0 || change2 != 0 {
			t.Logf("equal draw not zero: r=%d change1=%d change2=%d",
				rating, change1, change2)
			return false
		}
		return true
	}

	if err := quick.Check(f, quickCfg); err != nil {
		t.Errorf("draw symmetry property failed: %v", err)
	}
}

func TestPropertySummary(t *testing.T) {
	properties := []string{
		"TestPropertyZeroSum",
		"TestPropertyRatingFloor",
		"TestPropertySymmetry",
		"TestPropertyWinnerGains",
		"TestPropertyMonotonicity",
		"TestPropertyExpectedScoreHigherRatedAboveHalf",
		"TestPropertyChangeConsistency",
		"TestPropertyExpectedScoreComplementary",
		"TestPropertyChangeBoundedByKFactor",
		"TestPropertyDrawSymmetry",
	}

	fmt.Printf("\nProperty-based tests (%d properties, %d iterations each):\n", len(properties), quickCfg.MaxCount)
	for _, p := range properties {
		fmt.Printf("  - %s\n", p)
	}
}
