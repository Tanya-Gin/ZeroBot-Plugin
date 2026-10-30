package cybercat

import "testing"

func TestParseBreedInfoRejectsMissingBreedDetails(t *testing.T) {
	_, err := parseBreedInfo([]byte(`{"url":"https://example.test/cat.jpg","breeds":[]}`))
	if err == nil {
		t.Fatal("parseBreedInfo should reject a response without breed details")
	}
}

func TestParseBreedInfoExtractsBreedDetails(t *testing.T) {
	got, err := parseBreedInfo([]byte(`{
		"url":"https://example.test/cat.jpg",
		"breeds":[{"name":"Ragdoll","temperament":"Calm","description":"Friendly"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.TypeName != "布偶猫" || got.Temperament != "Calm" || got.Description != "Friendly" || got.ImageURL == "" {
		t.Fatalf("parsed breed info = %+v", got)
	}
}
