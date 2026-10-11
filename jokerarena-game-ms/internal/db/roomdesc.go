package db

import (
	"fmt"
	"strconv"
	"strings"
)

// The relational schema (rooms.description is free TEXT) has no dedicated columns for the
// target score or the bot flag, so they are stored in the description as "k=v;k=v".

// encodeRoomDescription serializes the room settings into rooms.description.
func encodeRoomDescription(targetScore int, withBot bool) string {
	return fmt.Sprintf("target_score=%d;with_bot=%t", targetScore, withBot)
}

// decodeRoomDescription parses rooms.description; unknown/missing values fall back to defaults.
func decodeRoomDescription(desc string) (targetScore int, withBot bool) {
	targetScore = 300
	for _, part := range strings.Split(desc, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "target_score":
			if n, err := strconv.Atoi(kv[1]); err == nil && n > 0 {
				targetScore = n
			}
		case "with_bot":
			withBot = kv[1] == "true"
		}
	}
	return targetScore, withBot
}

// statusFromResult maps matches.result to the API status.
func statusFromResult(result string) string {
	switch strings.ToUpper(result) {
	case "WIN", "LOSE":
		return "FINISHED"
	case "IN_PROGRESS":
		return "IN_PROGRESS"
	default:
		return "WAITING"
	}
}
