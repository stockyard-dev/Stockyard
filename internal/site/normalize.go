package site

import (
	"regexp"
	"strings"
)

// Strip these prefixes to get the core business type
var normPrefixes = []string{
	"i run a ", "i own a ", "i manage a ", "i operate a ",
	"i have a ", "i work at a ", "i work as a ", "i work in a ",
	"i'm a ", "im a ", "i am a ", "i am an ",
	"we run a ", "we own a ", "we are a ", "we have a ",
	"we operate a ", "my ", "our ", "the ",
	"small ", "little ", "local ", "new ", "growing ",
	"solo ", "independent ", "private ", "family-owned ",
	"family owned ",
}

// Strip location/detail suffixes
var normSuffixes = []*regexp.Regexp{
	regexp.MustCompile(`\s+(?:in|near|from|at|on|around|outside)\s+\w+.*$`),
	regexp.MustCompile(`\s+called\s+.+$`),
	regexp.MustCompile(`\s+named\s+.+$`),
	regexp.MustCompile(`\s+with\s+\d+\s+(?:members?|employees?|staff|people|clients?).*$`),
	regexp.MustCompile(`\s+since\s+\d{4}$`),
}

// normalizeInput strips filler to get the core business type.
// "I run a small barber shop called Tony's in Minneapolis" → "barber shop"
func normalizeInput(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))

	// Remove punctuation except hyphens and ampersands
	s = regexp.MustCompile(`[^\w\s\-&]`).ReplaceAllString(s, "")

	// Strip filler prefixes (apply repeatedly since some stack)
	changed := true
	for changed {
		changed = false
		for _, prefix := range normPrefixes {
			if strings.HasPrefix(s, prefix) {
				s = strings.TrimPrefix(s, prefix)
				changed = true
			}
		}
	}

	// Strip location and detail suffixes
	for _, re := range normSuffixes {
		s = re.ReplaceAllString(s, "")
	}

	// Strip remaining articles
	for _, a := range []string{"a ", "an ", "the "} {
		s = strings.ReplaceAll(" "+s+" ", " "+a, " ")
	}

	// Collapse whitespace
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")

	return s
}

// extractBusinessName pulls out a business name from the raw input.
// "I own a barber shop called Tony's in Minneapolis" → "Tony's"
func extractBusinessName(input string) string {
	s := strings.TrimSpace(input)

	// "called Tony's Barber Shop"
	if m := regexp.MustCompile(`(?i)called\s+(.+?)(?:\s+(?:in|on|at|near|from)\s|$)`).FindStringSubmatch(s); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	// "named Main Street Cuts"
	if m := regexp.MustCompile(`(?i)named\s+(.+?)(?:\s+(?:in|on|at|near|from)\s|$)`).FindStringSubmatch(s); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	// Quoted: "Tony's Barber Shop" or 'Tony's Barber Shop'
	if m := regexp.MustCompile(`["']([^"']+)["']`).FindStringSubmatch(s); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	return "" // no business name detected
}
