package engine

import (
	"regexp"
	"strings"
)

// splitIntoSentences splits text into sentences based on punctuation.
func splitIntoSentences(text string) []string {
	// Reusing the logic from parser/formats/text.go
	re := regexp.MustCompile(`[.!?]+[\s\n]*`)
	matches := re.FindAllStringIndex(text, -1)

	sentences := make([]string, 0)
	lastIndex := 0
	for _, match := range matches {
		sent := strings.TrimSpace(text[lastIndex:match[1]])
		if sent != "" {
			sentences = append(sentences, sent)
		}
		lastIndex = match[1]
	}

	if lastIndex < len(text) {
		last := strings.TrimSpace(text[lastIndex:])
		if last != "" {
			sentences = append(sentences, last)
		}
	}
	return sentences
}

// splitContextIntoSegmentsV2 splits context into segments that respect sentence boundaries.
func (e *defaultMemoryEngine) splitContextIntoSegmentsV2(context string, maxLength int) []string {
	if maxLength <= 0 {
		return []string{context}
	}

	sentences := splitIntoSentences(context)
	var segments []string
	var currentSegment strings.Builder

	for _, sentence := range sentences {
		// If adding this sentence exceeds maxLength, and we already have content, push currentSegment
		if currentSegment.Len() > 0 && currentSegment.Len()+len(sentence)+1 > maxLength {
			segments = append(segments, currentSegment.String())
			currentSegment.Reset()
		}

		// If a single sentence is longer than maxLength, we have to split it by character
		if len(sentence) > maxLength {
			// If currentSegment has content, push it first
			if currentSegment.Len() > 0 {
				segments = append(segments, currentSegment.String())
				currentSegment.Reset()
			}
			// Slicing the long sentence
			runes := []rune(sentence)
			for i := 0; i < len(runes); i += maxLength {
				end := i + maxLength
				if end > len(runes) {
					end = len(runes)
				}
				segments = append(segments, string(runes[i:end]))
			}
			continue
		}

		if currentSegment.Len() > 0 {
			currentSegment.WriteString(" ")
		}
		currentSegment.WriteString(sentence)
	}

	if currentSegment.Len() > 0 {
		segments = append(segments, currentSegment.String())
	}

	return segments
}
