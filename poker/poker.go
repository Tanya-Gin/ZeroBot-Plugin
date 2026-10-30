// Package poker 抽扑克牌。
package poker

import (
	"errors"
	"fmt"
	"math/rand"
	"net/url"

	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

const pokerCardBaseURL = "https://gitea.seku.su/fumiama/zbpdata/raw/branch/main/Poker/"

var pokerCardNames = []string{
	"10club.jpg", "10diamond.jpg", "10heart.jpg", "10spade.jpg",
	"2club.jpg", "2diamond.jpg", "2heart.jpg", "2spade.jpg",
	"3club.jpg", "3diamond.jpg", "3heart.jpg", "3spade.jpg",
	"4club.jpg", "4diamond.jpg", "4heart.jpg", "4spade.jpg",
	"5club.jpg", "5diamond.jpg", "5heart.jpg", "5spade.jpg",
	"6club.jpg", "6diamond.jpg", "6heart.jpg", "6spade.jpg",
	"7club.jpg", "7diamond.jpg", "7heart.jpg", "7spade.jpg",
	"8club.jpg", "8diamond.jpg", "8heart.jpg", "8spade.jpg",
	"9club.jpg", "9diamond.jpg", "9heart.jpg", "9spade.jpg",
	"Aclub.jpg", "Adiamond.jpg", "Aheart.jpg", "Aspade.jpg",
	"Jclub.jpg", "Jdiamond.jpg", "Jheart.jpg", "Jspade.jpg",
	"Kclub.jpg", "Kdiamond.jpg", "Kheart.jpg", "Kspade.jpg",
	"Lynette.jpg", "Lyney.jpg",
	"Qclub.jpg", "Qdiamond.jpg", "Qheart.jpg", "Qspade.jpg",
}

const pokerCardCount = 54

func randomCardURL(index int) (string, error) {
	if index < 0 || index >= len(pokerCardNames) {
		return "", fmt.Errorf("poker card index %d is out of range", index)
	}
	cardURL := pokerCardBaseURL + pokerCardNames[index]
	parsed, err := url.Parse(cardURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "gitea.seku.su" {
		return "", errors.New("poker card URL is invalid")
	}
	return parsed.String(), nil
}

var engine = control.AutoRegister(&ctrl.Options[*zero.Ctx]{
	DisableOnDefault:  false,
	Brief:             "抽扑克牌",
	Help:              "- 抽扑克\n- poker",
	PrivateDataFolder: "poker",
}).ApplySingle(ctxext.DefaultSingle)

func init() {
	engine.OnFullMatchGroup([]string{"抽扑克", "抽扑克牌", "poker"}, zero.OnlyGroup).
		SetBlock(true).
		Limit(ctxext.LimitByUser).
		Handle(func(ctx *zero.Ctx) {
			cardURL, err := randomCardURL(rand.Intn(len(pokerCardNames)))
			if err != nil {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("[poker]获取扑克图片失败"))
				return
			}
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Image(cardURL))
		})
}
