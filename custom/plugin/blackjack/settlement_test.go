package blackjack

import (
	"strings"
	"testing"
)

func TestSettlementExplainsWhyHandWonOrLost(t *testing.T) {
	hand := Hand{Cards: []Card{{Rank: "10"}, {Rank: "8"}}}
	cases := []struct {
		name       string
		state      HandState
		dealer     int
		dealerBJ   bool
		dealerBust bool
		wantResult string
		wantPayout int
	}{
		{"player bust", HandBust, 18, false, false, "超过21点", 0},
		{"timeout forfeiture", HandForfeited, 18, false, false, "超时弃权", 0},
		{"dealer blackjack", HandStand, 21, true, false, "庄家Blackjack", 0},
		{"dealer bust", HandStand, 22, false, true, "庄家爆牌", BaseChip * 2},
		{"player higher", HandStand, 17, false, false, "高于庄家", BaseChip * 2},
		{"tie", HandStand, 18, false, false, "平局", BaseChip},
		{"player lower", HandStand, 19, false, false, "低于庄家", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payout, result := handPayout(tc.state, hand, BaseChip, tc.dealer, tc.dealerBJ, tc.dealerBust)
			if payout != tc.wantPayout {
				t.Fatalf("payout = %d, want %d", payout, tc.wantPayout)
			}
			if !strings.Contains(result, tc.wantResult) {
				t.Fatalf("result %q does not explain %q", result, tc.wantResult)
			}
		})
	}
}

func TestSettlementShowsReturnedAmountAndNetChange(t *testing.T) {
	const bet = BaseChip
	returned := cashOutAmount(BaseChip*2, BaseChip)
	if returned != 1800 {
		t.Fatalf("cashOutAmount(win payout) = %d, want 1800", returned)
	}
	if got := handNetChange(returned, bet); got != 800 {
		t.Fatalf("handNetChange(win) = %d, want 800", got)
	}
	line := formatHandSettlement(1, Hand{Cards: []Card{{Rank: "10", Suit: "S"}, {Rank: "8", Suit: "H"}}}, bet, BaseChip*2, "获胜：点数高于庄家", returned, 800)
	for _, want := range []string{"手1", "18点", "下注100筹码", "结算200筹码", "返还1800", "净变化+800"} {
		if !strings.Contains(line, want) {
			t.Fatalf("settlement line %q does not contain %q", line, want)
		}
	}
}

func TestSettlementNetChangeIncludesCashoutFee(t *testing.T) {
	pushReturn := cashOutAmount(BaseChip, BaseChip)
	if pushReturn != MinBuyIn {
		t.Fatalf("push return = %d, want %d", pushReturn, MinBuyIn)
	}
	if got := handNetChange(pushReturn, BaseChip); got != 0 {
		t.Fatalf("push net change = %d, want 0", got)
	}
}
