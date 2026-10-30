package blackjack

import (
	"errors"
	"fmt"
	"time"

	"github.com/FloatTech/AnimeAPI/wallet"
)

var errGameFinished = errors.New("牌局已结束")

func (gm *GameManager) checkCooldown(groupID int64) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	last, ok := gm.lastCreateTime[groupID]
	if ok && time.Since(last) < CreateCooldown {
		return false
	}
	gm.lastCreateTime[groupID] = time.Now()
	return true
}
func (gm *GameManager) getGame(groupID int64) *Game {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	return gm.games[groupID]
}
func (gm *GameManager) addGame(g *Game) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	if _, ok := gm.games[g.GroupID]; ok {
		return false
	}
	gm.games[g.GroupID] = g
	return true
}
func (gm *GameManager) removeGame(groupID int64, target *Game) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	if target == nil || gm.games[groupID] == target {
		delete(gm.games, groupID)
	}
}

func (g *Game) touchTurn() uint64 {
	g.turnVersion++
	return g.turnVersion
}

// advancePure moves the cursor over finished hands. It returns whether a live
// hand remains, keeping message sending and dealer settlement outside the rule core.
func (g *Game) advancePure() bool {
	for g.CurrentPlayer < len(g.Players) {
		p := g.Players[g.CurrentPlayer]
		for g.CurrentHand < len(p.Hands) && p.HandStates[g.CurrentHand] != HandActive {
			g.CurrentHand++
		}
		if g.CurrentHand < len(p.Hands) {
			return true
		}
		g.CurrentPlayer++
		g.CurrentHand = 0
	}
	return false
}

func newPlayer(uid int64) *Player {
	return &Player{UID: uid, Hands: []Hand{{Cards: make([]Card, 0, 8)}}, Bets: []int{BaseChip}, HandStates: []HandState{HandActive}, PaidCoins: MinBuyIn, JoinTime: time.Now()}
}

func (gm *GameManager) hasGame(groupID int64) bool { return gm.getGame(groupID) != nil }

func playerMaximumPayout(p *Player) int {
	if p == nil {
		return 0
	}
	total := 0
	for _, bet := range p.Bets {
		if bet > 0 {
			total += bet
		}
	}
	return total
}

func maximumPayoutForPlayers(players []*Player) int {
	total := 0
	for _, p := range players {
		total += playerMaximumPayout(p)
	}
	return total
}

// maximumExposureChips is the conservative gross-return estimate used for
// banker solvency checks. It covers ordinary 2x wins and the 2.5x blackjack.
func maximumExposureChips(players []*Player, additionalBet int) int {
	total := 0
	for _, p := range players {
		if p == nil {
			continue
		}
		for _, bet := range p.Bets {
			if bet > 0 {
				candidate := bet * 2
				if bj := bet * 5 / 2; bj > candidate {
					candidate = bj
				}
				total += candidate
			}
		}
	}
	if additionalBet > 0 {
		total += additionalBet * 5 / 2
	}
	return total
}

func validatePlayerCapacity(players []*Player) error {
	if len(players) >= MaxPlayers {
		return fmt.Errorf("多人牌局已达到%d人上限", MaxPlayers)
	}
	return nil
}

func canAcceptExposure(g *Game, additionalBet int) bool {
	if g == nil || g.TestMode {
		return true
	}
	bankAfter := wallet.GetWalletOf(g.BankerID) + additionalBet*ChipRate
	return bankAfter >= maximumExposureChips(g.Players, additionalBet)*ChipRate
}

func handPayout(state HandState, h Hand, bet, dealer int, dealerBlackjack, dealerBust bool) (int, string) {
	if h.Value() > 21 {
		return 0, "爆牌：超过21点"
	}
	switch state {
	case HandBust, HandForfeited:
		if state == HandBust {
			return 0, "爆牌：超过21点"
		}
		return 0, "超时弃权：判负"
	case HandSurrendered:
		return bet / 2, "投降：退回一半下注"
	case HandBlackjack:
		if dealerBlackjack {
			return bet, "平局（双方Blackjack）"
		}
		return bet + bet*3/2, "Blackjack胜利：起手两张牌21点"
	}
	if dealerBlackjack {
		return 0, "失败：庄家Blackjack"
	}
	if dealerBust {
		return bet * 2, "获胜：庄家爆牌"
	}
	if h.Value() > dealer {
		return bet * 2, "获胜：点数高于庄家"
	}
	if h.Value() == dealer {
		return bet, "平局：点数与庄家相同"
	}
	return 0, "失败：点数低于庄家"
}

func cashOutAmount(payout, bet int) int {
	principal := bet * ChipRate
	if payout <= bet {
		return payout * ChipRate
	}
	profit := (payout - bet) * ChipRate
	return principal + profit*CashOutPercent/100
}

func handNetChange(returned, bet int) int {
	return returned - bet*ChipRate
}

func formatHandSettlement(index int, h Hand, bet, payout int, result string, returned, net int) string {
	return fmt.Sprintf("手%d %s（%d点，下注%d筹码）：%s，结算%d筹码，返还%d%s，净变化%+d%s\n", index, formatHand(&h), h.Value(), bet, result, payout, returned, wallet.GetWalletName(), net, wallet.GetWalletName())
}
