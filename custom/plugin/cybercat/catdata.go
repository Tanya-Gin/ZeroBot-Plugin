// Package cybercat 云养猫
package cybercat

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	fcext "github.com/FloatTech/floatbox/ctxext"
	"github.com/FloatTech/floatbox/file"
	sql "github.com/FloatTech/sqlite"

	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/tidwall/gjson"

	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

const (
	apiURL              = "https://api.thecatapi.com/v1/images/"
	defaultCatImage     = "base64://iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	maxCatResponseBytes = 10 << 20
)

var typeEN2ZH = map[string]string{
	"Abyssinian": "阿比西尼亚猫", "Aegean": "爱琴猫", "American Bobtail": "美国短尾猫", "American Curl": "美国卷耳猫", "American Shorthairs": "美洲短毛猫", "American Wirehair": "美国硬毛猫",
	"Arabian Mau": "美英猫", "Australian Mist": "澳大利亚雾猫", "Balinese": "巴厘岛猫", "Bambino": "班比诺猫", "Bengal": "孟加拉虎", "Birman": "比尔曼猫", "Bombay": "孟买猫", "British Longhair": "英国长毛猫",
	"British Shorthair": "英国短毛猫", "Burmese": "缅甸猫", "Burmilla": "博美拉猫", "California Spangled": "加州闪亮猫", "Chantilly-Tiffany": "查达利/蒂法尼猫", "Chartreux": "夏特鲁斯猫", "Chausie": "非洲狮子猫",
	"Cheetoh": "奇多猫", "Colorpoint Shorthair": "重点色短毛猫", "Cornish Rex": "康沃尔-雷克斯猫", "Cymric": "威尔士猫", "Cyprus": "塞浦路斯猫", "Devon Rex": "德文狸猫", "Donskoy": "顿斯科伊猫", "Dragon Li": "中国狸花猫",
	"Egyptian Mau": "埃及猫", "European Burmese": "欧洲缅甸猫", "Exotic Shorthair": "异国短毛猫", "Havana Brown": "哈瓦那褐猫", "Himalayan": "喜马拉雅猫", "Japanese Bobtail": "日本短尾猫", "Javanese": "爪哇猫",
	"Khao Manee": "泰国御猫", "Korat": "呵叻猫", "Kurilian": "千岛短尾猫", "LaPerm": "拉邦猫", "Maine Coon": "缅因猫", "Malayan": "马来猫", "Manx": "马恩岛猫", "Munchkin": "曼基康猫", "Nebelung": "内华达猫",
	"Norwegian Forest Cat": "挪威森林猫", "Ocicat": "欧西猫", "Oriental Shorthair": "东方短毛猫", "Persian": "波斯猫", "Pixie-bob": "北美洲短毛猫", "Ragamuffin": "褴褛猫", "Ragdoll": "布偶猫",
	"Russian Blue": "俄罗斯蓝猫", "Savannah": "沙凡那猫", "Scottish Fold": "苏格兰折耳猫", "Selkirk Rex": "塞尔凯克卷毛猫", "Siamese": "暹罗猫", "Siberian": "西伯利亚猫", "Singapura": "新加坡猫", "Snowshoe": "雪鞋猫",
	"Somali": "索马里猫", "Sphynx": "斯芬克斯猫", "Tonkinese": "东京猫", "Toyger": "玩具虎猫", "Turkish Angora": "土耳其安哥拉猫",
	"Turkish Van": "土耳其梵猫", "York Chocolate": "约克巧克力猫", "Cymic": "金力克长毛猫"}

var typeZH2Breeds = map[string]string{
	"阿比西尼亚猫": "abys", "爱琴猫": "aege", "美国短尾猫": "abob", "美国卷耳猫": "acur", "美洲短毛猫": "asho", "美国硬毛猫": "awir", "美英猫": "amau", "澳大利亚雾猫": "amis", "巴厘岛猫": "bali",
	"班比诺猫": "bamb", "孟加拉虎": "beng", "比尔曼猫": "birm", "孟买猫": "bomb", "英国长毛猫": "bslo", "英国短毛猫": "bsho", "缅甸猫": "bure", "博美拉猫": "buri", "加州闪亮猫": "cspa",
	"查达利/蒂法尼猫": "ctif", "夏特鲁斯猫": "char", "非洲狮子猫": "chau", "奇多猫": "chee", "重点色短毛猫": "csho", "康沃尔-雷克斯猫": "crex", "威尔士猫": "cymr", "塞浦路斯猫": "cypr",
	"德文狸猫": "drex", "顿斯科伊猫": "dons", "中国狸花猫": "lihu", "埃及猫": "emau", "欧洲缅甸猫": "ebur", "异国短毛猫": "esho", "哈瓦那褐猫": "hbro", "喜马拉雅猫": "hima", "日本短尾猫": "jbob",
	"爪哇猫": "java", "泰国御猫": "khao", "呵叻猫": "kora", "千岛短尾猫": "kuri", "拉邦猫": "lape", "缅因猫": "mcoo", "马来猫": "mala", "马恩岛猫": "manx", "曼基康猫": "munc", "内华达猫": "nebe",
	"挪威森林猫": "norw", "欧西猫": "ocic", "东方短毛猫": "orie", "波斯猫": "pers", "北美洲短毛猫": "pixi", "褴褛猫": "raga", "布偶猫": "ragd", "俄罗斯蓝猫": "rblu", "沙凡那猫": "sava",
	"苏格兰折耳猫": "sfol", "塞尔凯克卷毛猫": "srex", "暹罗猫": "siam", "西伯利亚猫": "sibe", "新加坡猫": "sing", "雪鞋猫": "snow", "索马里猫": "soma", "斯芬克斯猫": "sphy", "东京猫": "tonk",
	"玩具虎猫": "toyg", "土耳其安哥拉猫": "tang", "土耳其梵猫": "tvvan", "约克巧克力猫": "ycho", "金力克长毛猫": "cymi"}

type catdb struct {
	sync.RWMutex
	sql.Sqlite
}

type breedInfo struct {
	TypeName    string
	Temperament string
	Description string
	ImageURL    string
	Err         error
}

type catInfo struct {
	User           int64   // 主人
	Name           string  // 喵喵名称
	Type           string  // 品种
	Satiety        float64 // 饱食度
	Mood           int     // 心情
	Weight         float64 // 体重
	LastTime       int64   // 上次喂养时间
	Work           int64   // 打工时间
	ArenaTime      int64   // 上次PK时间
	Food           float64 // 食物数量
	Picurl         string  // 猫猫图片
	PetCount       int64   // 短时间内撸猫次数
	PetWindowStart int64   // 撸猫统计窗口开始时间
	AwayUntil      int64   // 离家状态截止时间
	LastAwayCheck  int64   // 最近一次回家判定时间
	Penalty        int     // 待缴纳的商店罚金
	ShopTime       int64   // 上次成功购买猫的时间
}

func (inf *catInfo) avatar(gid int64) string {
	// 构建缓存文件路径
	cache := path.Join(engine.DataFolder(), "cache")
	hash := sha256.Sum256([]byte(inf.Picurl))
	imgname := fmt.Sprintf("%d_%d_%x", inf.User, gid, hash[:6])
	imgfile := filepath.Join(cache, inf.Type+imgname+".png")
	aimgfile := filepath.Join(file.BOTPATH, imgfile)

	// 确保缓存目录存在
	_ = os.MkdirAll(cache, 0755)

	// 1. 如果本地已有缓存文件，直接返回
	if file.IsExist(aimgfile) {
		_ = removeCachedAvatars(cache, inf.User, gid, filepath.Base(aimgfile))
		return "file:///" + aimgfile
	}

	// 2. 优先使用已有的 Picurl（例如用户上传的猫娘图片）
	if inf.Picurl != "" {
		if err := downloadAndSave(inf.Picurl, aimgfile); err == nil {
			_ = removeCachedAvatars(cache, inf.User, gid, filepath.Base(aimgfile))
			return "file:///" + aimgfile
		}
		// 下载失败，继续尝试通过品种获取
	}

	// 3. 通过品种ID获取图片
	breedID, ok := typeZH2Breeds[inf.Type]
	if ok {
		// 构建请求URL（使用 breed_ids 参数）
		url := apiURL + "search?breed_ids=" + breedID + "&limit=1"

		data, err := getCatData(url)
		if err == nil {
			imgURL := gjson.ParseBytes(data).Get("0.url").String()
			if imgURL != "" {
				if err := downloadAndSave(imgURL, aimgfile); err == nil {
					_ = removeCachedAvatars(cache, inf.User, gid, filepath.Base(aimgfile))
					// 保存 URL 到数据库供下次使用
					inf.Picurl = imgURL
					go func() {
						gidStr := "group" + strconv.FormatInt(gid, 10)
						_ = catdata.insert(gidStr, inf) // 异步更新，忽略错误
					}()
					return "file:///" + aimgfile
				}
			}
		}
	}

	// 4. 后备方案：随机猫图
	result := suineko()
	if result.Err == nil && result.ImageURL != "" {
		if err := downloadAndSave(result.ImageURL, aimgfile); err == nil {
			_ = removeCachedAvatars(cache, inf.User, gid, filepath.Base(aimgfile))
			return "file:///" + aimgfile
		}
	}

	// 5. 最终后备：透明PNG（避免报错）
	return defaultCatImage
}

func removeCachedAvatars(cacheDir string, user, gid int64, keepName string) error {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	marker := fmt.Sprintf("%d_%d_", user, gid)
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == keepName || !strings.HasSuffix(entry.Name(), ".png") {
			continue
		}
		if !strings.Contains(entry.Name(), marker) {
			continue
		}
		if err := os.Remove(filepath.Join(cacheDir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// downloadAndSave 下载图片并保存到本地
func downloadAndSave(url, filePath string) error {
	data, err := getCatData(url)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(filePath), ".cybercat-image-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(0644); err != nil {
		tmp.Close()
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filePath)
}

var catHTTPClient = &http.Client{Timeout: 20 * time.Second}

func getCatData(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if apiKey := getCatAPIKey(); apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}
	resp, err := catHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("cat API returned HTTP %s", resp.Status)
	}
	if resp.ContentLength > maxCatResponseBytes {
		return nil, fmt.Errorf("cat API response is too large")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCatResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxCatResponseBytes {
		return nil, fmt.Errorf("cat API response is too large")
	}
	return data, nil
}

// getCatAPIKey API密钥
func getCatAPIKey() string {
	// Credentials must come from the process environment, never from source.
	return strings.TrimSpace(os.Getenv("THECATAPI_KEY"))
}

var (
	engine = control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "云养猫",
		Help: "一款既能能赚钱(?)又能看猫的养成类插件\n----------指令----------\n" +
			"- 吸猫\n(随机返回一只猫)\n- 吸xxx猫\n(吸指定猫种的猫)\n- 买猫\n- 买xxx猫\n- 买n袋猫粮\n- 喂猫n斤猫粮\n" +
			"- 猫猫打工[1-9]小时\n- 撸猫\n- 猫猫状态\n- 猫猫改名叫xxx\n" +
			"- 喵喵pk@对方QQ\n- 猫猫排行榜\n" +
			"\n---------注意事项---------" +
			"\n1.科学养猪(划去)\n" +
			"猫店开门时间为6点-21点\n喂猫时间为6点-9点、11点-14点、17点-20点;\n工作时间为6点-21点" +
			"\n2.一袋有五斤猫粮" +
			"\n3.猫猫心情影响吃饭饭和打工" +
			"\n4.越重的猫猫饭量越大呢!" +
			"\n5.一天只能打工一次,打工期间的猫猫无法喂养哦" +
			"\n6.品种为猫娘的猫猫可以使用“上传猫猫照片”更换图片",
		PrivateDataFolder: "cybercat",
	}).ApplySingle(ctxext.DefaultSingle)
	catdata = &catdb{Sqlite: sql.New(engine.DataFolder() + "catdata.db")}
	getdb   = fcext.DoOnceOnSuccess(func(ctx *zero.Ctx) bool {
		err := catdata.Open(time.Hour * 24)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return false
		}
		return true
	})
)

func init() {
	engine.OnRegex(`^吸(.*猫)$`).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		typeOfcat := ctx.State["regex_matched"].([]string)[1]
		if typeOfcat == "猫" {
			result := suineko()
			if result.Err != nil {
				ctx.SendChain(message.Text("[ERROR]: ", result.Err))
				return
			}
			ctx.SendChain(
				message.Image(result.ImageURL),
				message.Text("品种: ", result.TypeName,
					"\n气质:\n", result.Temperament,
					"\n描述:\n", result.Description),
			)
			return
		}
		breeds, ok := typeZH2Breeds[typeOfcat]
		if !ok {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("没有相关该品种的猫图"))
			return
		}
		picurl, err := getPicByBreed(breeds)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]: ", err))
			return
		}
		ctx.SendChain(message.Text("品种: ", typeOfcat), message.Image(picurl))
	})
}

func parseBreedInfo(data []byte) (breedInfo, error) {
	root := gjson.ParseBytes(data)
	if !root.IsObject() {
		return breedInfo{}, errors.New("cat API returned an invalid image object")
	}
	imageURL := root.Get("url").String()
	breed := root.Get("breeds.0")
	name := breed.Get("name").String()
	if imageURL == "" || name == "" {
		return breedInfo{}, errors.New("cat API image has no breed details")
	}
	return breedInfo{
		TypeName:    typeEN2ZH[name],
		Temperament: breed.Get("temperament").String(),
		Description: breed.Get("description").String(),
		ImageURL:    imageURL,
	}, nil
}

func parseSearchBreedInfo(data []byte) (breedInfo, error) {
	root := gjson.ParseBytes(data)
	if !root.IsArray() {
		return breedInfo{}, errors.New("cat API returned an invalid image list")
	}
	var lastErr error
	for _, item := range root.Array() {
		result, err := parseBreedInfo([]byte(item.Raw))
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("cat API returned no images")
	}
	return breedInfo{}, lastErr
}

type breedCandidate struct {
	ID   string
	Info breedInfo
}

func parseBreedCatalog(data []byte) ([]breedCandidate, error) {
	root := gjson.ParseBytes(data)
	if !root.IsArray() {
		return nil, errors.New("cat API returned an invalid breed list")
	}
	candidates := make([]breedCandidate, 0, len(root.Array()))
	for _, item := range root.Array() {
		id := item.Get("id").String()
		name := item.Get("name").String()
		if id == "" || name == "" {
			continue
		}
		candidates = append(candidates, breedCandidate{
			ID: id,
			Info: breedInfo{
				TypeName:    typeEN2ZH[name],
				Temperament: item.Get("temperament").String(),
				Description: item.Get("description").String(),
			},
		})
	}
	if len(candidates) == 0 {
		return nil, errors.New("cat API returned no usable breeds")
	}
	return candidates, nil
}

func breedCatalogFallback() (breedInfo, error) {
	data, err := getCatData("https://api.thecatapi.com/v1/breeds")
	if err != nil {
		return breedInfo{}, err
	}
	candidates, err := parseBreedCatalog(data)
	if err != nil {
		return breedInfo{}, err
	}
	start := rand.Intn(len(candidates))
	for i := 0; i < len(candidates); i++ {
		candidate := candidates[(start+i)%len(candidates)]
		imageData, imageErr := getCatData(apiURL + "search?breed_ids=" + candidate.ID + "&limit=1")
		if imageErr != nil {
			continue
		}
		imageURL := gjson.ParseBytes(imageData).Get("0.url").String()
		if imageURL == "" {
			continue
		}
		candidate.Info.ImageURL = imageURL
		if candidate.Info.TypeName == "" {
			candidate.Info.TypeName = "未知品种"
		}
		if candidate.Info.Temperament == "" {
			candidate.Info.Temperament = "暂无资料"
		}
		if candidate.Info.Description == "" {
			candidate.Info.Description = "暂无资料"
		}
		return candidate.Info, nil
	}
	return breedInfo{}, errors.New("cat API returned no image for known breeds")
}

func suineko() breedInfo {
	const searchURL = apiURL + "search?has_breeds=1&include_breeds=true&limit=10"
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		data, err := getCatData(searchURL)
		if err != nil {
			return breedInfo{Err: err}
		}
		result, err := parseSearchBreedInfo(data)
		if err == nil {
			if result.TypeName == "" {
				result.TypeName = "未知品种"
			}
			if result.Temperament == "" {
				result.Temperament = "暂无资料"
			}
			if result.Description == "" {
				result.Description = "暂无资料"
			}
			return result
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("cat API returned no usable breed details")
	}
	if fallback, err := breedCatalogFallback(); err == nil {
		return fallback
	}
	return breedInfo{Err: lastErr}
}

func getPicByBreed(catBreed string) (url string, err error) {
	data, err := getCatData(apiURL + "search?breed_ids=" + catBreed)
	if err != nil {
		return
	}
	url = gjson.ParseBytes(data).Get("0.url").String()
	if url == "" {
		err = errors.New("no image found")
	}
	return
}

func (sql *catdb) insert(gid string, dbInfo *catInfo) error {
	sql.Lock()
	defer sql.Unlock()
	if err := sql.ensureSchemaLocked(gid); err != nil {
		return err
	}
	return sql.Insert(gid, dbInfo)
}

func (db *catdb) find(gid, uid string) (dbInfo catInfo, err error) {
	db.Lock()
	defer db.Unlock()
	if err = db.ensureSchemaLocked(gid); err != nil {
		return
	}
	err = db.Find(gid, &dbInfo, "where User = ?", uid)
	if errors.Is(err, sql.ErrNullResult) {
		return catInfo{}, nil
	}
	return
}

func (db *catdb) del(gid, uid string) error {
	db.Lock()
	defer db.Unlock()
	if err := db.ensureSchemaLocked(gid); err != nil {
		return err
	}
	return db.Del(gid, "where User = ?", uid)
}

func (db *catdb) delcat(gid, uid string) error {
	db.Lock()
	defer db.Unlock()
	if err := db.ensureSchemaLocked(gid); err != nil {
		return err
	}
	dbInfo := catInfo{}
	if err := db.Find(gid, &dbInfo, "where User = ?", uid); err != nil {
		if errors.Is(err, sql.ErrNullResult) {
			return nil
		}
		return err
	}
	newInfo := catInfo{
		User: dbInfo.User,
		Food: dbInfo.Food,
	}
	return db.Insert(gid, &newInfo)
}

func (db *catdb) getGroupdata(gid string) (list []catInfo, err error) {
	db.Lock()
	defer db.Unlock()
	if err = db.ensureSchemaLocked(gid); err != nil {
		return
	}
	info := catInfo{}
	err = db.FindFor(gid, &info, "order by Weight DESC", func() error {
		if info.Name != "" {
			list = append(list, info)
		}
		return nil
	})
	if errors.Is(err, sql.ErrNullResult) {
		err = nil
	}
	return
}

func (db *catdb) ensureSchemaLocked(gid string) error {
	if err := db.Create(gid, &catInfo{}); err != nil {
		return err
	}
	columns := []struct {
		name string
		typ  string
	}{
		{name: "PetCount", typ: "BIGINT"},
		{name: "PetWindowStart", typ: "BIGINT"},
		{name: "AwayUntil", typ: "BIGINT"},
		{name: "LastAwayCheck", typ: "BIGINT"},
		{name: "Penalty", typ: "INTEGER"},
		{name: "ShopTime", typ: "BIGINT"},
	}
	for _, column := range columns {
		_, err := db.Exec("ALTER TABLE '" + gid + "' ADD COLUMN '" + column.name + "' " + column.typ + " NOT NULL DEFAULT 0")
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return err
		}
	}
	return nil
}
