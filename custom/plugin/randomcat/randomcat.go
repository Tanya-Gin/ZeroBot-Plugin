// Package randomcat 提供随机猫猫/狗狗图片。
package randomcat

import (
	"net/http"
	"time"

	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

const (
	randomCatAPIURL  = "https://api.thecatapi.com/v1/images/search"
	maxResponseBytes = 1 << 20
	requestTimeout   = 20 * time.Second
)

var (
	randomCatEndpoint   = randomCatAPIURL
	randomCatHTTPClient = &http.Client{Timeout: requestTimeout}
)

type randomCatImage struct {
	URL string `json:"url"`
}

func pluginHelp() string {
	return "发送“随机猫猫”获取一张随机猫猫图片\n发送“随机狗狗”获取一张随机狗狗图片或动图\n发送“随机猫狗”随机获取猫或狗"
}

func parseRandomCatResponse(data []byte) (string, error) {
	return parseImageListResponse(data, "cat")
}

func fetchRandomCat() (string, error) {
	data, err := fetchJSON(randomCatEndpoint)
	if err != nil {
		return "", err
	}
	return parseRandomCatResponse(data)
}

var engine = control.AutoRegister(&ctrl.Options[*zero.Ctx]{
	DisableOnDefault:  false,
	Brief:             "随机猫狗图片",
	Help:              pluginHelp(),
	PrivateDataFolder: "randomcat",
}).ApplySingle(ctxext.DefaultSingle)

func init() {
	engine.OnFullMatchGroup([]string{"随机猫猫", "随机狗狗", "随机猫狗"}, zero.OnlyGroup).
		SetBlock(true).
		Limit(ctxext.LimitByUser).
		Handle(func(ctx *zero.Ctx) {
			cmd, _ := ctx.State["matched"].(string)
			imageURL, err := fetchByCommand(cmd)
			if err != nil {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(failText(cmd)))
				return
			}
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Image(imageURL))
		})
}

func fetchByCommand(cmd string) (string, error) {
	switch cmd {
	case "随机狗狗":
		return fetchRandomDog()
	case "随机猫狗":
		return fetchRandomPet()
	default:
		return fetchRandomCat()
	}
}

func failText(cmd string) string {
	switch cmd {
	case "随机狗狗":
		return "随机狗狗获取失败，请稍后再试喵~"
	case "随机猫狗":
		return "随机猫狗获取失败，请稍后再试喵~"
	default:
		return "随机猫猫获取失败，请稍后再试喵~"
	}
}
