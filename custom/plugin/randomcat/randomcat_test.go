package randomcat

import (
	"strings"
	"testing"
)

func TestParseRandomCatResponseReturnsImageURL(t *testing.T) {
	got, err := parseRandomCatResponse([]byte(`[{"id":"abc","url":"https://cdn2.thecatapi.com/images/abc.jpg"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://cdn2.thecatapi.com/images/abc.jpg" {
		t.Fatalf("image URL = %q", got)
	}
}

func TestParseRandomCatResponseRejectsEmptyOrUnsafeURL(t *testing.T) {
	cases := []string{
		`[]`,
		`[{"url":""}]`,
		`[{"url":"http://example.test/cat.jpg"}]`,
		`[{"url":"javascript:alert(1)"}]`,
	}
	for _, input := range cases {
		if _, err := parseRandomCatResponse([]byte(input)); err == nil {
			t.Errorf("parseRandomCatResponse(%s) returned nil error", input)
		}
	}
}

func TestParseRandomDogResponseReturnsHTTPSImageURL(t *testing.T) {
	got, err := parseRandomDogResponse([]byte(`{"url":"https://random.dog/abc.gif","fileSizeBytes":12345}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://random.dog/abc.gif" {
		t.Fatalf("image URL = %q", got)
	}
}

func TestParseRandomDogResponseRejectsVideoUnsafeOrOversized(t *testing.T) {
	cases := []string{
		`{"url":"https://random.dog/abc.webm","fileSizeBytes":12345}`,
		`{"url":"https://random.dog/abc.mp4","fileSizeBytes":12345}`,
		`{"url":"http://random.dog/abc.jpg","fileSizeBytes":12345}`,
		`{"url":"javascript:alert(1)","fileSizeBytes":12345}`,
		`{"url":"https://random.dog/abc.gif","fileSizeBytes":5000000}`,
		`{"url":"","fileSizeBytes":12345}`,
		`[]`,
	}
	for _, input := range cases {
		if _, err := parseRandomDogResponse([]byte(input)); err == nil {
			t.Errorf("parseRandomDogResponse(%s) returned nil error", input)
		}
	}
}

func TestPluginHelpListsCatAndDogCommands(t *testing.T) {
	help := pluginHelp()
	for _, want := range []string{"随机猫猫", "随机狗狗", "随机猫狗"} {
		if !strings.Contains(help, want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestFetchByCommandRoutesDogAndPet(t *testing.T) {
	if cmd := failText("随机狗狗"); cmd != "随机狗狗获取失败，请稍后再试喵~" {
		t.Fatalf("dog fail text = %q", cmd)
	}
	if cmd := failText("随机猫狗"); cmd != "随机猫狗获取失败，请稍后再试喵~" {
		t.Fatalf("pet fail text = %q", cmd)
	}
	if cmd := failText("随机猫猫"); cmd != "随机猫猫获取失败，请稍后再试喵~" {
		t.Fatalf("cat fail text = %q", cmd)
	}
}
