package cybercat

import "testing"

func TestParseBreedCatalogExtractsUsableBreed(t *testing.T) {
	got, err := parseBreedCatalog([]byte(`[
		{"id":"ragd","name":"Ragdoll","temperament":"Calm","description":"Friendly"}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "ragd" || got[0].Info.TypeName != "布偶猫" || got[0].Info.Temperament != "Calm" {
		t.Fatalf("parsed catalog = %+v", got)
	}
}

func TestParseBreedCatalogSkipsEntriesWithoutRequiredFields(t *testing.T) {
	got, err := parseBreedCatalog([]byte(`[
		{"id":"","name":"No ID"},
		{"id":"ragd","name":"Ragdoll"}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "ragd" {
		t.Fatalf("parsed catalog = %+v", got)
	}
}
