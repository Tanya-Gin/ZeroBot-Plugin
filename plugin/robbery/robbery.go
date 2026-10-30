// Package robbery 打劫群友  基于“qqwife”插件魔改
package robbery

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	fcext "github.com/FloatTech/floatbox/ctxext"
	sql "github.com/FloatTech/sqlite"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"

	"github.com/FloatTech/AnimeAPI/wallet"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

type robberyRepo struct {
	sync.RWMutex
	db sql.Sqlite
}

type robberyRecord struct {
	UserID   int64  `db:"user_id"`   // 劫匪
	VictimID int64  `db:"victim_id"` // 受害者
	Time     string `db:"time"`      // 时间
	Result   string `db:"result"`    // success/failed
}

type robberyAttempt struct {
	ID       int64  `db:"id"`
	UserID   int64  `db:"user_id"`
	VictimID int64  `db:"victim_id"`
	Time     string `db:"time"`
	Result   string `db:"result"`
}

type robberyBlockReason int

const (
	robberyAllowed robberyBlockReason = iota
	robberyBlockRobberDaily
	robberyBlockVictimDaily
	robberyBlockVictimCooldown
)

const (
	robberyResultPending  = "pending"
	robberyResultSuccess  = "success"
	robberyResultFailed   = "failed"
	robberyVictimCooldown = 30 * time.Minute
	robberyMinWallet      = 1000
)

const robberyCommandPattern = `^打劫\s?(\[CQ:at,(?:[^,]*,)?qq=(\d+)(?:,[^,]*)?\]|(\d+))`

func canRobberyTarget(robberID, victimID, botID int64) bool {
	return victimID != robberID && victimID != botID
}

func evaluateRobberyAttempt(records []robberyRecord, robberID, victimID int64, now time.Time) robberyBlockReason {
	for _, record := range records {
		if record.Time == "" {
			continue
		}
		timestamp, err := time.Parse(time.RFC3339Nano, record.Time)
		if err != nil {
			// 兼容旧版按天保存的记录：旧记录只保留日期，视为当天的已占用记录。
			if record.Time == now.Format("2006/01/02") {
				if record.UserID == robberID {
					return robberyBlockRobberDaily
				}
				if record.VictimID == victimID {
					return robberyBlockVictimDaily
				}
			}
			continue
		}
		if sameDay(timestamp, now) && record.UserID == robberID {
			return robberyBlockRobberDaily
		}
		if record.VictimID != victimID {
			continue
		}
		if record.Result == robberyResultSuccess && sameDay(timestamp, now) {
			return robberyBlockVictimDaily
		}
		if (record.Result == robberyResultFailed || record.Result == robberyResultPending) && now.Sub(timestamp) < robberyVictimCooldown {
			return robberyBlockVictimCooldown
		}
	}
	return robberyAllowed
}

func sameDay(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func robberyTransferAmount(gain, victimWallet int) int {
	if gain < 0 || victimWallet < 0 {
		return 0
	}
	if gain > victimWallet {
		return victimWallet
	}
	return gain
}

func ensureRobberySchema(db *sql.Sqlite) error {
	if err := db.Create("criminal_record", &robberyRecord{}); err != nil {
		return err
	}
	// 旧版表只有 user_id、victim_id、time；旧记录按成功处理。
	if _, err := db.Exec("ALTER TABLE 'criminal_record' ADD COLUMN result TEXT NOT NULL DEFAULT 'success'"); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS robbery_attempts (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id BIGINT NOT NULL, victim_id BIGINT NOT NULL, time TEXT NOT NULL, result TEXT NOT NULL)"); err != nil {
		return err
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS robbery_attempts_user_time ON robbery_attempts(user_id, time)"); err != nil {
		return err
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS robbery_attempts_victim_time ON robbery_attempts(victim_id, time)"); err != nil {
		return err
	}
	return nil
}

func init() {
	var police robberyRepo
	engine := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "打劫别人的钱包",
		Help: "- 打劫[对方Q号|@对方QQ]\n" +
			"1. 劫匪和受害者钱包都必须至少有1000\n" +
			"2. 成功率35%；成功获得劫匪本次风险金额的1.5倍（最高10000），受害者实际损失相同\n" +
			"3. 失败损失为劫匪钱包的5%，最低1000、最高10000\n" +
			"4. 失败后有25%概率被捕；7天内每次入狱再增加10%，最高50%\n" +
			"5. 首次入狱12小时，7天内第2次18小时，第3次及以上24小时\n" +
			"6. 保释金按风险金额和7天内入狱次数计算，最高50000；本人或群友均可缴纳\n" +
			"7. 每日每人最多发起一次有效打劫；成功后受害者当天免疫，失败后有30分钟冷却\n" +
			"8. 服刑期间仅可查看监狱/刑期、办理保释、使用帮助/用法，管理员可强制释放\n",
		PrivateDataFolder: "robbery",
	}).ApplySingle(ctxext.NewGroupSingle("别着急，警察局门口排长队了！"))
	configurePrison(engine.DataFolder() + "prison.db")
	getdb := fcext.DoOnceOnSuccess(func(ctx *zero.Ctx) bool {
		police.db = sql.New(engine.DataFolder() + "robbery.db")
		err := police.db.Open(time.Hour)
		if err == nil {
			err = ensureRobberySchema(&police.db)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return false
			}
			return true
		}
		ctx.SendChain(message.Text("[ERROR]:", err))
		return false
	})

	// 打劫功能
	engine.OnRegex(robberyCommandPattern, getdb).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(func(ctx *zero.Ctx) {
			uid := ctx.Event.UserID
			fiancee := ctx.State["regex_matched"].([]string)
			victimID, _ := strconv.ParseInt(fiancee[2]+fiancee[3], 10, 64)
			if !canRobberyTarget(uid, victimID, ctx.Event.SelfID) {
				if victimID == uid {
					ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.At(uid), message.Text("不能打劫自己")))
				} else {
					ctx.SendChain(message.Text("不能打劫", botName()))
				}
				return
			}

			// 没有足够本金就不允许发起，避免“没钱也能无限刷概率”。
			if wallet.GetWalletOf(uid) < robberyMinWallet {
				ctx.SendChain(message.Text("你的钱包至少需要", robberyMinWallet, wallet.GetWalletName(), "才能打劫"))
				return
			}

			risk := robberyRisk(wallet.GetWalletOf(uid))
			if risk < robberyMinWallet {
				ctx.SendChain(message.Text("你的钱包至少需要", robberyMinWallet, wallet.GetWalletName(), "才能承担打劫风险"))
				return
			}
			victimWallet := wallet.GetWalletOf(victimID)
			if victimWallet < robberyMinWallet {
				ctx.SendChain(message.Text("对方太穷了！需要至少", robberyMinWallet, wallet.GetWalletName(), "才能被打劫"))
				return
			}

			// 在随机判定前占用当天尝试次数，成功和失败都不能绕过每日限制。
			reason, err := police.reserveAttempt(uid, victimID, time.Now())
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			switch reason {
			case robberyBlockVictimDaily:
				ctx.SendChain(message.Text("对方今天已经被打劫了，给人家留点后路吧"))
				return
			case robberyBlockRobberDaily:
				ctx.SendChain(message.Text("你今天已经尝试过打劫了，明天再来吧"))
				return
			case robberyBlockVictimCooldown:
				ctx.SendChain(message.Text("对方刚刚躲过一次打劫，30分钟内不能再次打劫"))
				return
			}

			// 35% success; otherwise charge the actual risk and make an independent arrest roll.
			if rand.Intn(100) >= robberySuccessPercent {
				if err := wallet.InsertWalletOf(uid, -risk); err != nil {
					ctx.SendChain(message.Text("[ERROR]:风险扣款失败，钱包操作异常。"))
					return
				}
				now := time.Now()
				if completeErr := police.completeAttempt(uid, victimID, now, robberyResultFailed); completeErr != nil {
					_ = wallet.InsertWalletOf(uid, risk)
					ctx.SendChain(message.Text("[ERROR]:犯罪记录更新失败，风险金额已尝试退回。"))
					return
				}
				history, historyErr := prisonRepo.history(uid)
				if historyErr != nil {
					ctx.SendChain(message.Text("打劫失败，损失", risk, wallet.GetWalletName(), "；监狱档案读取失败，未进行抓捕判定。"))
					return
				}
				recent := recentArrestCount(history, now)
				if rand.Intn(100) < arrestChance(recent) {
					prison, arrestErr := prisonRepo.incarcerate(uid, risk, recent, now)
					if arrestErr != nil {
						ctx.SendChain(message.Text("打劫失败，损失", risk, wallet.GetWalletName(), "；抓捕登记失败，请联系管理员核查。"))
						return
					}
					ctx.SendChain(message.At(uid), message.Text("打劫失败，损失", risk, wallet.GetWalletName(), "，而且被警察当场逮住！判处", int(prisonDuration(recent).Hours()), "小时。保释金：", prison.Bail, wallet.GetWalletName(), "。发送“查看监狱”查看详情。"))
					return
				}
				ctx.SendChain(message.Text("打劫失败，损失", risk, wallet.GetWalletName(), "；这次侥幸逃脱，但今天的打劫次数已用完。"))
				return
			}

			userIncrMoney := robberyReward(risk, victimWallet)

			// 方案A：成功时从受害者转出与劫匪获得额完全相同的金额，不再凭空生成货币。
			err = wallet.InsertWalletOf(victimID, -userIncrMoney)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:钱包坏掉力:\n", err))
				return
			}
			err = wallet.InsertWalletOf(uid, +userIncrMoney)
			if err != nil {
				// 给劫匪入账失败时尽力回滚受害者扣款。
				_ = wallet.InsertWalletOf(victimID, +userIncrMoney)
				_ = police.completeAttempt(uid, victimID, time.Now(), robberyResultFailed)
				ctx.SendChain(message.Text("[ERROR]:打劫失败，钱包变更已回滚\n", err))
				return
			}

			if err = police.completeAttempt(uid, victimID, time.Now(), robberyResultSuccess); err != nil {
				ctx.SendChain(message.Text("[ERROR]:犯罪记录更新失败\n", err))
				return
			}

			ctx.SendChain(message.At(uid), message.Text("打劫成功，钱包增加：", userIncrMoney, wallet.GetWalletName()))
			ctx.SendChain(message.At(victimID), message.Text("您被打劫，实际损失：", userIncrMoney, wallet.GetWalletName()))
		})
}

func (repo *robberyRepo) reserveAttempt(robberID, victimID int64, now time.Time) (robberyBlockReason, error) {
	repo.Lock()
	defer repo.Unlock()

	records, err := repo.recordsFor(robberID, victimID)
	if err != nil {
		return robberyAllowed, err
	}
	reason := evaluateRobberyAttempt(records, robberID, victimID, now)
	if reason != robberyAllowed {
		return reason, nil
	}
	return robberyAllowed, func() error {
		_, err := repo.db.Exec("INSERT INTO robbery_attempts (user_id, victim_id, time, result) VALUES (?, ?, ?, ?)", robberID, victimID, now.Format(time.RFC3339Nano), robberyResultPending)
		return err
	}()
}

func (repo *robberyRepo) completeAttempt(robberID, victimID int64, now time.Time, result string) error {
	repo.Lock()
	defer repo.Unlock()
	_, err := repo.db.Exec("UPDATE robbery_attempts SET time = ?, result = ? WHERE id = (SELECT id FROM robbery_attempts WHERE user_id = ? AND victim_id = ? AND result = ? ORDER BY id DESC LIMIT 1)", now.Format(time.RFC3339Nano), result, robberID, victimID, robberyResultPending)
	return err
}

func (repo *robberyRepo) recordsFor(robberID, victimID int64) ([]robberyRecord, error) {
	rows, err := sql.FindAll[robberyRecord](&repo.db, "criminal_record", "WHERE user_id = ? OR victim_id = ?", robberID, victimID)
	if err != nil && err != sql.ErrNullResult {
		return nil, err
	}
	records := make([]robberyRecord, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			records = append(records, *row)
		}
	}
	attempts, attemptErr := sql.FindAll[robberyAttempt](&repo.db, "robbery_attempts", "WHERE user_id = ? OR victim_id = ?", robberID, victimID)
	if attemptErr != nil && attemptErr != sql.ErrNullResult {
		return nil, attemptErr
	}
	for _, row := range attempts {
		if row != nil {
			records = append(records, robberyRecord{UserID: row.UserID, VictimID: row.VictimID, Time: row.Time, Result: row.Result})
		}
	}
	return records, nil
}
