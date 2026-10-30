package cybercat

import (
	"regexp"
	"strconv"
	"strings"

	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var arenaPKCommand = regexp.MustCompile(`^(喵喵|猫猫)(PK|pk)$`)

func matchArenaPK(ctx *zero.Ctx) bool {
	segments := ctx.Event.Message
	if target := arenaTargetFromSegments(segments); target != 0 {
		ctx.State["arena_target"] = target
		return true
	}
	// Use the original OneBot segments as a fallback if preprocessing changed them.
	segments = message.ParseMessage(ctx.Event.NativeMessage)
	if target := arenaTargetFromSegments(segments); target != 0 {
		ctx.State["arena_target"] = target
		return true
	}
	return false
}

func arenaTargetFromSegments(segments message.Message) int64 {
	var prefix strings.Builder
	for _, segment := range segments {
		if segment.Type == "at" {
			if !arenaPKCommand.MatchString(strings.TrimSpace(prefix.String())) {
				return 0
			}
			target, err := strconv.ParseInt(segment.Data["qq"], 10, 64)
			if err != nil || target <= 0 {
				return 0
			}
			return target
		}
		if segment.Type == "text" {
			prefix.WriteString(segment.Data["text"])
		}
	}
	return 0
}
