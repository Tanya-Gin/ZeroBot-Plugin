package whattoeat

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDisplayDishNameLocalizesKnownAndPreservesUnknown(t *testing.T) {
	if got := displayDishName("Kung Pao Chicken"); got != "宫保鸡丁" {
		t.Fatalf("localized dish name = %q", got)
	}
	if got := displayDishName("Unknown Dish"); got != "Unknown Dish" {
		t.Fatalf("unknown dish name = %q", got)
	}
}

func TestParseMealsResponseKeepsOnlyValidMealAndImage(t *testing.T) {
	data := []byte(`{"meals":[{"idMeal":"1","strMeal":"Kung Pao Chicken","strMealThumb":"https://www.themealdb.com/images/media/meals/example.jpg"},{"idMeal":"2","strMeal":"bad image","strMealThumb":"http://evil.example/a.jpg"},{"idMeal":"","strMeal":"missing id","strMealThumb":"https://www.themealdb.com/images/media/meals/a.jpg"}]}`)
	items, err := parseMealsResponse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "Kung Pao Chicken" {
		t.Fatalf("valid meals = %+v; expected only Kung Pao Chicken", items)
	}
}

func TestParseMealsResponseRejectsMalformedAndEmpty(t *testing.T) {
	for _, data := range [][]byte{[]byte("not-json"), []byte(`{"meals":[]}`), []byte(`{"meals":[{"idMeal":"1","strMeal":"unsafe","strMealThumb":"https://evil.example/x.jpg"}]}`)} {
		if _, err := parseMealsResponse(data); err == nil {
			t.Fatalf("parseMealsResponse(%q) unexpectedly succeeded", data)
		}
	}
}

func TestValidatePhotoURLRejectsNonHTTPSAndOtherHosts(t *testing.T) {
	for _, raw := range []string{"http://www.themealdb.com/images/media/meals/a.jpg", "https://evil.example/images/media/meals/a.jpg", "https://www.themealdb.com/other/a.jpg", "javascript:alert(1)"} {
		if _, err := validatePhotoURL(raw); err == nil {
			t.Errorf("validatePhotoURL(%q) unexpectedly succeeded", raw)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestFetchSuggestionUsesEndpointAndReturnsPairedImage(t *testing.T) {
	oldEndpoint, oldClient := mealsEndpoint, mealsHTTPClient
	t.Cleanup(func() { mealsEndpoint, mealsHTTPClient = oldEndpoint, oldClient })
	mealsEndpoint = "https://api.test.invalid/meals"
	mealsHTTPClient = &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.String() != mealsEndpoint {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL)
		}
		body := `{"meals":[{"idMeal":"123","strMeal":"Chicken Congee","strMealThumb":"https://www.themealdb.com/images/media/meals/congee.jpg"}]}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: ioNopCloser{Reader: strings.NewReader(body)}, ContentLength: int64(len(body)), Request: req}, nil
	})}
	got, err := fetchSuggestion()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "123" || got.Name != "Chicken Congee" || !strings.HasSuffix(got.Photo, "congee.jpg") {
		t.Fatalf("fetchSuggestion() = %+v", got)
	}
}

type ioNopCloser struct{ *strings.Reader }

func (r ioNopCloser) Close() error { return nil }
