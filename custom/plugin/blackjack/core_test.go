package blackjack

import "testing"

func TestHandValueDistinguishesBlackjackFromThreeCard21(t *testing.T) {
	bj := Hand{Cards: []Card{{Rank: "A"}, {Rank: "K"}}}
	if bj.Value() != 21 || !bj.IsBlackjack() {
		t.Fatalf("A+K = %d/%v, want 21/true", bj.Value(), bj.IsBlackjack())
	}
	three := Hand{Cards: []Card{{Rank: "7"}, {Rank: "7"}, {Rank: "7"}}}
	if three.Value() != 21 || three.IsBlackjack() {
		t.Fatalf("three-card 21 = %d/%v, want 21/false", three.Value(), three.IsBlackjack())
	}
}
func TestSplitCreatesIndependentHandsAndBets(t *testing.T) {
	p := &Player{Hands: []Hand{{Cards: []Card{{Rank: "8"}, {Rank: "8"}}}}, Bets: []int{BaseChip}, HandStates: []HandState{HandActive}}
	if err := p.splitHand(); err != nil {
		t.Fatalf("splitHand: %v", err)
	}
	if len(p.Hands) != 2 || len(p.Bets) != 2 || len(p.HandStates) != 2 {
		t.Fatalf("split lengths = %d/%d/%d", len(p.Hands), len(p.Bets), len(p.HandStates))
	}
	if len(p.Hands[0].Cards) != 1 || len(p.Hands[1].Cards) != 1 || p.Bets[0] != BaseChip || p.Bets[1] != BaseChip {
		t.Fatal("split did not preserve independent hands and bets")
	}
	if err := p.splitHand(); err == nil {
		t.Fatal("split was allowed twice")
	}
}
func TestDoubleRequiresInitialTwoCardHandAndEndsHand(t *testing.T) {
	p := &Player{Hands: []Hand{{Cards: []Card{{Rank: "5"}, {Rank: "6"}}}}, Bets: []int{BaseChip}, HandStates: []HandState{HandActive}}
	if err := p.doubleDown(); err != nil {
		t.Fatalf("doubleDown: %v", err)
	}
	if p.Bets[0] != BaseChip*2 || p.HandStates[0] != HandDoubled {
		t.Fatalf("double result = %d/%q", p.Bets[0], p.HandStates[0])
	}
	if err := p.doubleDown(); err == nil {
		t.Fatal("double was allowed twice")
	}
}
func TestSurrenderOnlyOnInitialHandAndReturnsHalfStake(t *testing.T) {
	p := &Player{Hands: []Hand{{Cards: []Card{{Rank: "10"}, {Rank: "6"}}}}, Bets: []int{BaseChip}, HandStates: []HandState{HandActive}}
	refund, err := p.surrender()
	if err != nil || refund != BaseChip/2 || p.HandStates[0] != HandSurrendered {
		t.Fatalf("surrender = %d/%v/%q", refund, err, p.HandStates[0])
	}
	p.Hands[0].Cards = append(p.Hands[0].Cards, Card{Rank: "2"})
	p.HandStates[0] = HandActive
	if _, err := p.surrender(); err == nil {
		t.Fatal("surrender was allowed after a hit")
	}
}
func TestHandPayouts(t *testing.T) {
	cases := []struct {
		name                 string
		state                HandState
		hand                 Hand
		dealer               int
		dealerBJ, dealerBust bool
		want                 int
	}{
		{"blackjack", HandBlackjack, Hand{Cards: []Card{{Rank: "A"}, {Rank: "K"}}}, 17, false, false, BaseChip + BaseChip*3/2},
		{"surrender", HandSurrendered, Hand{}, 20, false, false, BaseChip / 2},
		{"win", HandStand, Hand{Cards: []Card{{Rank: "10"}, {Rank: "8"}}}, 17, false, false, BaseChip * 2},
		{"push", HandStand, Hand{Cards: []Card{{Rank: "10"}, {Rank: "7"}}}, 17, false, false, BaseChip},
		{"dealer blackjack", HandStand, Hand{Cards: []Card{{Rank: "10"}, {Rank: "7"}}}, 21, true, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := handPayout(tc.state, tc.hand, BaseChip, tc.dealer, tc.dealerBJ, tc.dealerBust)
			if got != tc.want {
				t.Fatalf("payout = %d, want %d", got, tc.want)
			}
		})
	}
}
func TestStaleTurnVersionCannotForfeitNewTurn(t *testing.T) {
	g := &Game{turnVersion: 9}
	if g.turnIsCurrent(8) || !g.turnIsCurrent(9) {
		t.Fatal("turn token validation failed")
	}
}
