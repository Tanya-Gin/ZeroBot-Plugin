// Package blackjack 21点游戏
package blackjack

import (
	"time"

	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var (
	engine = control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "21点",
		Help:             blackjackHelpText(),
		PublicDataFolder: "Blackjack",
	})
	gameManager = &GameManager{games: make(map[int64]*Game), lastCreateTime: make(map[int64]time.Time)}
)

func init() {
	engine.OnFullMatch("21点规则", zero.OnlyGroup).SetBlock(true).Handle(handleRules)
	engine.OnRegex(`^(开始21点|21点单人)$`, zero.OnlyGroup).SetBlock(true).Handle(handleStartSingle)
	engine.OnRegex(`^(开始21点多人|21点房间)$`, zero.OnlyGroup).SetBlock(true).Handle(handleStartMulti)
	engine.OnRegex(`^(加入21点|加入)$`, zero.OnlyGroup).SetBlock(true).Handle(handleJoin)
	engine.OnRegex(`^(要牌|hit|h)$`, zero.OnlyGroup).SetBlock(true).Handle(handleHit)
	engine.OnRegex(`^(停牌|stand|s)$`, zero.OnlyGroup).SetBlock(true).Handle(handleStand)
	engine.OnRegex(`^(分牌|split)$`, zero.OnlyGroup).SetBlock(true).Handle(handleSplit)
	engine.OnRegex(`^(加倍|double)$`, zero.OnlyGroup).SetBlock(true).Handle(handleDouble)
	engine.OnRegex(`^(投降|surrender)$`, zero.OnlyGroup).SetBlock(true).Handle(handleSurrender)
	engine.OnRegex(`^(查看21点|牌局状态)$`, zero.OnlyGroup).SetBlock(true).Handle(handleStatus)
	engine.OnRegex(`^(强制结束21点|清理21点)$`, zero.OnlyGroup, zero.AdminPermission).SetBlock(true).Handle(handleForceEnd)
	engine.OnFullMatch("21点测试", zero.OnlyGroup, zero.AdminPermission).SetBlock(true).Handle(handleTestMode)
}
func blackjackHelpText() string {
	return "━━━ 21点命令与规则 ━━━\n\n" +
		"【开始游戏】\n" +
		"- 开始21点 / 21点单人：创建单人牌局并立即发牌\n" +
		"- 开始21点多人 / 21点房间：创建多人房间，30秒内可加入\n" +
		"- 加入21点 / 加入：加入等待中的多人房间\n\n" +
		"【进行中操作】\n" +
		"- 要牌 / hit / h：再抽一张牌\n" +
		"- 停牌 / stand / s：结束当前手牌操作\n" +
		"- 分牌 / split：起手两张同点数牌时分成两手，每局最多一次\n" +
		"- 加倍 / double：起手两张牌时追加一份下注，只抽一张后自动结束\n" +
		"- 投降 / surrender：起手两张牌时投降，退回该手一半下注\n" +
		"- 查看21点 / 牌局状态：查看当前牌局和所有公开手牌\n\n" +
		"【基础规则】\n" +
		"- 入场费：" + formatCoinAmount(MinBuyIn) + "，兑换 " + formatCoinAmount(BaseChip) + " 筹码\n" +
		"- A 可算 1 点或 11 点；J/Q/K 均算 10 点\n" +
		"- 手牌超过21点为爆牌；Blackjack 必须是起手两张牌合计21点\n" +
		"- 庄家不足17点必须继续要牌，17点或以上停牌\n" +
		"- 普通胜利按下注额2倍返还；平局返还原下注\n" +
		"- Blackjack 按下注额1.5倍净赔率结算\n" +
		"- 投降退回一半下注；爆牌、判负不返还\n" +
		"- 兑现时本金全额保护，只对盈利部分收取20%手续费\n\n" +
		"【牌局限制】\n" +
		"- 每个群独立一桌，机器人坐庄\n" +
		"- 多人牌局最多5名玩家；加入、分牌、加倍前会检查庄家最大赔付能力\n" +
		"- 每次操作60秒无回应，本手判负\n" +
		"- 整局最长5分钟，超时会退回已记录的投入\n" +
		"- 结算前会预检并记录赔付；若结算失败，将停止本局并执行整局退款\n" +
		"- 管理员可发送“强制结束21点”或“清理21点”结束牌局并退款\n\n" +
		"【测试】\n" +
		"- 21点测试：管理员测试模式，不修改真实钱包"
}

func handleRules(ctx *zero.Ctx) {
	ctx.SendChain(message.Text(blackjackHelpText()))
}
