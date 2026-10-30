package cybercat

import (
	"testing"

	"github.com/wdvxdr1123/ZeroBot/message"
)

func TestArenaTargetFromSegmentsIgnoresMentionDisplayName(t *testing.T) {
	for _, command := range []string{"喵喵PK", "猫猫PK", "喵喵pk", "猫猫pk"} {
		segments := message.Message{
			{Type: "text", Data: map[string]string{"text": command + " "}},
			{Type: "at", Data: map[string]string{"qq": "123456", "name": "猫咪[✨]·特殊名"}},
			{Type: "text", Data: map[string]string{"text": " 约战"}},
		}
		if got := arenaTargetFromSegments(segments); got != 123456 {
			t.Errorf("target for %q with special display name = %d, want 123456", command, got)
		}
	}
}

func TestArenaTargetFromSegmentsRejectsInvalidMention(t *testing.T) {
	cases := []message.Message{
		{{Type: "text", Data: map[string]string{"text": "猫猫PK "}}},
		{{Type: "text", Data: map[string]string{"text": "猫猫PK "}}, {Type: "at", Data: map[string]string{"qq": "昵称✨"}}},
		{{Type: "text", Data: map[string]string{"text": "猫猫PK其他 "}}, {Type: "at", Data: map[string]string{"qq": "123456"}}},
	}
	for i, segments := range cases {
		if got := arenaTargetFromSegments(segments); got != 0 {
			t.Errorf("case %d got invalid target %d", i, got)
		}
	}
}
