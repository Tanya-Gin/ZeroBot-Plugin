package randomcat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
)

const maxDogFileBytes = 4 << 20

var allowedImageExt = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".gif":  {},
}

type randomDogResponse struct {
	URL           string `json:"url"`
	FileSizeBytes int64  `json:"fileSizeBytes"`
}

func parseImageListResponse(data []byte, kind string) (string, error) {
	var images []randomCatImage
	if err := json.Unmarshal(data, &images); err != nil {
		return "", fmt.Errorf("invalid %s API response: %w", kind, err)
	}
	if len(images) == 0 || strings.TrimSpace(images[0].URL) == "" {
		return "", fmt.Errorf("%s API returned no image", kind)
	}
	parsed, err := url.Parse(strings.TrimSpace(images[0].URL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", fmt.Errorf("%s API returned an unsafe image URL", kind)
	}
	return parsed.String(), nil
}

func parseRandomDogResponse(data []byte) (string, error) {
	var payload randomDogResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("invalid dog API response: %w", err)
	}
	if payload.FileSizeBytes > maxDogFileBytes {
		return "", errors.New("dog API returned an oversized image")
	}
	return validateImageURL(payload.URL)
}

func validateImageURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", errors.New("API returned an unsafe image URL")
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	if _, ok := allowedImageExt[ext]; !ok {
		return "", errors.New("API returned a non-image URL")
	}
	return parsed.String(), nil
}
