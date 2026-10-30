package cybercat

import (
	"math/rand"
	"sort"
	"sync"
	"time"

	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

const (
	petWindow          = 10 * time.Minute
	petTriggerCount    = int64(5)
	petAwayProbability = 20
	awayDuration       = 24 * time.Hour
	awayReturnChance   = 35
	awayPenalty        = 100
)

var catLocks sync.Map

func lockCat(gid, uid string) *sync.Mutex {
	key := gid + ":" + uid
	lock, _ := catLocks.LoadOrStore(key, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func lockCats(gid, uid1, uid2 string) func() {
	keys := []string{gid + ":" + uid1, gid + ":" + uid2}
	sort.Strings(keys)
	locks := make([]*sync.Mutex, 0, len(keys))
	for i, key := range keys {
		if i > 0 && key == keys[i-1] {
			continue
		}
		value, _ := catLocks.LoadOrStore(key, &sync.Mutex{})
		lock := value.(*sync.Mutex)
		lock.Lock()
		locks = append(locks, lock)
	}
	return func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].Unlock()
		}
	}
}

func sameLocalDay(unix int64, now time.Time) bool {
	if unix <= 0 {
		return false
	}
	then := time.Unix(unix, 0).In(time.Local)
	now = now.In(time.Local)
	return then.Year() == now.Year() && then.YearDay() == now.YearDay()
}

type awayResult uint8

const (
	awayNone awayResult = iota
	awayStillAway
	awayHome
	awayExpired
)

// recordPet records one petting action. roll is an inclusive-exclusive
// percentage roll in [0, 100); it is passed in so the state transition is
// deterministic in tests.
func recordPet(info *catInfo, now int64, roll int) bool {
	if info.PetWindowStart == 0 || now-info.PetWindowStart >= int64(petWindow/time.Second) || now < info.PetWindowStart {
		info.PetWindowStart = now
		info.PetCount = 1
		return false
	}

	info.PetCount++
	if info.PetCount != petTriggerCount {
		return false
	}

	// A failed check starts a fresh window. This keeps the trigger at exactly
	// the fifth pet rather than making every later pet another lottery ticket.
	info.PetCount = 0
	info.PetWindowStart = 0
	if roll >= 0 && roll < petAwayProbability {
		enterAway(info, now)
		return true
	}
	return false
}

func enterAway(info *catInfo, now int64) {
	info.AwayUntil = now + int64(awayDuration/time.Second)
	info.LastAwayCheck = 0
	info.PetCount = 0
	info.PetWindowStart = 0
	info.Penalty = 0
}

// updateAwayState performs at most one return check during the 24-hour away
// period. The caller should persist info when the result is not awayNone.
func updateAwayState(info *catInfo, now int64, roll int) awayResult {
	if info.AwayUntil == 0 {
		return awayNone
	}
	if now >= info.AwayUntil {
		expireAway(info, now)
		return awayExpired
	}
	before := info.LastAwayCheck
	if tryRecoverAway(info, now, roll) {
		return awayHome
	}
	if info.LastAwayCheck != before {
		return awayStillAway
	}
	return awayStillAway
}

func refreshAwayState(gid string, info *catInfo) (awayResult, error) {
	if info.AwayUntil == 0 {
		return awayNone, nil
	}
	before := *info
	result := updateAwayState(info, time.Now().Unix(), rand.Intn(100))
	if *info == before {
		return result, nil
	}
	if err := catdata.insert(gid, info); err != nil {
		return result, err
	}
	return result, nil
}

func handleAwayState(ctx *zero.Ctx, id any, gid string, info *catInfo) bool {
	result, err := refreshAwayState(gid, info)
	if err != nil {
		ctx.SendChain(message.Text("[ERROR]:", err))
		return true
	}
	if result == awayNone {
		return false
	}
	switch result {
	case awayHome:
		ctx.SendChain(message.Reply(id), message.Text("猫猫回家了,但还在生气,暂时不想进行这个操作"))
	case awayExpired:
		ctx.SendChain(message.Reply(id), message.Text("猫猫一天都没有回来,商店将你列入黑名单;下次买猫前需缴纳", awayPenalty, "椛椛币罚金"))
	default:
		ctx.SendChain(message.Reply(id), message.Text("猫猫还在离家中,暂时不能进行这个操作"))
	}
	return true
}

// tryRecoverAway performs at most one return check during the 24-hour away
// period. It returns true only when the cat comes home.
func tryRecoverAway(info *catInfo, now int64, roll int) bool {
	if info.AwayUntil == 0 {
		return false
	}
	if now >= info.AwayUntil {
		expireAway(info, now)
		return false
	}
	if info.LastAwayCheck != 0 && now-info.LastAwayCheck < int64(awayDuration/time.Second) {
		return false
	}
	info.LastAwayCheck = now
	if roll < 0 || roll >= awayReturnChance {
		return false
	}

	info.AwayUntil = 0
	info.LastAwayCheck = 0
	info.Penalty = 0
	info.PetCount = 0
	info.PetWindowStart = 0
	info.Satiety = 0
	info.Mood = 0
	info.Weight = 1
	info.LastTime = now
	info.Work = 0
	info.ArenaTime = 0
	return true
}

// expireAway converts an unrecovered away cat into a pending fine. The
// owner's food is retained, matching the existing delcat behavior.
func expireAway(info *catInfo, now int64) bool {
	if info.AwayUntil == 0 || now < info.AwayUntil {
		return false
	}
	info.AwayUntil = 0
	info.LastAwayCheck = 0
	info.Penalty = awayPenalty
	info.Name = ""
	info.Type = ""
	info.Satiety = 0
	info.Mood = 0
	info.Weight = 0
	info.LastTime = 0
	info.Work = 0
	info.ArenaTime = 0
	info.Picurl = ""
	info.PetCount = 0
	info.PetWindowStart = 0
	return true
}
