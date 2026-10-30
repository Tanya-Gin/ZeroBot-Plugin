package randomcat

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
)

const (
	randomDogAPIURL   = "https://random.dog/woof.json?filter=mp4,webm"
	theDogAPIURL      = "https://api.thedogapi.com/v1/images/search"
	randomDogAttempts = 3
)

var (
	randomDogEndpoint = randomDogAPIURL
	theDogAPIEndpoint = theDogAPIURL
)

func fetchJSON(endpoint string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ZeroBot-randomcat/1.0")
	resp, err := randomCatHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("API returned HTTP %s", resp.Status)
	}
	if resp.ContentLength > maxResponseBytes {
		return nil, errors.New("API response is too large")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBytes {
		return nil, errors.New("API response is too large")
	}
	return data, nil
}

var pickDog = func() bool {
	return rand.Intn(2) == 0
}

func fetchRandomPet() (string, error) {
	if pickDog() {
		return fetchRandomDog()
	}
	return fetchRandomCat()
}

func fetchRandomDog() (string, error) {
	var last error
	for i := 0; i < randomDogAttempts; i++ {
		data, err := fetchJSON(randomDogEndpoint)
		if err != nil {
			last = err
			continue
		}
		imageURL, err := parseRandomDogResponse(data)
		if err != nil {
			last = err
			continue
		}
		return imageURL, nil
	}
	data, err := fetchJSON(theDogAPIEndpoint)
	if err != nil {
		if last != nil {
			return "", last
		}
		return "", err
	}
	imageURL, err := parseRandomCatResponse(data)
	if err != nil {
		if last != nil {
			return "", last
		}
		return "", err
	}
	return validateImageURL(imageURL)
}
