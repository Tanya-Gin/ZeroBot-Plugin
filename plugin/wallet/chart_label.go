package wallet

import (
	"os"

	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/img/text"
	"github.com/golang/freetype/truetype"
)

const (
	walletRankLabelMaxRunes = 12
	walletRankCacheVersion  = "-v2"
)

func walletRankCacheName(groupID, day string) string {
	return groupID + day + "walletRank" + walletRankCacheVersion + ".png"
}

func loadChartFont() (*truetype.Font, error) {
	if _, err := file.GetLazyData(text.FontFile, control.Md5File, true); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(text.FontFile)
	if err != nil {
		return nil, err
	}
	return truetype.Parse(data)
}

// sanitizeChartLabel keeps labels renderable by go-chart's single-font renderer.
// Unsupported runes become '?' so a nickname never turns into a silent blank.
// Long labels are truncated by runes, preserving UTF-8 boundaries.
func sanitizeChartLabel(font *truetype.Font, label string, maxRunes int) string {
	if maxRunes > 0 && len([]rune(label)) > maxRunes {
		label = string([]rune(label)[:maxRunes-1]) + "…"
	}
	output := make([]rune, 0, len(label))
	for _, r := range label {
		if font.Index(r) == 0 && r != 0 {
			output = append(output, '?')
			continue
		}
		output = append(output, r)
	}
	return string(output)
}
