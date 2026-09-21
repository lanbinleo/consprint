package backend

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"gorm.io/datatypes"
)

func NewID(prefix string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return prefix + "_" + hex.EncodeToString(b[:])
}

func NormalizeTerm(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "’", "'")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func Slugify(s string) string {
	s = NormalizeTerm(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "item"
	}
	return out
}

var unitNumRE = regexp.MustCompile(`(?i)^unit\s+(\d+)`)
var topicNumRE = regexp.MustCompile(`^(\d+)\.(\d+)`)

func UnitSlug(title string, position int) string {
	if strings.HasPrefix(strings.ToLower(title), "science practices") {
		return "science-practices"
	}
	if m := unitNumRE.FindStringSubmatch(title); len(m) == 2 {
		return "u" + m[1]
	}
	return "unit-" + Slugify(title)
}

func TopicSlug(title string, position int) string {
	if strings.HasPrefix(strings.ToLower(title), "set ") {
		return Slugify(title)
	}
	if m := topicNumRE.FindStringSubmatch(title); len(m) == 3 {
		return "t" + m[1] + "-" + m[2]
	}
	return "topic-" + Slugify(title)
}

func Clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func env(key, fallbackValue string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return fallbackValue
}

func envInt(key string, fallbackValue int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallbackValue
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return fallbackValue
}

func envBool(key string, fallbackValue bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallbackValue
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

// productionMode reports whether the server runs with deploy hardening
// expected. It mirrors the check in main.requireDeployConfig so backend
// gates (registration admin grants, startup admin promotion) enforce the
// same boundary.
func productionMode() bool {
	return envBool("APP_REQUIRE_SECURE_CONFIG", false) ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("GIN_MODE")), "release")
}

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return strings.TrimSpace(v)
}

func marshalBlocks(blocks []map[string]string) datatypes.JSON {
	if len(blocks) == 0 {
		return datatypes.JSON([]byte("[]"))
	}
	normalized := make([]map[string]string, 0, len(blocks))
	for _, block := range blocks {
		text := strings.TrimSpace(block["text"])
		if text == "" {
			continue
		}
		kind := strings.TrimSpace(block["type"])
		if kind == "" {
			kind = "paragraph"
		}
		normalized = append(normalized, map[string]string{"type": kind, "text": text})
	}
	if len(normalized) == 0 {
		return datatypes.JSON([]byte("[]"))
	}
	out, _ := json.Marshal(normalized)
	return datatypes.JSON(out)
}

var errNotFound = errors.New("not found")
