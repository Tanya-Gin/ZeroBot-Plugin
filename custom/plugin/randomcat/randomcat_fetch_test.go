package randomcat

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFetchRandomCatUsesAPIResponse(t *testing.T) {
	oldEndpoint := randomCatEndpoint
	oldClient := randomCatHTTPClient
	t.Cleanup(func() {
		randomCatEndpoint = oldEndpoint
		randomCatHTTPClient = oldClient
	})

	randomCatEndpoint = "https://cat-api.test/images/search"
	randomCatHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.String() != randomCatEndpoint {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`[{"url":"https://cdn.test/cat.jpg"}]`)),
			Header:     make(http.Header),
		}, nil
	})}

	got, err := fetchRandomCat()
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://cdn.test/cat.jpg" {
		t.Fatalf("image URL = %q", got)
	}
}

func TestFetchRandomDogUsesRandomDogAPI(t *testing.T) {
	oldDog := randomDogEndpoint
	oldClient := randomCatHTTPClient
	t.Cleanup(func() {
		randomDogEndpoint = oldDog
		randomCatHTTPClient = oldClient
	})

	randomDogEndpoint = "https://dog-api.test/woof.json?filter=mp4,webm"
	randomCatHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.String() != randomDogEndpoint {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"url":"https://cdn.test/dog.gif","fileSizeBytes":2048}`)),
			Header:     make(http.Header),
		}, nil
	})}

	got, err := fetchRandomDog()
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://cdn.test/dog.gif" {
		t.Fatalf("image URL = %q", got)
	}
}

func TestFetchRandomDogRetriesInvalidThenFallsBackToTheDogAPI(t *testing.T) {
	oldDog := randomDogEndpoint
	oldTheDog := theDogAPIEndpoint
	oldClient := randomCatHTTPClient
	t.Cleanup(func() {
		randomDogEndpoint = oldDog
		theDogAPIEndpoint = oldTheDog
		randomCatHTTPClient = oldClient
	})

	randomDogEndpoint = "https://dog-api.test/woof.json?filter=mp4,webm"
	theDogAPIEndpoint = "https://thedogapi.test/v1/images/search"
	hits := map[string]int{}
	randomCatHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		hits[req.URL.String()]++
		switch req.URL.String() {
		case randomDogEndpoint:
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`{"url":"https://cdn.test/dog.webm","fileSizeBytes":2048}`)),
				Header:     make(http.Header),
			}, nil
		case theDogAPIEndpoint:
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`[{"url":"https://cdn.test/dog.jpg"}]`)),
				Header:     make(http.Header),
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
			return nil, nil
		}
	})}

	got, err := fetchRandomDog()
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://cdn.test/dog.jpg" {
		t.Fatalf("image URL = %q", got)
	}
	if hits[randomDogEndpoint] != randomDogAttempts {
		t.Fatalf("random.dog hits = %d, want %d", hits[randomDogEndpoint], randomDogAttempts)
	}
	if hits[theDogAPIEndpoint] != 1 {
		t.Fatalf("thedogapi hits = %d, want 1", hits[theDogAPIEndpoint])
	}
}

func TestFetchRandomPetUsesCatWhenPickerSelectsCat(t *testing.T) {
	oldPick := pickDog
	oldCat := randomCatEndpoint
	oldClient := randomCatHTTPClient
	t.Cleanup(func() {
		pickDog = oldPick
		randomCatEndpoint = oldCat
		randomCatHTTPClient = oldClient
	})
	pickDog = func() bool { return false }
	randomCatEndpoint = "https://cat-api.test/images/search"
	randomCatHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != randomCatEndpoint {
			t.Fatalf("unexpected request: %s", req.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`[{"url":"https://cdn.test/cat.jpg"}]`)),
			Header:     make(http.Header),
		}, nil
	})}
	got, err := fetchRandomPet()
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://cdn.test/cat.jpg" {
		t.Fatalf("image URL = %q", got)
	}
}

func TestFetchRandomPetUsesDogWhenPickerSelectsDog(t *testing.T) {
	oldPick := pickDog
	oldDog := randomDogEndpoint
	oldClient := randomCatHTTPClient
	t.Cleanup(func() {
		pickDog = oldPick
		randomDogEndpoint = oldDog
		randomCatHTTPClient = oldClient
	})
	pickDog = func() bool { return true }
	randomDogEndpoint = "https://dog-api.test/woof.json?filter=mp4,webm"
	randomCatHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != randomDogEndpoint {
			t.Fatalf("unexpected request: %s", req.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"url":"https://cdn.test/dog.png","fileSizeBytes":111}`)),
			Header:     make(http.Header),
		}, nil
	})}
	got, err := fetchRandomPet()
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://cdn.test/dog.png" {
		t.Fatalf("image URL = %q", got)
	}
}
