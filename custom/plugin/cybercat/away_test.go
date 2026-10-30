package cybercat

import (
	"testing"
	"time"
)

func TestRecordPetTriggersAwayOnFifthPetWithinWindow(t *testing.T) {
	info := catInfo{}
	const now = int64(1_000_000)

	for i := int64(0); i < petTriggerCount-1; i++ {
		if away := recordPet(&info, now+i, 99); away {
			t.Fatalf("pet %d unexpectedly triggered away", i+1)
		}
	}
	if info.PetCount != petTriggerCount-1 {
		t.Fatalf("pet count = %d, want %d", info.PetCount, petTriggerCount-1)
	}
	if !recordPet(&info, now+petTriggerCount-1, 0) {
		t.Fatal("fifth pet with a successful roll should trigger away")
	}
}

func TestRecordPetDoesNotTriggerAfterWindowExpires(t *testing.T) {
	info := catInfo{}
	const now = int64(1_000_000)

	for i := int64(0); i < petTriggerCount-1; i++ {
		recordPet(&info, now+i, 0)
	}
	if got := recordPet(&info, now+int64(petWindow/time.Second), 0); got {
		t.Fatal("pets outside the window must not trigger away")
	}
	if info.PetCount != 1 {
		t.Fatalf("pet count after reset = %d, want 1", info.PetCount)
	}
}

func TestEnterAwayInitializesPenaltyWindow(t *testing.T) {
	info := catInfo{User: 42, Name: "小猫", Type: "布偶猫", Food: 7}
	const now = int64(1_000_000)

	enterAway(&info, now)
	if info.AwayUntil != now+int64(awayDuration/time.Second) {
		t.Fatalf("AwayUntil = %d, want %d", info.AwayUntil, now+int64(awayDuration/time.Second))
	}
	if info.Penalty != 0 {
		t.Fatalf("Penalty = %d, want 0 while cat is away", info.Penalty)
	}
	if info.PetCount != 0 || info.PetWindowStart != 0 {
		t.Fatal("pet counter must reset when cat leaves")
	}
}

func TestRecoverAwayRestoresCatAtMinimumAttributes(t *testing.T) {
	info := catInfo{Name: "小猫", Type: "布偶猫", Satiety: 80, Mood: 70, Weight: 8, Food: 4}
	enterAway(&info, 1_000_000)

	if !tryRecoverAway(&info, 1_000_001, 34) {
		t.Fatal("roll below recovery probability should bring cat home")
	}
	if info.AwayUntil != 0 || info.Penalty != 0 {
		t.Fatal("recovered cat must no longer be away or fined")
	}
	if info.Satiety != 0 || info.Mood != 0 || info.Weight != 1 {
		t.Fatalf("recovered attributes = (%v, %d, %v), want (0, 0, 1)", info.Satiety, info.Mood, info.Weight)
	}
}

func TestRecoverAwayOnlyChecksOnceBeforeDeadline(t *testing.T) {
	info := catInfo{Name: "小猫", Type: "布偶猫"}
	const now = int64(1_000_000)
	enterAway(&info, now)

	if tryRecoverAway(&info, now+1, 99) {
		t.Fatal("failed recovery roll must not bring cat home")
	}
	if tryRecoverAway(&info, now+2, 0) {
		t.Fatal("recovery must not be rolled repeatedly in the same day")
	}
}

func TestExpireAwayCreatesPenaltyAndRemovesCatIdentity(t *testing.T) {
	info := catInfo{User: 42, Name: "小猫", Type: "布偶猫", Food: 6}
	const now = int64(1_000_000)
	enterAway(&info, now)

	if !expireAway(&info, now+int64(awayDuration/time.Second)) {
		t.Fatal("away state should expire at the deadline")
	}
	if info.Name != "" || info.Type != "" || info.Penalty != awayPenalty {
		t.Fatalf("expired state = name %q type %q penalty %d", info.Name, info.Type, info.Penalty)
	}
	if info.Food != 6 {
		t.Fatalf("food = %v, want 6 preserved", info.Food)
	}
}

func TestLockCatsHandlesSameCatWithoutDeadlocking(t *testing.T) {
	unlock := lockCats("group-test", "user-test", "user-test")
	unlock()
}

func TestLockCatsUsesStableOrderForReversedPairs(t *testing.T) {
	unlock := lockCats("group-test", "user-a", "user-b")
	done := make(chan struct{})
	go func() {
		otherUnlock := lockCats("group-test", "user-b", "user-a")
		otherUnlock()
		close(done)
	}()
	unlock()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reversed lock acquisition did not complete")
	}
}
