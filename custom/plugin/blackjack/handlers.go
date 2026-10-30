package blackjack

import (
	"fmt"
	"time"

	"github.com/FloatTech/AnimeAPI/wallet"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

func handleStartSingle(ctx *zero.Ctx) { startGame(ctx, ModeSingle) }
func handleStartMulti(ctx *zero.Ctx)  { startGame(ctx, ModeMulti) }
func startGame(ctx *zero.Ctx, mode GameMode) {
	gid, uid := ctx.Event.GroupID, ctx.Event.UserID
	if !gameManager.checkCooldown(gid) {
		ctx.SendChain(message.Text("操作过快，请稍后再试"))
		return
	}
	if gameManager.getGame(gid) != nil {
		ctx.SendChain(message.Text("本群已有一桌21点，请等待结束"))
		return
	}
	if uid == ctx.Event.SelfID {
		ctx.SendChain(message.Text("庄家不能参加自己的牌局"))
		return
	}
	if wallet.GetWalletOf(uid) < MinBuyIn {
		ctx.SendChain(message.Text("余额不足，开局需要", formatCoinAmount(MinBuyIn)))
		return
	}
	if wallet.GetWalletOf(ctx.Event.SelfID) < BankerReserve {
		ctx.SendChain(message.Text("庄家储备金不足，暂时无法开局"))
		return
	}
	g := newGame(ctx, mode, false)
	if err := debitToBank(g, uid, MinBuyIn); err != nil {
		ctx.SendChain(message.Text("开局失败：", err.Error()))
		return
	}
	g.Players = append(g.Players, newPlayer(uid))
	if !gameManager.addGame(g) {
		_ = refundPaidCoins(g)
		ctx.SendChain(message.Text("本群已有一桌21点，请稍后再试"))
		return
	}
	g.startExpiry()
	if mode == ModeSingle {
		g.mu.Lock()
		g.dealInitial()
		g.advanceLocked(ctx, "21点游戏开始")
		g.mu.Unlock()
		return
	}
	ctx.SendChain(message.Text("多人房间已创建，房主："), message.At(uid), message.Text(fmt.Sprintf("\n入场费：%s，%d秒内发送加入21点", formatCoinAmount(MinBuyIn), int(JoinTimeout/time.Second))))
	time.AfterFunc(JoinTimeout, func() {
		game := gameManager.getGame(gid)
		if game == nil {
			return
		}
		game.mu.Lock()
		defer game.mu.Unlock()
		if game.Status != StatusWaiting {
			return
		}
		game.Status = StatusPlaying
		game.dealInitial()
		game.advanceLocked(game.ctx, "等待结束，游戏开始")
	})
}
func handleJoin(ctx *zero.Ctx) {
	g := gameManager.getGame(ctx.Event.GroupID)
	if g == nil {
		ctx.SendChain(message.Text("当前没有等待加入的多人牌局"))
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	uid := ctx.Event.UserID
	if g.Mode != ModeMulti || g.Status != StatusWaiting {
		ctx.SendChain(message.Text("当前牌局已开始，不能加入"))
		return
	}
	if g.hasPlayer(uid) {
		ctx.SendChain(message.Text("你已经加入本局"))
		return
	}
	if err := validatePlayerCapacity(g.Players); err != nil {
		ctx.SendChain(message.Text(err.Error()))
		return
	}
	if uid == g.BankerID || wallet.GetWalletOf(uid) < MinBuyIn {
		ctx.SendChain(message.Text("余额不足或庄家不能加入自己的牌局"))
		return
	}
	if !canAcceptExposure(g, BaseChip) {
		ctx.SendChain(message.Text("庄家当前可承担的赔付额度不足，暂时不能加入"))
		return
	}
	if err := debitToBank(g, uid, MinBuyIn); err != nil {
		ctx.SendChain(message.Text("加入失败：", err.Error()))
		return
	}
	g.Players = append(g.Players, newPlayer(uid))
	ctx.SendChain(message.Text("加入成功，当前玩家：", len(g.Players)))
}
func currentPlayerFor(g *Game, uid int64) (*Player, *Hand, bool) {
	p, h, ok := g.activeHand()
	if !ok || p.UID != uid {
		return nil, nil, false
	}
	return p, h, true
}
func handleHit(ctx *zero.Ctx) {
	g := gameManager.getGame(ctx.Event.GroupID)
	if g == nil {
		ctx.SendChain(message.Text("当前没有进行中的牌局"))
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Status != StatusPlaying {
		ctx.SendChain(message.Text("当前不能操作牌局"))
		return
	}
	p, h, ok := currentPlayerFor(g, ctx.Event.UserID)
	if !ok {
		ctx.SendChain(message.Text("还没轮到你操作"))
		return
	}
	card := g.drawCard()
	h.Cards = append(h.Cards, card)
	g.LastAction = time.Now()
	if h.Value() > 21 {
		p.HandStates[g.CurrentHand] = HandBust
		g.advanceLocked(ctx, fmt.Sprintf("要牌 %s，爆牌", formatCard(card)))
		return
	}
	if h.Value() == 21 {
		p.HandStates[g.CurrentHand] = HandStand
		g.advanceLocked(ctx, fmt.Sprintf("要牌 %s，已满21点，自动结束本手", formatCard(card)))
		return
	}
	ctx.SendChain(message.Text("你要到 ", formatCard(card), "，当前", h.Value(), "点；未超过21点，继续选择要牌或停牌"))
	g.scheduleTurnTimeout()
}
func handleStand(ctx *zero.Ctx) {
	g := gameManager.getGame(ctx.Event.GroupID)
	if g == nil {
		ctx.SendChain(message.Text("当前没有进行中的牌局"))
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Status != StatusPlaying {
		return
	}
	p, _, ok := currentPlayerFor(g, ctx.Event.UserID)
	if !ok {
		ctx.SendChain(message.Text("还没轮到你操作"))
		return
	}
	p.HandStates[g.CurrentHand] = HandStand
	g.LastAction = time.Now()
	g.advanceLocked(ctx, "停牌")
}
func handleSplit(ctx *zero.Ctx) {
	g := gameManager.getGame(ctx.Event.GroupID)
	if g == nil {
		ctx.SendChain(message.Text("当前没有进行中的牌局"))
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Status != StatusPlaying {
		return
	}
	p, _, ok := currentPlayerFor(g, ctx.Event.UserID)
	if !ok {
		ctx.SendChain(message.Text("还没轮到你操作"))
		return
	}
	if !p.canSplit() {
		ctx.SendChain(message.Text("只有未分牌的两张同点数手牌才能分牌"))
		return
	}
	if !canAcceptExposure(g, BaseChip) {
		ctx.SendChain(message.Text("庄家当前可承担的赔付额度不足，暂时不能分牌"))
		return
	}
	if err := debitExtra(g, p.UID); err != nil {
		ctx.SendChain(message.Text("分牌失败：", err.Error()))
		return
	}
	if err := p.splitHand(); err != nil {
		rollbackExtra(g, p.UID)
		ctx.SendChain(message.Text("分牌失败：", err.Error()))
		return
	}
	g.dealSplitCards(p)
	g.LastAction = time.Now()
	g.advanceLocked(ctx, "分牌成功；每手独立与庄家比较")
}
func handleDouble(ctx *zero.Ctx) {
	g := gameManager.getGame(ctx.Event.GroupID)
	if g == nil {
		ctx.SendChain(message.Text("当前没有进行中的牌局"))
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Status != StatusPlaying {
		return
	}
	p, h, ok := currentPlayerFor(g, ctx.Event.UserID)
	if !ok {
		ctx.SendChain(message.Text("还没轮到你操作"))
		return
	}
	if !p.canDouble() {
		ctx.SendChain(message.Text("加倍只能在未分牌的前两张牌使用"))
		return
	}
	if !canAcceptExposure(g, BaseChip) {
		ctx.SendChain(message.Text("庄家当前可承担的赔付额度不足，暂时不能加倍"))
		return
	}
	if err := debitExtra(g, p.UID); err != nil {
		ctx.SendChain(message.Text("加倍失败：", err.Error()))
		return
	}
	if err := p.doubleDown(); err != nil {
		rollbackExtra(g, p.UID)
		ctx.SendChain(message.Text("加倍失败：", err.Error()))
		return
	}
	card := g.drawCard()
	h.Cards = append(h.Cards, card)
	p.HandStates[g.CurrentHand] = HandDoubled
	g.LastAction = time.Now()
	g.advanceLocked(ctx, fmt.Sprintf("加倍并获得 %s", formatCard(card)))
}
func handleSurrender(ctx *zero.Ctx) {
	g := gameManager.getGame(ctx.Event.GroupID)
	if g == nil {
		ctx.SendChain(message.Text("当前没有进行中的牌局"))
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Status != StatusPlaying {
		return
	}
	p, _, ok := currentPlayerFor(g, ctx.Event.UserID)
	if !ok {
		ctx.SendChain(message.Text("还没轮到你操作"))
		return
	}
	if err := func() error { _, err := p.surrender(); return err }(); err != nil {
		ctx.SendChain(message.Text("投降失败：", err.Error()))
		return
	}
	g.LastAction = time.Now()
	g.advanceLocked(ctx, "投降，本手退回一半下注")
}
func handleStatus(ctx *zero.Ctx) {
	g := gameManager.getGame(ctx.Event.GroupID)
	if g == nil {
		ctx.SendChain(message.Text("当前没有进行中的牌局"))
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	dealer := "未发牌"
	if len(g.Dealer.Cards) > 0 {
		dealer = formatCard(g.Dealer.Cards[0]) + " [🃏]"
	}
	parts := []message.Segment{message.Text("━━━ 21点状态 ━━━\n庄家：" + dealer + "\n")}
	for _, p := range g.Players {
		parts = append(parts, message.Text("玩家"), message.At(p.UID), message.Text("："))
		for i, h := range p.Hands {
			parts = append(parts, message.Text(fmt.Sprintf("手%d %s（%d点，%s） ", i+1, formatHand(&h), h.Value(), p.HandStates[i])))
		}
		parts = append(parts, message.Text("\n"))
	}
	ctx.SendChain(parts...)
}
func handleForceEnd(ctx *zero.Ctx) {
	g := gameManager.getGame(ctx.Event.GroupID)
	if g == nil {
		ctx.SendChain(message.Text("当前没有进行中的牌局"))
		return
	}
	g.mu.Lock()
	var refundErr error
	if g.Status != StatusFinished {
		refundErr = refundPaidCoins(g)
		g.Status = StatusFinished
	}
	g.mu.Unlock()
	gameManager.removeGame(g.GroupID, g)
	if refundErr != nil {
		ctx.SendChain(message.Text("管理员已结束本局，但退款核验失败，请检查审计日志"))
	} else {
		ctx.SendChain(message.Text("管理员已结束本局，所有已投入金币已退回"))
	}
}
func handleTestMode(ctx *zero.Ctx) {
	g := newGame(ctx, ModeSingle, true)
	g.Players = append(g.Players, newPlayer(ctx.Event.UserID))
	if !gameManager.addGame(g) {
		ctx.SendChain(message.Text("本群已有一桌21点"))
		return
	}
	g.mu.Lock()
	g.dealInitial()
	g.advanceLocked(ctx, "21点测试模式开始（不涉及真实钱包）")
	g.mu.Unlock()
}
