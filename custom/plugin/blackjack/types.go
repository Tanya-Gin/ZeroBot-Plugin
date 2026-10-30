package blackjack

import (
	"errors"
	"sync"
	"time"

	zero "github.com/wdvxdr1123/ZeroBot"
)

const (
	// ChipRate is the wallet-currency value of one game chip.
	ChipRate         = 10
	BaseChip         = 100
	MinBuyIn         = BaseChip * ChipRate
	CashOutPercent   = 80
	BankerReserve    = 10000
	JoinTimeout      = 30 * time.Second
	ActionTimeout    = 60 * time.Second
	MaxGameDuration  = 5 * time.Minute
	CreateCooldown   = 10 * time.Second
	DealerStandValue = 17
	MaxPlayers       = 5
)

var (
	ErrInvalidSplit      = errors.New("当前手牌不满足分牌条件")
	ErrInvalidDouble     = errors.New("当前手牌不满足加倍条件")
	ErrInvalidSurrender  = errors.New("当前手牌不满足投降条件")
	ErrInsufficientFunds = errors.New("钱包余额不足")
)

type Card struct{ Name, Suit, Rank string }
type Hand struct{ Cards []Card }

type HandState string

const (
	HandActive      HandState = "active"
	HandStand       HandState = "stand"
	HandBust        HandState = "bust"
	HandBlackjack   HandState = "blackjack"
	HandDoubled     HandState = "doubled"
	HandSurrendered HandState = "surrendered"
	HandForfeited   HandState = "forfeited"
)

type GameMode string

const (
	ModeSingle GameMode = "single"
	ModeMulti  GameMode = "multi"
)

type GameStatus string

const (
	StatusWaiting   GameStatus = "waiting"
	StatusPlaying   GameStatus = "playing"
	StatusResolving GameStatus = "resolving"
	StatusFinished  GameStatus = "finished"
)

type Player struct {
	UID        int64
	Hands      []Hand
	Bets       []int
	HandStates []HandState
	PaidCoins  int
	JoinTime   time.Time
	SplitUsed  bool
}

type Game struct {
	mu            sync.Mutex
	GroupID       int64
	BankerID      int64
	Mode          GameMode
	Status        GameStatus
	Dealer        Hand
	Players       []*Player
	Deck          []Card
	StartedAt     time.Time
	LastAction    time.Time
	CurrentPlayer int
	CurrentHand   int
	TestMode      bool
	ctx           *zero.Ctx
	turnVersion   uint64
}

type GameManager struct {
	mu             sync.Mutex
	games          map[int64]*Game
	lastCreateTime map[int64]time.Time
}

func (h Hand) Value() int {
	value, aces := 0, 0
	for _, c := range h.Cards {
		switch c.Rank {
		case "A":
			value += 11
			aces++
		case "10", "J", "Q", "K":
			value += 10
		default:
			if len(c.Rank) == 1 && c.Rank[0] >= '2' && c.Rank[0] <= '9' {
				value += int(c.Rank[0] - '0')
			}
		}
	}
	for aces > 0 && value > 21 {
		value -= 10
		aces--
	}
	return value
}
func (h Hand) IsBlackjack() bool { return len(h.Cards) == 2 && h.Value() == 21 }

func cardPoint(rank string) int {
	if rank == "A" {
		return 11
	}
	if rank == "10" || rank == "J" || rank == "Q" || rank == "K" {
		return 10
	}
	if len(rank) == 1 && rank[0] >= '2' && rank[0] <= '9' {
		return int(rank[0] - '0')
	}
	return 0
}

func (p *Player) canSplit() bool {
	return !p.SplitUsed && len(p.Hands) == 1 && len(p.Bets) == 1 && len(p.HandStates) == 1 && len(p.Hands[0].Cards) == 2 && p.HandStates[0] == HandActive && cardPoint(p.Hands[0].Cards[0].Rank) == cardPoint(p.Hands[0].Cards[1].Rank)
}
func (p *Player) splitHand() error {
	if !p.canSplit() {
		return ErrInvalidSplit
	}
	cards := p.Hands[0].Cards
	p.Hands[0].Cards = []Card{cards[0]}
	p.Hands = append(p.Hands, Hand{Cards: []Card{cards[1]}})
	p.Bets = append(p.Bets, p.Bets[0])
	p.HandStates = append(p.HandStates, HandActive)
	p.SplitUsed = true
	return nil
}
func (p *Player) canDouble() bool {
	return len(p.Hands) == 1 && len(p.Bets) == 1 && len(p.HandStates) == 1 && len(p.Hands[0].Cards) == 2 && p.HandStates[0] == HandActive
}
func (p *Player) doubleDown() error {
	if !p.canDouble() {
		return ErrInvalidDouble
	}
	p.Bets[0] *= 2
	p.HandStates[0] = HandDoubled
	return nil
}
func (p *Player) canSurrender() bool {
	return len(p.Hands) == 1 && len(p.Bets) == 1 && len(p.HandStates) == 1 && len(p.Hands[0].Cards) == 2 && p.HandStates[0] == HandActive
}
func (p *Player) surrender() (int, error) {
	if !p.canSurrender() {
		return 0, ErrInvalidSurrender
	}
	p.HandStates[0] = HandSurrendered
	return p.Bets[0] / 2, nil
}

func (g *Game) activeHand() (*Player, *Hand, bool) {
	if g.CurrentPlayer < 0 || g.CurrentPlayer >= len(g.Players) {
		return nil, nil, false
	}
	p := g.Players[g.CurrentPlayer]
	if g.CurrentHand < 0 || g.CurrentHand >= len(p.Hands) {
		return p, nil, false
	}
	return p, &p.Hands[g.CurrentHand], true
}
func (g *Game) turnIsCurrent(token uint64) bool { return g.turnVersion == token }
