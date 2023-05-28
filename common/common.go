package common

import "strings"

func ToArray(input string) []string {
	tokens := strings.Split(input, ";")
	result := make([]string, 0)
	for _, next := range tokens {
		next = strings.TrimSpace(next)
		if next == "" {
			continue
		}
		result = append(result, next)
	}
	return result
}
