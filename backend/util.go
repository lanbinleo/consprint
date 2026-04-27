package backend

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"
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
