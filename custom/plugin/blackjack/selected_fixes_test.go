package blackjack

import (
	"strings"
	"testing"
)

func TestAdvancePureSkipsCompletedSplitHands(t *testing.T) {
	g := &Game{
		Players: []*Player{{
			Hands:      []Hand{{}, {}},
			HandStates: []HandState{HandStand, HandActive},
		}},
		CurrentPlayer: 0,
		CurrentHand:   0,
	}
	if !g.advancePure() || g.CurrentHand != 1 {
		t.Fatalf("advancePure() did not move to the second split hand: live=%v hand=%d", g.CurrentHand < len(g.Players[0].Hands), g.CurrentHand)
	}
}

func TestAdvancePureFinishesWhenAllSplitHandsAreDone(t *testing.T) {
	g := &Game{
		Players: []*Player{{
			Hands:      []Hand{{}, {}},
			HandStates: []HandState{HandStand, HandStand},
		}},
		CurrentPlayer: 0,
		CurrentHand:   0,
	}
	if g.advancePure() {
		t.Fatal("advancePure() reported a live hand after both split hands finished")
	}
}

func TestCashOutProtectsPrincipalAndChargesOnlyProfit(t *testing.T) {
	cases := []struct {
		name   string
		payout int
		bet    int
		want   int
	}{
		{"loss", 0, BaseChip, 0},
		{"surrender", BaseChip / 2, BaseChip, MinBuyIn / 2},
		{"push", BaseChip, BaseChip, MinBuyIn},
		{"normal win", BaseChip * 2, BaseChip, 1800},
		{"blackjack", BaseChip + BaseChip*3/2, BaseChip, 2200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cashOutAmount(tc.payout, tc.bet); got != tc.want {
				t.Fatalf("cashOutAmount(%d,%d) = %d, want %d", tc.payout, tc.bet, got, tc.want)
			}
		})
	}
}

func TestMaximumPayoutUsesEachHandAndCapsPlayerCount(t *testing.T) {
	if MaxPlayers != 5 {
		t.Fatalf("MaxPlayers = %d, want 5", MaxPlayers)
	}
	p := &Player{Hands: []Hand{{}, {}}, Bets: []int{BaseChip * 2, BaseChip * 2}}
	if got := playerMaximumPayout(p); got != BaseChip*4 {
		t.Fatalf("playerMaximumPayout() = %d, want %d", got, BaseChip*4)
	}
	if got := maximumPayoutForPlayers([]*Player{p}); got != BaseChip*4 {
		t.Fatalf("maximumPayoutForPlayers() = %d, want %d", got, BaseChip*4)
	}
}

func TestSplitTwentyOneHandsAreAutoTerminalAndAdvance(t *testing.T) {
	p := &Player{
		Hands:      []Hand{{Cards: []Card{{Rank: "A"}}}, {Cards: []Card{{Rank: "A"}}}},
		Bets:       []int{BaseChip, BaseChip},
		HandStates: []HandState{HandActive, HandActive},
	}
	g := &Game{Players: []*Player{p}, Deck: []Card{{Rank: "10"}, {Rank: "10"}}, CurrentPlayer: 0, CurrentHand: 0}
	g.dealSplitCards(p)
	if p.HandStates[0] != HandStand || p.HandStates[1] != HandStand {
		t.Fatalf("split hands after 21 = %q/%q, want stand/stand", p.HandStates[0], p.HandStates[1])
	}
	if g.advancePure() {
		t.Fatal("advancePure reported a live hand after both split hands reached 21")
	}
}

func TestPlayerCapacityAllowsFourAndRejectsFive(t *testing.T) {
	players := make([]*Player, 4)
	if err := validatePlayerCapacity(players); err != nil {
		t.Fatalf("four players rejected: %v", err)
	}
	players = append(players, &Player{})
	if err := validatePlayerCapacity(players); err == nil {
		t.Fatal("five players were accepted despite the table limit")
	}
}

func TestFormatHandSettlementIncludesReasonAndFinancials(t *testing.T) {
	line := formatHandSettlement(1, Hand{Cards: []Card{{Rank: "10", Suit: "S"}, {Rank: "8", Suit: "H"}}}, BaseChip, BaseChip*2, "获胜：点数高于庄家", 1800, 800)
	for _, want := range []string{"18点", "下注100筹码", "获胜：点数高于庄家", "结算200筹码", "返还1800", "净变化+800"} {
		if !strings.Contains(line, want) {
			t.Fatalf("settlement line %q does not contain %q", line, want)
		}
	}
}
