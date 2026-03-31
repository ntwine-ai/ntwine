package harness

import (
	"fmt"
	"strings"
)

func sanitizeKey(key string) (string, error) {
	if len(key) > 200 {
		return "", fmt.Errorf("key too long (max 200 chars)")
	}

	if key == "." || key == ".." || strings.HasPrefix(key, "../") || strings.Contains(key, "/../") {
		return "", fmt.Errorf("invalid key")
	}

	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' ||
			r == '"' || r == '<' || r == '>' || r == '|' || r == '\x00' || r == '.' {
			return '_'
		}
		return r
	}, key)

	if safe == "" {
		return "", fmt.Errorf("key is empty after sanitization")
	}

	return safe, nil
}
