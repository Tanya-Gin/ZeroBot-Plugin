// Package mcfish 钓鱼模拟器
package mcfish

import (
	"image"
	"image/color"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FloatTech/AnimeAPI/wallet"
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/math"
	"github.com/FloatTech/gg"
	"github.com/FloatTech/gg/factory"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/img/text"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension/rate"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var (
	storeLimiter = rate.NewManager[int64](time.Second*3, 1)
	refresh      = false
	timeNow      = 0
	tradeMu      sync.Mutex
	refreshMu    sync.Mutex
	userLocks    sync.Map
	refreshFish  = func(ctx *zero.Ctx) bool {
		refreshMu.Lock()
		defer refreshMu.Unlock()
		if refresh && timeNow == localDateKey(time.Now()) {
			return true
		}
		var err error
		refresh, err = dbdata.refreshStroeInfo()
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.1]:", err))
			return refresh
		}
		timeNow = localDateKey(time.Now())
		return refresh
	}
)

func lockUser(uid int64) func() {
	mu, _ := userLocks.LoadOrStore(uid, &sync.Mutex{})
	mutex := mu.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

func limitSet(ctx *zero.Ctx) *rate.Limiter {
	return storeLimiter.Load(ctx.Event.UserID)
}

func init() {
	engine.OnFullMatchGroup([]string{"钓鱼看板", "钓鱼商店"}, getdb, refreshFish).SetBlock(true).Limit(limitSet).Handle(func(ctx *zero.Ctx) {
		infos, err := dbdata.getStoreInfo()
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.2]:", err))
			return
		}
		var picImage image.Image
		if len(infos) == 0 {
			picImage, err = drawStroeEmptyImage()
		} else {
			picImage, err = drawStroeInfoImage(infos)
		}
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.3]:", err))
			return
		}
		pic, err := factory.ToBytes(picImage)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.4]:", err))
			return
		}
		ctx.SendChain(message.ImageBytes(pic))
	})
	engine.OnRegex(`^出售(`+strings.Join(thingList, "|")+`)\s*(\d*)$`, getdb, refreshFish).SetBlock(true).Limit(limitSet).Handle(func(ctx *zero.Ctx) {
		uid := ctx.Event.UserID
		defer lockUser(uid)()
		thingName := ctx.State["regex_matched"].([]string)[1]
		number, countErr := parseCount(ctx.State["regex_matched"].([]string)[2])
		if countErr != nil {
			ctx.SendChain(message.Text("请输入正确的数量"))
			return
		}
		if strings.Contains(thingName, "竿") {
			number = 1
		}

		// 检测物品交易次数
		if strings.Contains(thingName, "竿") {
			number, err := dbdata.checkCanSalesFor(uid, thingName, number, false)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR,查询购买资质失败]:", err))
				return
			}
			if number <= 0 {
				ctx.SendChain(message.Text("一天只能交易10把鱼竿,明天再来售卖吧"))
				return
			}
		}

		articles, err := dbdata.getUserThingInfo(uid, thingName)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.5]:", err))
			return
		}
		if len(articles) == 0 {
			ctx.SendChain(message.Text("你的背包不存在该物品"))
			return
		}
		index := 0
		thing := article{}
		transactionDone := false
		userChanged := false
		storeChanged := false
		walletCredited := false
		payout := 0
		var soldThingBefore article
		var usedRecordBefore article
		recordChanged := false
		var soldStoreBefore, soldStoreAfter store
		defer func() {
			if transactionDone {
				return
			}
			if walletCredited {
				_ = wallet.InsertWalletOf(uid, -payout)
			}
			if storeChanged {
				if soldStoreBefore == (store{}) {
					soldStoreAfter.Number = 0
					_ = dbdata.updateStoreInfo(soldStoreAfter)
				} else {
					_ = dbdata.updateStoreInfo(soldStoreBefore)
				}
			}
			if userChanged {
				_ = dbdata.updateUserThingInfo(uid, soldThingBefore)
			}
			if recordChanged {
				_ = dbdata.updateUserThingInfo(uid, usedRecordBefore)
			}
		}()
		if len(articles) > 1 {
			msg := make(message.Message, 0, 3+len(articles))
			msg = append(msg, message.Reply(ctx.Event.MessageID), message.Text("找到以下物品:\n"))
			for i, info := range articles {
				if info.Other != "" && info.Name != "美西螈" {
					msg = append(msg, message.Text("[", i, "] ", info.Name, "(", info.Other, ")\n"))
				} else {
					msg = append(msg, message.Text(
						"[", i, "]", info.Name, "  数量: ", info.Number, "\n"))
				}
			}
			msg = append(msg, message.Text("————————\n输入对应序号进行出售,或回复“取消”取消"))
			ctx.Send(msg)
			// 等待用户下一步选择
			sell := false
			recv, cancel := zero.NewFutureEvent("message", 999, false, zero.RegexRule(`^(取消|\d+)$`), zero.CheckUser(ctx.Event.UserID), zero.CheckGroup(ctx.Event.GroupID)).Repeat()
			defer cancel()
			for {
				select {
				case <-time.After(time.Second * 120):
					ctx.Send(
						message.ReplyWithMessage(ctx.Event.MessageID,
							message.Text("等待超时,取消出售"),
						),
					)
					return
				case e := <-recv:
					nextcmd := e.Event.Message.String()
					if nextcmd == "取消" {
						ctx.Send(
							message.ReplyWithMessage(ctx.Event.MessageID,
								message.Text("已取消出售"),
							),
						)
						return
					}
					index, err = strconv.Atoi(nextcmd)
					if err != nil || index < 0 || index > len(articles)-1 {
						ctx.SendChain(message.At(ctx.Event.UserID), message.Text("请输入正确的序号"))
						continue
					}
					sell = true
				}
				if sell {
					break
				}
			}
		}

		thing = articles[index]
		soldThingBefore = thing
		if thing.Number < number {
			number = thing.Number
		}

		pice, err := calculateItemMarketValue(thingName, thing.Other)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go]:", err))
			return
		}
		previewPayout := pice * number * int(100-poleSaleTaxPercent) / 100
		ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("是否接受商店将以", previewPayout, "收购", number, "个", thingName, "?\n回答\"是\"或\"否\"")))
		// 等待用户下一步选择
		recv, cancel1 := zero.NewFutureEvent("message", 999, false, zero.RegexRule(`^(是|否)$`), zero.CheckUser(ctx.Event.UserID), zero.CheckGroup(ctx.Event.GroupID)).Repeat()
		defer cancel1()
		buy := false
		for {
			select {
			case <-time.After(time.Second * 60):
				ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("等待超时,取消出售")))
				return
			case e := <-recv:
				nextcmd := e.Event.Message.String()
				if nextcmd == "否" {
					ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("已取消出售")))
					return
				}
				buy = true
			}
			if buy {
				break
			}
		}

		records, err := dbdata.getUserThingInfo(uid, "唱片")
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.9.1]:", err))
			return
		}
		if len(records) != 0 {
			recordInfo := records[0]
			numberOfRecord := recordInfo.Number
			if thingName == "唱片" {
				numberOfRecord -= number
			}
			if numberOfRecord > 0 {
				ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("是否使用唱片让价格翻倍?\n回答\"是\"或\"否\"")))
				// 等待用户下一步选择
				recv, cancel2 := zero.NewFutureEvent("message", 999, false, zero.RegexRule(`^(是|否)$`), zero.CheckUser(ctx.Event.UserID), zero.CheckGroup(ctx.Event.GroupID)).Repeat()
				defer cancel2()
				use := false
				checkTime := false
				for {
					select {
					case <-time.After(time.Second * 60):
						checkTime = true
					case e := <-recv:
						nextcmd := e.Event.Message.String()
						if nextcmd == "是" {
							use = true
						}
						checkTime = true
					}
					if checkTime {
						break
					}
				}
				if use {
					pice *= 2
					if thingName == "唱片" {
						thing.Number--
					}
					usedRecordBefore = recordInfo
					recordInfo.Number--
					err = dbdata.updateUserThingInfo(uid, recordInfo)
					if err != nil {
						ctx.SendChain(message.Text("[ERROR at store.go.9.2]:", err))
						return
					}
					recordChanged = true
				}
			}
		}
		tradeMu.Lock()
		defer tradeMu.Unlock()
		msg := ""
		curse, err := dbdata.getNumberFor(uid, "宝藏诅咒")
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.9.3]:", err))
			return
		}
		if curse != 0 {
			msg = "\n(你身上绑定了" + strconv.Itoa(curse) + "层诅咒)"
			pice = pice * (100 - 10*curse) / 100
		}
		thing.Number -= number
		err = dbdata.updateUserThingInfo(uid, thing)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.6]:", err))
			return
		}
		userChanged = true
		newCommodity := store{}
		if isPole(thing.Name) {
			if pice >= priceList[poleBaseName(thing.Name)]*2 { // 无附魔的不要
				newCommodity = store{
					Duration: nextItemID(),
					Type:     "pole",
					Name:     thing.Name,
					Number:   1,
					Price:    pice,
					Other:    thing.Other,
				}
				polelist, _ := dbdata.getStoreThingInfo(thing.Name)
				if len(polelist) > 5 { // 超出上限的不要
					newCommodity.Type = "waste"
				}
			}
		} else {
			things, err1 := dbdata.getStoreThingInfo(thingName)
			if err1 != nil {
				ctx.SendChain(message.Text("[ERROR at store.go.8]:", err1))
				return
			}
			if len(things) == 0 {
				things = append(things, store{
					Duration: nextItemID(),
					Name:     thing.Name,
					Price:    pice,
					Type:     thing.Type,
				})
			} else {
				soldStoreBefore = things[0]
			}
			newCommodity = things[0]
			if newCommodity.Number < 255 {
				newCommodity.Number += number
				if newCommodity.Number > 255 {
					newCommodity.Number = 255
				}
			}
		}
		if newCommodity != (store{}) && newCommodity.Type != "waste" { // 不收垃圾
			soldStoreAfter = newCommodity
			err = dbdata.updateStoreInfo(newCommodity)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR at store.go.9]:", err))
				return
			}
			storeChanged = true
		}
		pice = pice * int(100-poleSaleTaxPercent) / 100
		payout = pice * number
		err = wallet.InsertWalletOf(uid, payout)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.10]:", err))
			return
		}
		walletCredited = true
		transactionDone = true
		if strings.Contains(thingName, "竿") {
			err = dbdata.updateCurseFor(uid, "sell", 1)
			if err != nil {
				logrus.Warnln(err)
			}
		}
		// 更新交易限制
		err = dbdata.updateCanSalesFor(uid, thingName, number, false)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR,记录鱼类交易数量失败，此次交易不记录]:", err))
		}

		ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("成功出售", thingName, "：", number, "个", ",你赚到了", pice*number, msg)))
	})
	engine.OnRegex(`^出售所有垃圾`, getdb, refreshFish).SetBlock(true).Limit(limitSet).Handle(func(ctx *zero.Ctx) {
		uid := ctx.Event.UserID
		defer lockUser(uid)()

		articles, err := dbdata.getUserTypeInfo(uid, "waste")
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:获取背包信息错误", err))
			return
		}
		if len(articles) == 0 {
			ctx.SendChain(message.Text("你的背包不存在该物品"))
			return
		}
		if len(articles) > 1 {
			msg := make(message.Message, 0, 3+len(articles))
			msg = append(msg, message.Reply(ctx.Event.MessageID), message.Text("找到以下物品:\n"))
			for i, info := range articles {
				msg = append(msg, message.Text(
					"[", i, "]", info.Name, "  数量: ", info.Number, "\n"))
			}
			ctx.Send(msg)
		}

		pice := 0
		for _, info := range articles {
			pice += (priceList[info.Name] * discountList[info.Name] / 100) * info.Number * 8 / 10
		}

		ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("是否接受回收站将以", pice, "收购全部垃圾", "?\n回答\"是\"或\"否\"")))
		// 等待用户下一步选择
		recv, cancel1 := zero.NewFutureEvent("message", 999, false, zero.RegexRule(`^(是|否)$`), zero.CheckUser(ctx.Event.UserID), zero.CheckGroup(ctx.Event.GroupID)).Repeat()
		defer cancel1()
		buy := false
		for {
			select {
			case <-time.After(time.Second * 60):
				ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("等待超时,取消出售垃圾")))
				return
			case e := <-recv:
				nextcmd := e.Event.Message.String()
				if nextcmd == "否" {
					ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("已取消出售")))
					return
				}
				buy = true
			}
			if buy {
				break
			}
		}

		tradeMu.Lock()
		defer tradeMu.Unlock()
		msg := ""
		curse, err := dbdata.getNumberFor(uid, "宝藏诅咒")
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.9.3]:", err))
			return
		}
		if curse != 0 {
			msg = "\n(你身上绑定了" + strconv.Itoa(curse) + "层诅咒)"
			pice = pice * (100 - 10*curse) / 100
		}

		changed := make([]article, 0, len(articles))
		transactionDone := false
		defer func() {
			if transactionDone {
				return
			}
			for _, info := range changed {
				_ = dbdata.updateUserThingInfo(uid, info)
			}
		}()
		for _, info := range articles {
			original := info
			info.Number = 0
			err = dbdata.updateUserThingInfo(uid, info)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR at store.go.6]:", err))
				return
			}
			changed = append(changed, original)
		}
		err = wallet.InsertWalletOf(uid, pice)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR，出售垃圾失败，回收站卷款跑路了]:", err))
			return
		}
		transactionDone = true
		ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("出售成功,你赚到了", pice, msg)))
	})
	engine.OnRegex(`^购买(`+strings.Join(thingList, "|")+`|初始木竿)\s*(\d*)$`, getdb, refreshFish).SetBlock(true).Limit(limitSet).Handle(func(ctx *zero.Ctx) {
		uid := ctx.Event.UserID
		defer lockUser(uid)()
		thingName := ctx.State["regex_matched"].([]string)[1]
		number, countErr := parseCount(ctx.State["regex_matched"].([]string)[2])
		if countErr != nil {
			ctx.SendChain(message.Text("请输入正确的数量"))
			return
		}
		if strings.Contains(thingName, "竿") {
			number = 1
		}

		numberOfPole, err := dbdata.getNumberFor(uid, "竿")
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.9.3]:", err))
			return
		}
		if numberOfPole > 50 {
			ctx.SendChain(message.Text("你有", numberOfPole, "支鱼竿,大于50支的玩家不允许购买东西"))
			return
		}

		// 检测物品交易次数
		number, err = dbdata.checkCanSalesFor(uid, thingName, number, true)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.75]:", err))
			return
		}
		if number <= 0 {
			var msg string
			if strings.Contains(thingName, "竿") {
				msg = "一天只能交易10把鱼竿,明天再来购买吧"
			} else {
				msg = "一天只能购买30次物品，明天再来吧~"
			}
			ctx.SendChain(message.Text(msg))
			return
		}

		thingInfos, err := dbdata.getStoreThingInfo(thingName)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.11]:", err))
			return
		}
		if len(thingInfos) == 0 {
			ctx.SendChain(message.Text("当前商店并没有上架该物品"))
			return
		}
		if thingName == "净化书" {
			curse, err := dbdata.getNumberFor(uid, "宝藏诅咒")
			if err != nil {
				ctx.SendChain(message.Text("[ERROR at store.go.303]:", err))
				return
			}
			if curse == 0 {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("你身上没有诅咒,不能购买该商品"))
				return
			}
			bless, err := dbdata.getNumberFor(uid, "净化书")
			if err != nil {
				ctx.SendChain(message.Text("[ERROR at store.go.303]:", err))
				return
			}
			if bless >= curse {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("你身上净化书已足够,不能购买该商品"))
				return
			}
			if curse < number {
				number = curse
			}
		}
		index := 0
		pice := make([]int, 0, len(thingInfos))
		for _, info := range thingInfos {
			thingPice, valuationErr := calculateItemMarketValue(info.Name, info.Other)
			if valuationErr != nil {
				ctx.SendChain(message.Text("[ERROR at store.go]:", valuationErr))
				return
			}
			pice = append(pice, thingPice)
		}
		if len(thingInfos) > 1 {
			msg := make(message.Message, 0, 3+len(thingInfos))
			msg = append(msg, message.Text("找到以下物品:\n"))
			for i, info := range thingInfos {
				if strings.Contains(thingName, "竿") || thingName == "三叉戟" {
					msg = append(msg, message.Text(
						"[", i, "]", info.Name, "(", info.Other, ") 价格:", pice[i], "\n"))
				} else {
					msg = append(msg, message.Text(
						"[", i, "]", info.Name, "  数量:", info.Number, "  价格:", pice[i], "\n"))
				}
			}
			msg = append(msg, message.Text("————————\n输入对应序号进行购买,或回复“取消”取消"))
			ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, msg...))
			// 等待用户下一步选择
			sell := false
			recv, cancel := zero.NewFutureEvent("message", 999, false, zero.RegexRule(`^(取消|\d+)$`), zero.CheckUser(ctx.Event.UserID), zero.CheckGroup(ctx.Event.GroupID)).Repeat()
			defer cancel()
			for {
				select {
				case <-time.After(time.Second * 120):
					ctx.Send(
						message.ReplyWithMessage(ctx.Event.MessageID,
							message.Text("等待超时,取消购买"),
						),
					)
					return
				case e := <-recv:
					nextcmd := e.Event.Message.String()
					if nextcmd == "取消" {
						ctx.Send(
							message.ReplyWithMessage(ctx.Event.MessageID,
								message.Text("已取消购买"),
							),
						)
						return
					}
					index, err = strconv.Atoi(nextcmd)
					if err != nil || index < 0 || index > len(thingInfos)-1 {
						ctx.SendChain(message.At(ctx.Event.UserID), message.Text("请输入正确的序号"))
						continue
					}
					sell = true
				}
				if sell {
					break
				}
			}
		}

		thing := thingInfos[index]
		if thing.Number < number {
			ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("商店数量不足")))
			return
		}
		price := pice[index] * number

		msg := ""
		times := math.Min(3, number)
		coupon, err := dbdata.getCouponFor(uid, times)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.9.2]:", err))
			return
		}
		if coupon != -1 {
			msg += "\n(半价福利还有" + strconv.Itoa(3-coupon) + "次)"
			price = pice[index]*(number-coupon) + (pice[index]/2)*coupon
		}
		curse, err := dbdata.getNumberFor(uid, "宝藏诅咒")
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.9.3]:", err))
			return
		}
		if curse != 0 {
			msg += "\n(你身上绑定了" + strconv.Itoa(curse) + "层诅咒)"
			price = price * (100 + 10*curse) / 100
		}

		money := wallet.GetWalletOf(uid)
		if money < price {
			ctx.SendChain(message.Text("你身上的钱(", money, ")不够支付", msg))
			return
		}

		ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("你确定花费", price, "购买", number, "个", thingName, "?", msg, "\n回答\"是\"或\"否\"")))
		// 等待用户下一步选择
		recv, cancel1 := zero.NewFutureEvent("message", 999, false, zero.RegexRule(`^(是|否)$`), zero.CheckUser(ctx.Event.UserID), zero.CheckGroup(ctx.Event.GroupID)).Repeat()
		defer cancel1()
		buy := false
		for {
			select {
			case <-time.After(time.Second * 60):
				ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("等待超时,取消购买")))
				return
			case e := <-recv:
				nextcmd := e.Event.Message.String()
				if nextcmd == "否" {
					ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("已取消购买")))
					return
				}
				buy = true
			}
			if buy {
				break
			}
		}

		tradeMu.Lock()
		defer tradeMu.Unlock()
		money = wallet.GetWalletOf(uid)
		if money < price {
			ctx.SendChain(message.Text("确认购买时余额已变化，当前余额(", money, ")不够支付"))
			return
		}
		var ok bool
		thing, ok, err = dbdata.checkStoreFor(thing, number)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.11]:", err))
			return
		}
		if !ok {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("你慢了一步,物品被别人买走了"))
			return
		}
		storeBefore := thing
		transactionDone := false
		walletDebited := false
		purchaseItemChanged := false
		var purchaseBefore article
		var newCommodity article
		defer func() {
			if transactionDone {
				return
			}
			if walletDebited {
				_ = wallet.InsertWalletOf(uid, price)
			}
			_ = dbdata.updateStoreInfo(storeBefore)
			if purchaseItemChanged {
				_ = dbdata.updateUserThingInfo(uid, purchaseBefore)
			}
		}()
		thing.Number -= number
		err = dbdata.updateStoreInfo(thing)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.12]:", err))
			return
		}
		err = wallet.InsertWalletOf(uid, -price)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.13]:", err))
			return
		}
		walletDebited = true
		if strings.Contains(thingName, "竿") || thingName == "三叉戟" {
			newCommodity = article{
				Duration: nextItemID(),
				Type:     thing.Type,
				Name:     thing.Name,
				Number:   1,
				Other:    thing.Other,
			}
			if thingName == "初始木竿" {
				newCommodity.Name = "木竿"
			}
			purchaseBefore = newCommodity
			purchaseBefore.Number = 0
		} else {
			things, err1 := dbdata.getUserThingInfo(uid, thingName)
			if err1 != nil {
				ctx.SendChain(message.Text("[ERROR at store.go.15]:", err1))
				return
			}
			if len(things) == 0 {
				things = append(things, article{
					Duration: nextItemID(),
					Name:     thing.Name,
					Type:     thing.Type,
				})
				purchaseBefore = things[0]
			} else {
				purchaseBefore = things[0]
			}
			newCommodity = things[0]
			newCommodity.Number += number
		}
		err = dbdata.updateUserThingInfo(uid, newCommodity)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.14]:", err))
			return
		}
		purchaseItemChanged = true
		if coupon != -1 {
			if _, couponErr := dbdata.useCouponAt(uid, coupon); couponErr != nil {
				ctx.SendChain(message.Text("[ERROR at store.go.16]:", couponErr))
				return
			}
		} else if buyErr := dbdata.updateBuyTimeFor(uid, 1); buyErr != nil {
			ctx.SendChain(message.Text("[ERROR at store.go.16]:", buyErr))
			return
		}
		transactionDone = true
		if strings.Contains(thingName, "竿") {
			err = dbdata.updateCurseFor(uid, "buy", 1)
			if err != nil {
				logrus.Warnln(err)
			}
		}
		// 更新交易限制
		err = dbdata.updateCanSalesFor(uid, thingName, number, true)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR,记录鱼类交易数量失败，此次交易不记录]:", err))
		}
		ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text("你用", price, "购买了", number, thingName)))
	})
}

func drawStroeEmptyImage() (picImage image.Image, err error) {
	fontdata, err := file.GetLazyData(text.BoldFontFile, control.Md5File, true)
	if err != nil {
		return nil, err
	}
	canvas := gg.NewContext(1000, 300)
	// 画底色
	canvas.DrawRectangle(0, 0, 1000, 300)
	canvas.SetRGBA255(255, 255, 255, 150)
	canvas.Fill()
	// 边框框
	canvas.DrawRectangle(0, 0, 1000, 300)
	canvas.SetLineWidth(3)
	canvas.SetRGBA255(0, 0, 0, 255)
	canvas.Stroke()

	canvas.SetColor(color.Black)
	err = canvas.ParseFontFace(fontdata, 100)
	if err != nil {
		return nil, err
	}
	textW, textH := canvas.MeasureString("价格信息")
	canvas.DrawString("价格信息", 10, 10+textH*1.2)
	canvas.DrawLine(10, textH*1.6, textW, textH*1.6)
	canvas.SetLineWidth(3)
	canvas.SetRGBA255(0, 0, 0, 255)
	canvas.Stroke()
	if err = canvas.ParseFontFace(fontdata, 50); err != nil {
		return nil, err
	}
	canvas.DrawStringAnchored("当前商店并没有上架任何物品", 500, 10+textH*2+50, 0.5, 0)
	return canvas.Image(), nil
}

func drawStroeInfoImage(stroeInfo []store) (picImage image.Image, err error) {
	fontdata, err := file.GetLazyData(text.BoldFontFile, control.Md5File, true)
	if err != nil {
		return nil, err
	}
	canvas := gg.NewContext(1, 1)
	err = canvas.ParseFontFace(fontdata, 100)
	if err != nil {
		return nil, err
	}
	titleW, titleH := canvas.MeasureString("价格信息")

	err = canvas.ParseFontFace(fontdata, 50)
	if err != nil {
		return nil, err
	}
	_, textH := canvas.MeasureString("高度")
	nameW, _ := canvas.MeasureString("下界合金竿(100/100/0/0)")
	numberW, _ := canvas.MeasureString("10000")
	priceW, _ := canvas.MeasureString("10000")

	bolckW := int(10 + nameW + 50 + numberW + 50 + priceW + 10)
	backY := 10 + int(titleH*2+10)*2 + 10 + (len(stroeInfo)+len(discountList)/2+2)*int(textH*2) + 10
	canvas = gg.NewContext(bolckW, math.Max(backY, 500))
	// 画底色
	canvas.DrawRectangle(0, 0, float64(bolckW), float64(backY))
	canvas.SetRGBA255(150, 150, 150, 255)
	canvas.Fill()

	// 放字
	canvas.SetColor(color.Black)
	err = canvas.ParseFontFace(fontdata, 100)
	if err != nil {
		return nil, err
	}
	canvas.DrawString("今日波动", 10, 10+titleH*1.2)
	canvas.DrawLine(10, titleH*1.6, titleW, titleH*1.6)
	canvas.SetLineWidth(3)
	canvas.SetRGBA255(0, 0, 0, 255)
	canvas.Stroke()

	textDy := 10 + titleH*1.7
	if err = canvas.ParseFontFace(fontdata, 35); err != nil {
		return nil, err
	}
	textDx, textDh := canvas.MeasureString("下界合金竿(均价1000)")
	valueDx, _ := canvas.MeasureString("+100%")
	i := 0
	for _, name := range thingList {
		text := name + "(均价" + strconv.Itoa(priceList[name]) + ") "

		if i == 2 {
			i = 0
			textDy += textDh * 2
		}
		canvas.SetColor(color.Black)
		canvas.DrawStringAnchored(text, 20+(textDx+valueDx+10)*float64(i)+10, textDy+textDh/2, 0, 0.5)
		if discountList[name]-100 > 0 {
			canvas.SetRGBA255(200, 50, 50, 255)
			text = "+" + strconv.Itoa(discountList[name]-100) + "%"
		} else {
			canvas.SetRGBA255(63, 133, 55, 255)
			text = strconv.Itoa(discountList[name]-100) + "%"
		}
		canvas.DrawStringAnchored(text, 20+(textDx+valueDx+10)*float64(i)+10+textDx+10, textDy+textDh/2, 0, 0.5)
		i++
	}
	canvas.SetColor(color.Black)
	textDy += textDh * 2
	canvas.DrawStringAnchored("注:出售商品将会额外扣除20%的税收,附魔鱼竿请按实际价格", 10, textDy+10+textDh/2, 0, 0.5)

	textDy += textH * 2
	err = canvas.ParseFontFace(fontdata, 100)
	if err != nil {
		return nil, err
	}
	canvas.DrawString("上架内容", 10, textDy+titleH*1.2)
	canvas.DrawLine(10, textDy+titleH*1.6, titleW, textDy+titleH*1.6)
	canvas.SetLineWidth(3)
	canvas.SetRGBA255(0, 0, 0, 255)
	canvas.Stroke()

	textDy += 10 + titleH*1.7
	if err = canvas.ParseFontFace(fontdata, 50); err != nil {
		return nil, err
	}

	canvas.DrawStringAnchored("名称", 10+nameW/2, textDy+textH/2, 0.5, 0.5)
	canvas.DrawStringAnchored("数量", 10+nameW+10+numberW/2, textDy+textH/2, 0.5, 0.5)
	canvas.DrawStringAnchored("价格", 10+nameW+10+numberW+50+priceW/2, textDy+textH/2, 0.5, 0.5)

	for _, info := range stroeInfo {
		textDy += textH * 2
		name := info.Name
		if info.Other != "" && info.Name != "美西螈" {
			name += "(" + info.Other + ")"
		}
		numberStr := strconv.Itoa(info.Number)
		pice, err := calculateItemMarketValue(info.Name, info.Other)
		if err != nil {
			return nil, err
		}

		canvas.DrawStringAnchored(name, 10+nameW/2, textDy+textH/2, 0.5, 0.5)
		canvas.DrawStringAnchored(numberStr, 10+nameW+10+numberW/2, textDy+textH/2, 0.5, 0.5)
		canvas.DrawStringAnchored(strconv.Itoa(pice), 10+nameW+10+numberW+50+priceW/2, textDy+textH/2, 0.5, 0.5)
	}
	return canvas.Image(), nil
}
