package index

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// splitForEmbedding divides text into pieces that each fit within limit
// bytes.
//
// Embedding models have small inputs — a 512-token sentence encoder is
// the norm — and an oversized request is rejected rather than clipped.
// Truncating to fit would silently drop the tail of every long frame and
// make it unsearchable, with nothing in the index to indicate content is
// missing. Splitting keeps all of it.
//
// Breaks are preferred at natural boundaries, strongest first: blank
// line, newline, sentence end, then any space. A chunk that ends
// mid-sentence still embeds, but its vector blends two unrelated
// thoughts and retrieves poorly, so the boundary is worth some slack.
// Only when a run has no boundary at all — minified JSON, a base64 blob
// — does it cut at the limit, always on a rune boundary so the result
// stays valid UTF-8.
func splitForEmbedding(text string, limit int) []string {
	if limit <= 0 || len(text) <= limit {
		return []string{text}
	}

	var out []string
	rest := text
	for len(rest) > limit {
		cut := breakPoint(rest, limit)
		piece := strings.TrimRight(rest[:cut], " \t\r\n")
		if piece != "" {
			out = append(out, piece)
		}
		rest = strings.TrimLeft(rest[cut:], " \t\r\n")
	}
	if rest != "" {
		out = append(out, rest)
	}
	if len(out) == 0 {
		// Whitespace-only input: keep one piece so the caller still
		// records a frame rather than silently skipping it.
		return []string{text}
	}
	return out
}

// breakPoint picks where to cut s so the first piece is at most limit
// bytes.
//
// minFill stops a boundary search from producing a tiny fragment: a
// blank line 20 bytes in shouldn't yield a 20-byte chunk when the budget
// is 1500. Below that fraction the weaker boundary is preferred.
func breakPoint(s string, limit int) int {
	const minFill = 0.5
	floor := int(float64(limit) * minFill)

	window := s[:limit]

	if i := strings.LastIndex(window, "\n\n"); i >= floor {
		return i + 2
	}
	if i := strings.LastIndexByte(window, '\n'); i >= floor {
		return i + 1
	}
	if i := lastSentenceEnd(window); i >= floor {
		return i
	}
	if i := strings.LastIndexByte(window, ' '); i >= floor {
		return i + 1
	}

	// No usable boundary — cut at the limit, backing off to the start
	// of the last complete rune.
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	if cut == 0 {
		// A single rune longer than the limit. Take it whole; one
		// oversized chunk is better than an infinite loop.
		_, size := utf8.DecodeRuneInString(s)
		return size
	}
	return cut
}

// lastSentenceEnd returns the offset just past the final sentence
// terminator followed by whitespace, or -1.
//
// The trailing-whitespace requirement avoids treating the dot in
// "config.yaml" or "v1.2" as a sentence break.
func lastSentenceEnd(s string) int {
	for i := len(s) - 1; i > 0; i-- {
		if !unicode.IsSpace(rune(s[i])) {
			continue
		}
		switch s[i-1] {
		case '.', '!', '?':
			return i + 1
		}
	}
	return -1
}
