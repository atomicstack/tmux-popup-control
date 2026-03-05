package gotmuxcc

import (
	"strconv"
	"strings"
)

func checkSessionName(name string) bool {
	if name == "" {
		return false
	}
	if strings.Contains(name, ":") {
		return false
	}
	if strings.Contains(name, ".") {
		return false
	}
	return true
}

func isOne(value string) bool {
	return value == "1"
}

func parseList(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, ",")
}

func atoi(value string) int {
	n, _ := strconv.Atoi(value)
	return n
}

func atoi32(value string) int32 {
	return int32(atoi(value))
}
