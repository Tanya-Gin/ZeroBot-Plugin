package blackjack

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/FloatTech/AnimeAPI/wallet"
	"github.com/FloatTech/ZeroBot-Plugin/plugin/poker"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

func parsePokerCardName(name string) Card {
	base := strings.TrimSuffix(name, ".jpg")
	for _, item := range []struct{ suffix, suit string }{{"club", "C"}, {"diamond", "D"}, {"heart", "H"}, {"spade", "S"}} {
		if strings.HasSuffix(base, item.suffix) {
			return Card{Name: name, Suit: item.suit, Rank: strings.TrimSuffix(base, item.suffix)}
		}
	}
	return Card{Name: name, Rank: "JOKER"}
}
func newDeck() []Card {
	canonical := poker.StandardDeck()
	deck := make([]Card, 0, len(canonical))
	for _, c := range canonical {
		deck = append(deck, parsePokerCardName(c.Name))
	}
	return deck
}
func shuffledDeck() []Card {
	deck := newDeck()
	rand.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	return deck
}
func (g *Game) drawCard() Card {
	if len(g.Deck) == 0 {
		g.Deck = shuffledDeck()
	}
	c := g.Deck[0]
	g.Deck = g.Deck[1:]
	return c
}
func newGame(ctx *zero.Ctx, mode GameMode, testMode bool) *Game {
	now := time.Now()
	status := StatusWaiting
	if mode == ModeSingle {
		status = StatusPlaying
	}
	return &Game{GroupID: ctx.Event.GroupID, BankerID: ctx.Event.SelfID, Mode: mode, Status: status, Dealer: Hand{Cards: make([]Card, 0, 8)}, Players: make([]*Player, 0, 8), Deck: shuffledDeck(), StartedAt: now, LastAction: now, TestMode: testMode, ctx: ctx}
}
func (g *Game) dealInitial() {
	for _, p := range g.Players {
		p.Hands[0].Cards = append(p.Hands[0].Cards, g.drawCard(), g.drawCard())
		if p.Hands[0].IsBlackjack() {
			p.HandStates[0] = HandBlackjack
		}
	}
	g.Dealer.Cards = append(g.Dealer.Cards, g.drawCard(), g.drawCard())
}
func (g *Game) dealSplitCards(p *Player) {
	for i := range p.Hands {
		p.Hands[i].Cards = append(p.Hands[i].Cards, g.drawCard())
		if p.Hands[i].Value() == 21 {
			p.HandStates[i] = HandStand
		}
	}
}
func debitToBank(g *Game, uid int64, amount int) error {
	if g.TestMode {
		return nil
	}
	if uid == g.BankerID {
		return fmt.Errorf("庄家不能参加自己的牌局")
	}
	before := wallet.GetWalletOf(uid)
	if before < amount {
		return ErrInsufficientFunds
	}
	if err := wallet.InsertWalletOf(uid, -amount); err != nil {
		return err
	}
	if wallet.GetWalletOf(uid) != before-amount {
		_ = wallet.InsertWalletOf(uid, amount)
		return ErrInsufficientFunds
	}
	bankBefore := wallet.GetWalletOf(g.BankerID)
	if err := wallet.InsertWalletOf(g.BankerID, amount); err != nil || wallet.GetWalletOf(g.BankerID) != bankBefore+amount {
		_ = wallet.InsertWalletOf(uid, amount)
		if wallet.GetWalletOf(g.BankerID) != bankBefore {
			_ = wallet.InsertWalletOf(g.BankerID, bankBefore-wallet.GetWalletOf(g.BankerID))
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("庄家钱包入账校验失败")
	}
	return nil
}
func refundPaidCoins(g *Game) error {
	if g.TestMode {
		return nil
	}
	var refundErrs []error
	for _, p := range g.Players {
		if p.PaidCoins <= 0 {
			continue
		}
		if err := transferVerified(g.BankerID, p.UID, p.PaidCoins); err == nil {
			p.PaidCoins = 0
		} else {
			refundErrs = append(refundErrs, fmt.Errorf("uid=%d amount=%d: %w", p.UID, p.PaidCoins, err))
		}
	}
	return errors.Join(refundErrs...)
}
func rollbackExtra(g *Game, uid int64) {
	if g.TestMode {
		return
	}
	for _, p := range g.Players {
		if p.UID != uid || p.PaidCoins < MinBuyIn {
			continue
		}
		if wallet.InsertWalletOf(p.UID, MinBuyIn) == nil {
			_ = wallet.InsertWalletOf(g.BankerID, -MinBuyIn)
			p.PaidCoins -= MinBuyIn
		}
		return
	}
}
func debitExtra(g *Game, uid int64) error {
	var player *Player
	for _, p := range g.Players {
		if p.UID == uid {
			player = p
			break
		}
	}
	if player == nil {
		return fmt.Errorf("玩家不在牌局中")
	}
	if err := debitToBank(g, uid, MinBuyIn); err != nil {
		return err
	}
	player.PaidCoins += MinBuyIn
	return nil
}
func (g *Game) startExpiry() {
	gid := g.GroupID
	time.AfterFunc(MaxGameDuration, func() {
		game := gameManager.getGame(gid)
		if game == nil {
			return
		}
		game.mu.Lock()
		if game.Status == StatusFinished {
			game.mu.Unlock()
			return
		}
		refundErr := refundPaidCoins(game)
		game.Status = StatusFinished
		ctx := game.ctx
		game.mu.Unlock()
		gameManager.removeGame(gid, game)
		if ctx != nil {
			if refundErr != nil {
				ctx.SendChain(message.Text("本局超过5分钟，牌局已结束，但退款核验失败，请管理员检查审计日志"))
			} else {
				ctx.SendChain(message.Text("本局超过5分钟，已自动结束，所有已投入金币已退回"))
			}
		}
	})
}
func (g *Game) scheduleTurnTimeout() {
	g.touchTurn()
	token, gid := g.turnVersion, g.GroupID
	time.AfterFunc(ActionTimeout, func() {
		game := gameManager.getGame(gid)
		if game == nil {
			return
		}
		game.mu.Lock()
		defer game.mu.Unlock()
		if game.Status != StatusPlaying || !game.turnIsCurrent(token) {
			return
		}
		p, _, ok := game.activeHand()
		if !ok || p.HandStates[game.CurrentHand] != HandActive {
			return
		}
		p.HandStates[game.CurrentHand] = HandForfeited
		game.advanceLocked(game.ctx, "本手超时，已判负")
	})
}
func (g *Game) advanceLocked(ctx *zero.Ctx, prefix string) {
	if len(g.Dealer.Cards) == 2 && g.Dealer.IsBlackjack() {
		g.dealerTurn(ctx)
		return
	}
	if !g.advancePure() {
		g.dealerTurn(ctx)
		return
	}
	g.LastAction = time.Now()
	g.scheduleTurnTimeout()
	g.sendPrompt(ctx, g.Players[g.CurrentPlayer], prefix)
}
func (g *Game) dealerTurn(ctx *zero.Ctx) {
	g.Status = StatusResolving
	for g.Dealer.Value() < DealerStandValue {
		g.Dealer.Cards = append(g.Dealer.Cards, g.drawCard())
	}
	g.settle(ctx)
}

type settlementPayment struct {
	player *Player
	hand   int
	amount int
	payout int
	result string
}

func buildSettlementPlan(g *Game, dealer int, dealerBlackjack, dealerBust bool) ([]settlementPayment, int) {
	plan := make([]settlementPayment, 0)
	total := 0
	for _, p := range g.Players {
		for i, h := range p.Hands {
			payout, result := handPayout(p.HandStates[i], h, p.Bets[i], dealer, dealerBlackjack, dealerBust)
			amount := cashOutAmount(payout, p.Bets[i])
			plan = append(plan, settlementPayment{player: p, hand: i, amount: amount, payout: payout, result: result})
			total += amount
		}
	}
	return plan, total
}

func transferVerified(from, to int64, amount int) error {
	if amount <= 0 {
		return nil
	}
	fromBefore := wallet.GetWalletOf(from)
	if fromBefore < amount {
		return fmt.Errorf("账户余额不足")
	}
	if err := wallet.InsertWalletOf(from, -amount); err != nil {
		return err
	}
	if got := wallet.GetWalletOf(from); got != fromBefore-amount {
		_ = wallet.InsertWalletOf(from, amount)
		return fmt.Errorf("扣款校验失败")
	}
	toBefore := wallet.GetWalletOf(to)
	if err := wallet.InsertWalletOf(to, amount); err != nil {
		_ = wallet.InsertWalletOf(from, amount)
		return err
	}
	if got := wallet.GetWalletOf(to); got != toBefore+amount {
		_ = wallet.InsertWalletOf(from, amount)
		return fmt.Errorf("入账校验失败")
	}
	return nil
}

func rollbackSettlementPayments(g *Game, applied []settlementPayment) {
	for i := len(applied) - 1; i >= 0; i-- {
		payment := applied[i]
		if payment.amount == 0 {
			continue
		}
		if err := transferVerified(payment.player.UID, g.BankerID, payment.amount); err != nil {
			log.Printf("[blackjack] settlement rollback failed group=%d uid=%d hand=%d amount=%d err=%v", g.GroupID, payment.player.UID, payment.hand+1, payment.amount, err)
		}
	}
}

func applySettlementPlan(g *Game, plan []settlementPayment, total int) error {
	if g.TestMode || total == 0 {
		return nil
	}
	if wallet.GetWalletOf(g.BankerID) < total {
		return fmt.Errorf("庄家余额不足，需赔付%d%s", total, wallet.GetWalletName())
	}
	applied := make([]settlementPayment, 0, len(plan))
	for _, payment := range plan {
		if err := transferVerified(g.BankerID, payment.player.UID, payment.amount); err != nil {
			rollbackSettlementPayments(g, applied)
			return err
		}
		applied = append(applied, payment)
	}
	return nil
}

func (g *Game) settle(ctx *zero.Ctx) {
	if g.Status == StatusFinished {
		return
	}
	dealer, dealerBJ, dealerBust := g.Dealer.Value(), g.Dealer.IsBlackjack(), g.Dealer.Value() > 21
	plan, total := buildSettlementPlan(g, dealer, dealerBJ, dealerBust)
	log.Printf("[blackjack] settlement audit group=%d players=%d total=%d%s", g.GroupID, len(g.Players), total, wallet.GetWalletName())
	if err := applySettlementPlan(g, plan, total); err != nil {
		log.Printf("[blackjack] settlement failed group=%d total=%d%s err=%v; refunding paid-in coins", g.GroupID, total, wallet.GetWalletName(), err)
		refundErr := refundPaidCoins(g)
		g.Status = StatusFinished
		if ctx != nil {
			if refundErr != nil {
				ctx.SendChain(message.Text("结算失败，牌局已停止，但退款核验失败，请管理员检查审计日志"))
			} else {
				ctx.SendChain(message.Text("结算失败，牌局已停止，已退回本局记录的投入：", err.Error()))
			}
		}
		gameManager.removeGame(g.GroupID, g)
		return
	}
	parts := []message.Segment{message.Text(fmt.Sprintf("━━━ 游戏结束 ━━━\n庄家：%s（%d点）\n判定：未爆牌且点数高于庄家即获胜；庄家爆牌时未爆牌玩家获胜；点数相同为平局。\n结算：下注以筹码计算，1筹码=%d%s；本金全额保护，盈利部分按%d%%兑现。\n\n", formatHand(&g.Dealer), dealer, ChipRate, wallet.GetWalletName(), CashOutPercent))}
	for _, p := range g.Players {
		parts = append(parts, message.At(p.UID), message.Text("\n"))
		for i, h := range p.Hands {
			payment := planForHand(plan, p, i)
			parts = append(parts, message.Text(formatHandSettlement(i+1, h, p.Bets[i], payment.payout, payment.result, payment.amount, handNetChange(payment.amount, p.Bets[i]))))
		}
	}
	if ctx != nil {
		ctx.SendChain(parts...)
	}
	g.Status = StatusFinished
	gameManager.removeGame(g.GroupID, g)
}

func planForHand(plan []settlementPayment, player *Player, hand int) settlementPayment {
	for _, payment := range plan {
		if payment.player == player && payment.hand == hand {
			return payment
		}
	}
	return settlementPayment{player: player, hand: hand, result: "结算数据缺失"}
}
func (g *Game) sendPrompt(ctx *zero.Ctx, p *Player, prefix string) {
	_, h, ok := g.activeHand()
	if !ok {
		return
	}
	text := formatTurnPrompt(g, p, h)
	if prefix != "" {
		text = prefix + "\n" + text
	}
	ctx.SendChain(message.At(p.UID), message.Text(text))
}

func formatTurnPrompt(g *Game, p *Player, h *Hand) string {
	dealer := "未发牌"
	if len(g.Dealer.Cards) > 0 {
		dealer = formatCard(g.Dealer.Cards[0]) + " + 暗牌"
	}
	bet := 0
	if g.CurrentHand >= 0 && g.CurrentHand < len(p.Bets) {
		bet = p.Bets[g.CurrentHand]
	}
	return fmt.Sprintf("庄家明牌：%s\n轮到你的手%d：%s（%d点，下注%d筹码=%d%s本金）\n提示：点数不超过21且高于庄家才能赢；也可以等待庄家爆牌。\n操作：要牌 / 停牌 / 分牌 / 加倍 / 投降", dealer, g.CurrentHand+1, formatHand(h), h.Value(), bet, bet*ChipRate, wallet.GetWalletName())
}
func (g *Game) hasPlayer(uid int64) bool {
	for _, p := range g.Players {
		if p.UID == uid {
			return true
		}
	}
	return false
}
func formatCard(c Card) string {
	return c.Rank + map[string]string{"S": "♠", "H": "♥", "D": "♦", "C": "♣"}[c.Suit]
}
func formatHand(h *Hand) string {
	cards := make([]string, 0, len(h.Cards))
	for _, c := range h.Cards {
		cards = append(cards, formatCard(c))
	}
	return strings.Join(cards, " ")
}
func formatCoinAmount(amount int) string { return fmt.Sprintf("%d%s", amount, wallet.GetWalletName()) }
