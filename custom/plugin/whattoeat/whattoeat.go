// Package whattoeat recommends a Chinese meal with a matching photo.
package whattoeat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

const (
	mealsAPIURL      = "https://www.themealdb.com/api/json/v1/1/filter.php?a=Chinese"
	maxResponseBytes = 1 << 20
	requestTimeout   = 12 * time.Second
)

var (
	mealsEndpoint   = mealsAPIURL
	mealsHTTPClient = &http.Client{Timeout: requestTimeout}
)

type meal struct {
	ID    string `json:"idMeal"`
	Name  string `json:"strMeal"`
	Photo string `json:"strMealThumb"`
}

type mealsResponse struct {
	Meals []meal `json:"meals"`
}

func helpText() string {
	return "━━━ 今天吃什么 ━━━\n\n" +
		"【使用方法】\n" +
		"- 今天吃什么 / 吃什么：随机推荐一道中式菜品，并附带菜品图片\n\n" +
		"【说明】\n" +
		"- 菜品来源：TheMealDB（https://www.themealdb.com/）\n" +
		"- 仅推荐中式菜品，其他菜系暂不支持\n\n" +
		"【示例】\n" +
		"- 今天吃什么\n" +
		"- 吃什么"
}

var chineseDishNames = map[string]string{
	"Air Fryer Egg Rolls":                         "空气炸锅春卷",
	"Beef and Broccoli Stir-Fry":                  "西兰花炒牛肉",
	"Beef Lo Mein":                                "牛肉炒面",
	"Chicken Congee":                              "鸡肉粥",
	"Chicken Fried Rice":                          "鸡肉炒饭",
	"Chinese Orange Chicken":                      "陈皮鸡",
	"Chinese Tomato Egg Stir Fry":                 "番茄炒蛋",
	"Egg Drop Soup":                               "蛋花汤",
	"Egg Foo Young":                               "芙蓉蛋",
	"General Tsos Chicken":                        "左宗棠鸡",
	"Hot and Sour Soup":                           "酸辣汤",
	"Kung Pao Chicken":                            "宫保鸡丁",
	"Kung Po Prawns":                              "宫保虾仁",
	"Ma Po Tofu":                                  "麻婆豆腐",
	"Napa Cabbage with Dried Shrimp":              "虾米白菜",
	"Ramen Noodles with Boiled Egg":               "鸡蛋拉面",
	"Sesame Cucumber Salad":                       "芝麻凉拌黄瓜",
	"Shrimp Chow Fun":                             "虾仁炒河粉",
	"Shrimp With Snow Peas":                       "虾仁炒荷兰豆",
	"Sichuan Eggplant":                            "鱼香茄子",
	"Sichuan Style Stir-Fried Chinese Long Beans": "干煸四季豆",
	"Silken Tofu with Sesame Soy Sauce":           "芝麻酱油嫩豆腐",
	"Singapore Noodles with Shrimp":               "虾仁星洲炒米粉",
	"Sweet and Sour Chicken":                      "糖醋鸡",
	"Sweet and Sour Pork":                         "糖醋里脊",
	"Szechuan Beef":                               "四川牛肉",
	"Wontons":                                     "馄饨",
}

func displayDishName(name string) string {
	if localized, ok := chineseDishNames[name]; ok {
		return localized
	}
	return name
}

var engine = control.AutoRegister(&ctrl.Options[*zero.Ctx]{
	DisableOnDefault:  false,
	Brief:             "今天吃什么（带菜品图片）",
	Help:              helpText(),
	PrivateDataFolder: "whattoeat",
}).ApplySingle(ctxext.DefaultSingle)

func init() {
	engine.OnFullMatchGroup([]string{"今天吃什么", "吃什么", "中午吃什么", "晚上吃什么", "午饭吃什么", "晚饭吃什么"}, zero.OnlyGroup).
		SetBlock(true).
		Limit(ctxext.LimitByUser).
		Handle(func(ctx *zero.Ctx) {
			dish, err := fetchSuggestion()
			if err != nil {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("今天吃什么暂时没取到推荐，稍后再试试喵。"))
				return
			}
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("今天推荐："+displayDishName(dish.Name)+"（"+dish.Name+"）\n菜系：中式\n图片来源：TheMealDB"), message.Image(dish.Photo))
		})
}

func parseMealsResponse(data []byte) ([]meal, error) {
	var payload mealsResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("解析菜品 API 响应失败: %w", err)
	}
	valid := make([]meal, 0, len(payload.Meals))
	for _, item := range payload.Meals {
		item.ID = strings.TrimSpace(item.ID)
		item.Name = strings.TrimSpace(item.Name)
		photo, err := validatePhotoURL(item.Photo)
		if item.ID == "" || item.Name == "" || err != nil {
			continue
		}
		item.Photo = photo
		valid = append(valid, item)
	}
	if len(valid) == 0 {
		return nil, errors.New("菜品 API 没有返回有效菜品和图片")
	}
	return valid, nil
}

func validatePhotoURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "www.themealdb.com" || !strings.HasPrefix(parsed.Path, "/images/media/meals/") {
		return "", errors.New("菜品 API 返回了无效图片地址")
	}
	return parsed.String(), nil
}

func fetchSuggestion() (meal, error) {
	request, err := http.NewRequest(http.MethodGet, mealsEndpoint, nil)
	if err != nil {
		return meal{}, fmt.Errorf("创建菜品请求失败: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "ZeroBot-WhatToEat/1.0")
	response, err := mealsHTTPClient.Do(request)
	if err != nil {
		return meal{}, fmt.Errorf("请求菜品 API 失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return meal{}, fmt.Errorf("菜品 API 返回 HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxResponseBytes {
		return meal{}, errors.New("菜品 API 响应超过大小限制")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return meal{}, fmt.Errorf("读取菜品 API 响应失败: %w", err)
	}
	if len(body) > maxResponseBytes {
		return meal{}, errors.New("菜品 API 响应超过大小限制")
	}
	items, err := parseMealsResponse(body)
	if err != nil {
		return meal{}, err
	}
	return items[rand.Intn(len(items))], nil
}
