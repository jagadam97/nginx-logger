package utils

import (
	"strconv"
	"time"
)

// ConvertTimestamp converts log timestamp format
func ConvertTimestamp(input string) string {
	layout := "02/Jan/2006:15:04:05 +0000"
	parsedTime, err := time.Parse(layout, input)
	if err != nil {
		return ""
	}
	return parsedTime.Format("2006-01-02 15:04:05")
}

// StringToInt converts string to int
func StringToInt(str string) int {
	i, err := strconv.Atoi(str)
	if err != nil {
		return 0
	}
	return i
}

// StringToFloat converts string to float
func StringToFloat(str string) float64 {
	i, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0.0
	}
	return i
}
