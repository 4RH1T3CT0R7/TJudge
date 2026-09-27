package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestStrategyTally(t *testing.T) {
	share := func(v float64) *float64 { return &v }
	const C, D = MoveCooperate, MoveDefect

	tests := []struct {
		name string
		own  [][]int // матчи стороны
		opp  [][]int
		want StrategyProfile
	}{
		{
			name: "tit for tat против вечного предателя",
			own:  [][]int{{C, D, D, D}},
			opp:  [][]int{{D, D, D, D}},
			want: StrategyProfile{Matches: 1, Cooperation: share(0.25), Niceness: share(1),
				Retaliation: share(1), Provocability: share(1)},
		},
		{
			name: "вечный предатель против tit for tat",
			own:  [][]int{{D, D, D, D}},
			opp:  [][]int{{C, D, D, D}},
			want: StrategyProfile{Matches: 1, Cooperation: share(0), Niceness: share(0),
				Retaliation: share(1)},
		},
		{
			name: "grim и tit for tat против разового предательства",
			own:  [][]int{{C, C, D, D}, {C, C, D, C}},
			opp:  [][]int{{C, D, C, C}, {C, D, C, C}},
			want: StrategyProfile{Matches: 2, Cooperation: share(5.0 / 8), Niceness: share(1),
				Retaliation: share(1), Forgiveness: share(0.5), Provocability: share(1)},
		},
		{
			name: "предательство в один ход с соперником",
			own:  [][]int{{C, D, C}},
			opp:  [][]int{{C, D, C}},
			want: StrategyProfile{Matches: 1, Cooperation: share(2.0 / 3), Niceness: share(0),
				Retaliation: share(0), Provocability: share(0)},
		},
		{
			name: "матч без ходов не считается",
			own:  [][]int{{}},
			opp:  [][]int{{C}},
			want: StrategyProfile{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tally StrategyTally
			for i := range tt.own {
				tally.Add(tt.own[i], tt.opp[i])
			}
			id := uuid.New()
			tt.want.TeamID, tt.want.TeamName = id, "t"
			assert.Equal(t, &tt.want, tally.Profile(id, "t"))
		})
	}
}
