package robbery

import (
	"fmt"
	errors "github.com/pkg/errors"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FloatTech/AnimeAPI/wallet"
	sql "github.com/FloatTech/sqlite"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

const (
	robberySuccessPercent     = 35
	robberyArrestBasePercent  = 25
	robberyArrestStepPercent  = 10
	robberyArrestMaxPercent   = 50
	robberyRiskPercent        = 5
	robberyRiskMin            = 1000
	robberyRiskMax            = 10000
	robberyRewardRiskMultiple = 3 // 1.5x risk expressed in half-units
	robberyBailRiskMultiple   = 2
	robberyBailMin            = 2000
	robberyBailMax            = 50000
	robberyRewardMax          = 10000
	robberyRepeatWindow       = 7 * 24 * time.Hour
	prisonMatcherPriority     = -1000
)

const (
	prisonResultArrested = "arrested"
	prisonResultReleased = "released"
)

type prisonRecord struct {
	UserID     int64  `db:"user_id"`
	ArrestedAt string `db:"arrested_at"`
	Until      string `db:"until"`
	Bail       int    `db:"bail"`
	Arrests    int    `db:"arrests"`
	Status     string `db:"status"`
	History    string `db:"history"`
}

type prisonRepository struct {
	mu sync.Mutex
	db sql.Sqlite
}

var prisonRepo *prisonRepository
var prisonOpenOnce sync.Once
var prisonOpenErr error

func configurePrison(path string) {
	prisonOpenOnce.Do(func() {
		prisonRepo = &prisonRepository{db: sql.New(path)}
		if err := prisonRepo.db.Open(time.Hour); err != nil {
			prisonOpenErr = err
			return
		}
		prisonOpenErr = prisonRepo.db.Create("prison", &prisonRecord{})
	})
}

func (repo *prisonRepository) active(uid int64, now time.Time) (prisonRecord, bool, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	row := prisonRecord{}
	err := repo.db.FindFor("prison", &row, "WHERE user_id = ?", func() error { return nil }, uid)
	if err == sql.ErrNullResult {
		return prisonRecord{}, false, nil
	}
	if err != nil {
		return prisonRecord{}, false, err
	}
	if row.Status != prisonResultArrested {
		return row, false, nil
	}
	until, err := time.Parse(time.RFC3339Nano, row.Until)
	if err != nil {
		return row, false, err
	}
	if !now.Before(until) {
		_, err = repo.db.Exec("UPDATE prison SET status = ? WHERE user_id = ?", prisonResultReleased, uid)
		return row, false, err
	}
	return row, true, nil
}

func recentArrestCount(records []prisonRecord, now time.Time) int {
	count := 0
	for _, r := range records {
		for _, value := range strings.Split(r.History, ",") {
			t, err := time.Parse(time.RFC3339Nano, value)
			if err == nil && !t.After(now) && now.Sub(t) < robberyRepeatWindow {
				count++
			}
		}
	}
	return count
}

func robberyRisk(walletBalance int) int {
	if walletBalance <= 0 {
		return 0
	}
	risk := int(math.Ceil(float64(walletBalance) * robberyRiskPercent / 100))
	if risk < robberyRiskMin {
		risk = robberyRiskMin
	}
	if risk > robberyRiskMax {
		risk = robberyRiskMax
	}
	if risk > walletBalance {
		risk = walletBalance
	}
	return risk
}

func robberyReward(risk, victimBalance int) int {
	reward := risk * robberyRewardRiskMultiple / 2
	if reward > robberyRewardMax {
		reward = robberyRewardMax
	}
	if reward > victimBalance {
		reward = victimBalance
	}
	return robberyTransferAmount(reward, victimBalance)
}

func arrestChance(recent int) int {
	chance := robberyArrestBasePercent + max(0, recent)*robberyArrestStepPercent
	return min(chance, robberyArrestMaxPercent)
}

func prisonDuration(recent int) time.Duration {
	switch {
	case recent <= 0:
		return 12 * time.Hour
	case recent == 1:
		return 18 * time.Hour
	default:
		return 24 * time.Hour
	}
}

func bailAmount(risk, recent int) int {
	base := max(robberyBailMin, risk*robberyBailRiskMultiple)
	amount := base * (100 + 50*max(0, recent)) / 100
	return min(amount, robberyBailMax)
}

func (repo *prisonRepository) history(uid int64) ([]prisonRecord, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	rows, err := sql.FindAll[prisonRecord](&repo.db, "prison", "WHERE user_id = ?", uid)
	if err == sql.ErrNullResult {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]prisonRecord, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			out = append(out, *row)
		}
	}
	return out, nil
}

func (repo *prisonRepository) incarcerate(uid int64, risk, recent int, now time.Time) (prisonRecord, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	row := prisonRecord{UserID: uid}
	err := repo.db.FindFor("prison", &row, "WHERE user_id = ?", func() error { return nil }, uid)
	if err != nil && err != sql.ErrNullResult {
		return prisonRecord{}, err
	}
	until := now.Add(prisonDuration(recent))
	history := row.History
	if history != "" {
		history += ","
	}
	history += now.Format(time.RFC3339Nano)
	next := prisonRecord{UserID: uid, ArrestedAt: now.Format(time.RFC3339Nano), Until: until.Format(time.RFC3339Nano), Bail: bailAmount(risk, recent), Arrests: recent + 1, Status: prisonResultArrested, History: history}
	if err == sql.ErrNullResult {
		err = repo.db.Insert("prison", &next)
	} else {
		_, err = repo.db.Exec("UPDATE prison SET arrested_at=?, until=?, bail=?, arrests=?, status=?, history=? WHERE user_id=?", next.ArrestedAt, next.Until, next.Bail, next.Arrests, next.Status, next.History, uid)
	}
	return next, err
}

func (repo *prisonRepository) release(uid int64) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	_, err := repo.db.Exec("UPDATE prison SET status = ? WHERE user_id = ? AND status = ?", prisonResultReleased, uid, prisonResultArrested)
	return err
}

func (repo *prisonRepository) bail(uid int64, now time.Time, charge, refund func(int) error) (prisonRecord, bool, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	row := prisonRecord{}
	err := repo.db.FindFor("prison", &row, "WHERE user_id = ?", func() error { return nil }, uid)
	if err == sql.ErrNullResult {
		return prisonRecord{}, false, nil
	}
	if err != nil {
		return prisonRecord{}, false, err
	}
	if row.Status != prisonResultArrested {
		return row, false, nil
	}
	until, err := time.Parse(time.RFC3339Nano, row.Until)
	if err != nil {
		return row, false, err
	}
	if !now.Before(until) {
		_, err = repo.db.Exec("UPDATE prison SET status = ? WHERE user_id = ?", prisonResultReleased, uid)
		return row, false, err
	}
	if err := charge(row.Bail); err != nil {
		return row, true, err
	}
	if _, err := repo.db.Exec("UPDATE prison SET status = ? WHERE user_id = ? AND status = ?", prisonResultReleased, uid, prisonResultArrested); err != nil {
		if refundErr := refund(row.Bail); refundErr != nil {
			return row, true, errors.Errorf("release update failed (%v); refund failed (%v)", err, refundErr)
		}
		return row, true, err
	}
	return row, true, nil
}

func botName() string {
	if len(zero.BotConfig.NickName) > 0 && strings.TrimSpace(zero.BotConfig.NickName[0]) != "" {
		return zero.BotConfig.NickName[0]
	}
	return "穗月"
}

var prisonNudges = []string{
	"监狱内不得使用电话，写信也不行！这里的信鸽已经被没收了。",
	"你目前正在服刑，%s没有办法在监狱和你做生意；或者你可以问问监狱里的“快递先生”？听说他什么都能送——当然，这只是个传说。",
	"牢饭今日菜单：白开水、空气，以及一点点后悔。",
	"你敲了敲牢门，没有回应。大概连牢门都觉得你该反省。",
	"狱警提醒：不要尝试发送“打劫”，这只会让你的悔恨更加具体。",
	"你向窗外望去，群友们正在快乐地和%s玩耍。",
	"监狱 Wi-Fi 密码：不存在。",
	"写信功能暂未开放，毕竟你连纸都没有。",
}

func prisonNudgeText(index int, nickname string) string {
	if len(prisonNudges) == 0 {
		return ""
	}
	text := prisonNudges[index%len(prisonNudges)]
	return strings.ReplaceAll(text, "%s", nickname)
}

func jailStatusText(record prisonRecord, now time.Time) string {
	until, err := time.Parse(time.RFC3339Nano, record.Until)
	if err != nil {
		return "刑期数据暂时无法读取，请联系管理员处理。"
	}
	left := until.Sub(now)
	if left < 0 {
		left = 0
	}
	hours := int(left / time.Hour)
	minutes := int(left % time.Hour / time.Minute)
	return fmt.Sprintf("当前状态：服刑中\n剩余刑期：%d小时%d分\n保释金：%d%s\n近期入狱次数：%d次\n可以发送“交保释金”，也可以请群友发送“保释 @你”。", hours, minutes, record.Bail, wallet.GetWalletName(), record.Arrests)
}

func ensurePrisonReady(ctx *zero.Ctx) bool {
	if prisonRepo == nil || prisonOpenErr != nil {
		ctx.SendChain(message.Text("监狱档案暂时无法打开，请稍后再试。"))
		return false
	}
	return true
}

func shouldAllowPrisonCommand(text string) bool {
	text = strings.TrimSpace(text)
	text = strings.TrimLeft(text, "/")
	return text == "查看监狱" || text == "我的刑期" || text == "交保释金" ||
		strings.HasPrefix(text, "保释") || strings.HasPrefix(text, "帮助") ||
		strings.HasPrefix(text, "用法") || strings.HasPrefix(text, "强制释放")
}

func registerPrisonCommands(engine *zero.Engine) {
	engine.OnFullMatch("查看监狱").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		if !ensurePrisonReady(ctx) {
			return
		}
		r, active, err := prisonRepo.active(ctx.Event.UserID, time.Now())
		if err != nil {
			ctx.SendChain(message.Text("监狱档案读取失败。"))
			return
		}
		if !active {
			ctx.SendChain(message.Text("你目前不在监狱里。自由的空气真好，记得珍惜。"))
			return
		}
		ctx.SendChain(message.At(ctx.Event.UserID), message.Text(jailStatusText(r, time.Now())))
	})
	engine.OnFullMatch("我的刑期").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		if !ensurePrisonReady(ctx) {
			return
		}
		r, active, err := prisonRepo.active(ctx.Event.UserID, time.Now())
		if err != nil {
			ctx.SendChain(message.Text("监狱档案读取失败。"))
			return
		}
		if !active {
			ctx.SendChain(message.Text("你目前不在监狱里。"))
			return
		}
		ctx.SendChain(message.At(ctx.Event.UserID), message.Text(jailStatusText(r, time.Now())))
	})
	engine.OnFullMatch("交保释金").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		if !ensurePrisonReady(ctx) {
			return
		}
		uid := ctx.Event.UserID
		_, active, err := prisonRepo.bail(uid, time.Now(), func(amount int) error {
			if wallet.GetWalletOf(uid) < amount {
				return errors.Errorf("余额不足")
			}
			return wallet.InsertWalletOf(uid, -amount)
		}, func(amount int) error { return wallet.InsertWalletOf(uid, amount) })
		if err != nil {
			if strings.Contains(err.Error(), "余额不足") {
				ctx.SendChain(message.Text("钱包余额不够缴纳保释金；可以请群友发送“保释 @你”。"))
			} else {
				ctx.SendChain(message.Text("保释处理失败，请联系管理员核对。"))
			}
			return
		}
		if !active {
			ctx.SendChain(message.Text("你已经自由了，不需要再交保释金。"))
			return
		}
		ctx.SendChain(message.At(uid), message.Text("你终于出狱了。外面的世界很美好，希望你没有继续物色新的作案工具。"))
	})
	engine.OnRegex(`^保释(?:\s*\[CQ:at,(?:[^,]*,)?qq=(\d+)(?:,[^,]*)?\]|\s+(\d+))?$`).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		if !ensurePrisonReady(ctx) {
			return
		}
		matches, _ := ctx.State["regex_matched"].([]string)
		target := ctx.Event.UserID
		if len(matches) > 2 && (matches[1] != "" || matches[2] != "") {
			target, _ = strconv.ParseInt(matches[1]+matches[2], 10, 64)
		}
		if target == 0 || target == ctx.Event.SelfID {
			ctx.SendChain(message.Text("保释对象无效。"))
			return
		}
		payer := ctx.Event.UserID
		r, active, err := prisonRepo.bail(target, time.Now(), func(amount int) error {
			if wallet.GetWalletOf(payer) < amount {
				return errors.Errorf("余额不足")
			}
			return wallet.InsertWalletOf(payer, -amount)
		}, func(amount int) error { return wallet.InsertWalletOf(payer, amount) })
		if err != nil {
			if strings.Contains(err.Error(), "余额不足") {
				ctx.SendChain(message.Text("你的钱包余额不够缴纳这笔保释金。"))
			} else {
				ctx.SendChain(message.Text("保释处理失败，请联系管理员核对。"))
			}
			return
		}
		if !active {
			ctx.SendChain(message.Text("这个人已经不在监狱里了。"))
			return
		}
		ctx.SendChain(message.At(target), message.At(payer), message.Text("缴纳了", r.Bail, wallet.GetWalletName(), "，"), message.Text("你终于出狱了。外面的世界很美好，希望你没有继续物色新的作案工具。"))
	})
	engine.OnRegex(`^强制释放\s*(\d+)$`, zero.AdminPermission).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		if !ensurePrisonReady(ctx) {
			return
		}
		uid, _ := strconv.ParseInt(ctx.State["regex_matched"].([]string)[1], 10, 64)
		if err := prisonRepo.release(uid); err != nil {
			ctx.SendChain(message.Text("强制释放失败，请检查监狱记录。"))
			return
		}
		ctx.SendChain(message.Text("已为该用户解除监狱状态。"))
	})
	engine.OnMessage(func(ctx *zero.Ctx) bool {
		if prisonRepo == nil || prisonOpenErr != nil || ctx.Event.UserID == 0 || ctx.Event.UserID == ctx.Event.SelfID {
			return false
		}
		text := ctx.Event.RawMessage
		allowed := shouldAllowPrisonCommand(text)
		if allowed {
			return false
		}
		_, active, err := prisonRepo.active(ctx.Event.UserID, time.Now())
		return err != nil || active
	}).SetPriority(prisonMatcherPriority).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		idx := rand.Intn(len(prisonNudges))
		ctx.SendChain(message.At(ctx.Event.UserID), message.Text(prisonNudgeText(idx, botName())))
	})
}

func init() {
	// Register the globally-prioritized prison gate and administrative/user escape hatches.
	engine := zero.New()
	registerPrisonCommands(engine)
}
