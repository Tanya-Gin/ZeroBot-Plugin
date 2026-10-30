package cybercat

import "testing"

func TestParseSearchBreedInfoSkipsImagesWithoutBreedDetails(t *testing.T) {
	got, err := parseSearchBreedInfo([]byte(`[
		{"id":"without-breed","url":"https://example.test/empty.jpg","breeds":[]},
		{"id":"with-breed","url":"https://example.test/cat.jpg","breeds":[{"name":"Ragdoll","temperament":"Calm","description":"Friendly"}]}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if got.TypeName != "布偶猫" || got.Temperament != "Calm" || got.Description != "Friendly" || got.ImageURL == "" {
		t.Fatalf("parsed search breed info = %+v", got)
	}
}

func TestParseSearchBreedInfoRejectsBatchWithoutUsableBreed(t *testing.T) {
	_, err := parseSearchBreedInfo([]byte(`[
		{"id":"without-breed","url":"https://example.test/empty.jpg","breeds":[]}
	]`))
	if err == nil {
		t.Fatal("parseSearchBreedInfo should reject a batch without usable breed details")
	}
}
