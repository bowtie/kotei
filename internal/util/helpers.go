package util

import (
	"strconv"
	"strings"

	"github.com/fatih/color"
)

var (
	Red        = color.New(color.FgRed).SprintFunc()
	RedBold    = color.New(color.FgRed, color.Bold).SprintFunc()
	Green      = color.New(color.FgGreen).SprintFunc()
	GreenBold  = color.New(color.FgGreen, color.Bold).SprintFunc()
	Yellow     = color.New(color.FgYellow).SprintFunc()
	YellowBold = color.New(color.FgYellow, color.Bold).SprintFunc()
	Blue       = color.New(color.FgBlue).SprintFunc()
	BlueBold   = color.New(color.FgBlue, color.Bold).SprintFunc()
	Purple     = color.New(color.FgMagenta).SprintFunc()
	PurpleBold = color.New(color.FgMagenta, color.Bold).SprintFunc()
	Cyan       = color.New(color.FgCyan).SprintFunc()
	CyanBold   = color.New(color.FgCyan, color.Bold).SprintFunc()
	Gray       = color.New(color.FgHiBlack).SprintFunc()
	GrayBold   = color.New(color.FgWhite, color.Bold).SprintFunc()
	Bold       = color.New(color.Bold).SprintFunc()
)

func AtoiSimple(s string) int {
	i, _ := strconv.Atoi(strings.TrimSpace(s))
	return i
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Iif(condition bool, trueVal, falseVal string) string {
	if condition {
		return trueVal
	}
	return falseVal
}

func AddEpisodesToMap(targetMap map[int]bool, episodes []int) {
	for _, ep := range episodes {
		targetMap[ep] = true
	}
}

func GetOrdinalSuffix(day int) string {
	if day <= 0 {
		return ""
	}
	if day%100 >= 11 && day%100 <= 13 {
		return "th"
	}
	switch day % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
}
