// Package formats - Text chunking implementation
package formats

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/NortonBen/ai-memory-go/schema"
)

// TextParser implements the Parser interface for text content
type TextParser struct {
	config *schema.ChunkingConfig
}

// NewTextParser creates a new text parser with the given configuration
func NewTextParser(config *schema.ChunkingConfig) *TextParser {
	if config == nil {
		config = schema.DefaultChunkingConfig()
	}
	if config.MaxSize <= 0 {
		config.MaxSize = schema.DefaultChunkingConfig().MaxSize
	}
	if config.MinSize < 0 {
		config.MinSize = 0
	}
	if config.Overlap < 0 {
		config.Overlap = 0
	}
	// Guard against invalid window that can cause non-progressing loops.
	if config.Overlap >= config.MaxSize {
		config.Overlap = config.MaxSize - 1
		if config.Overlap < 0 {
			config.Overlap = 0
		}
	}
	return &TextParser{config: config}
}

// ParseText implements text chunking based on the configured strategy
func (tp *TextParser) ParseText(ctx context.Context, content string) ([]*schema.Chunk, error) {
	switch tp.config.Strategy {
	case schema.StrategyParagraph:
		return tp.chunkByParagraph(content, "text")
	case schema.StrategySentence:
		return tp.chunkBySentence(content, "text")
	case schema.StrategyFixedSize:
		return tp.chunkByFixedSize(content, "text")
	case schema.StrategySemantic:
		return tp.chunkBySemantic(content, "text")
	default:
		return tp.chunkByParagraph(content, "text")
	}
}

// ParseFile parses a file based on its extension
func (tp *TextParser) ParseFile(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	// This will be implemented with file reading logic
	// For now, return empty slice
	return []*schema.Chunk{}, nil
}

// ParseMarkdown parses markdown content with structure preservation
func (tp *TextParser) ParseMarkdown(ctx context.Context, content string) ([]*schema.Chunk, error) {
	return tp.chunkByParagraph(content, "markdown")
}

// ParsePDF parses PDF files using the PDFParser
func (tp *TextParser) ParsePDF(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	return nil, fmt.Errorf("PDF parsing not implemented in TextParser")
}

// DetectContentType detects the type of content
func (tp *TextParser) DetectContentType(content string) schema.ChunkType {
	// Simple heuristics for content type detection
	if strings.Contains(content, "```") ||
		strings.Contains(content, "func ") ||
		strings.Contains(content, "class ") ||
		strings.Contains(content, "def ") ||
		strings.Contains(content, "function ") ||
		strings.Contains(content, "import ") ||
		strings.Contains(content, "package ") {
		return schema.ChunkTypeCode
	}
	if (strings.Contains(content, "#") && strings.Contains(content, "\n")) ||
		strings.Contains(content, "**") ||
		strings.Contains(content, "[") && strings.Contains(content, "](") {
		return schema.ChunkTypeMarkdown
	}
	return schema.ChunkTypeText
}

// chunkByParagraph splits text into paragraph-based chunks
func (tp *TextParser) chunkByParagraph(content, source string) ([]*schema.Chunk, error) {
	// Split by double newlines (paragraphs)
	paragraphs := regexp.MustCompile(`\n\s*\n`).Split(content, -1)

	chunks := make([]*schema.Chunk, 0)
	currentChunk := ""

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// If a single paragraph is larger than MaxSize, split it safely.
		if len([]rune(para)) > tp.config.MaxSize {
			if currentChunk != "" && len(currentChunk) >= tp.config.MinSize {
				chunk := schema.NewChunk(currentChunk, source, schema.ChunkTypeParagraph)
				chunks = append(chunks, chunk)
				currentChunk = ""
			}
			chunks = append(chunks, tp.splitLargeUnit(para, source, schema.ChunkTypeParagraph)...)
			continue
		}

		// If adding this paragraph exceeds max size, save current chunk
		if len(currentChunk)+len(para)+2 > tp.config.MaxSize && currentChunk != "" {
			// Only create chunk if it meets minimum size
			if len(currentChunk) >= tp.config.MinSize {
				chunk := schema.NewChunk(currentChunk, source, schema.ChunkTypeParagraph)
				chunks = append(chunks, chunk)
			}

			// Start new chunk with overlap
			if tp.config.Overlap > 0 && len(currentChunk) > tp.config.Overlap {
				candidate := getLastNChars(currentChunk, tp.config.Overlap) + "\n\n" + para
				if len(candidate) <= tp.config.MaxSize {
					currentChunk = candidate
				} else {
					currentChunk = para
				}
			} else {
				currentChunk = para
			}
		} else {
			if currentChunk != "" {
				currentChunk += "\n\n" + para
			} else {
				currentChunk = para
			}
		}
	}

	// Add remaining chunk if it meets minimum size
	if currentChunk != "" && len(currentChunk) >= tp.config.MinSize {
		chunk := schema.NewChunk(currentChunk, source, schema.ChunkTypeParagraph)
		chunks = append(chunks, chunk)
	}

	// If no chunks were created but we have content, create one chunk regardless of MinSize
	if len(chunks) == 0 && strings.TrimSpace(content) != "" {
		chunk := schema.NewChunk(strings.TrimSpace(content), source, schema.ChunkTypeParagraph)
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// chunkBySentence splits text into sentence-based chunks
func (tp *TextParser) chunkBySentence(content, source string) ([]*schema.Chunk, error) {
	sentences := splitIntoSentences(content)

	chunks := make([]*schema.Chunk, 0)
	currentChunk := ""
	var currentSentences []string

	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}

		// If a single sentence is larger than MaxSize, split it safely.
		if len([]rune(sentence)) > tp.config.MaxSize {
			if currentChunk != "" && len(currentChunk) >= tp.config.MinSize {
				chunk := schema.NewChunk(currentChunk, source, schema.ChunkTypeSentence)
				chunks = append(chunks, chunk)
			}
			chunks = append(chunks, tp.splitLargeUnit(sentence, source, schema.ChunkTypeSentence)...)
			currentChunk = ""
			currentSentences = nil
			continue
		}

		// If adding this sentence exceeds max size, save current chunk
		if len(currentChunk)+len(sentence)+1 > tp.config.MaxSize && currentChunk != "" {
			if len(currentChunk) >= tp.config.MinSize {
				chunk := schema.NewChunk(currentChunk, source, schema.ChunkTypeSentence)
				chunks = append(chunks, chunk)
			}

			// Get overlap sentences
			overlapText := ""
			if tp.config.Overlap > 0 {
				overlapText = tp.getOverlapSentences(currentSentences)
			}

			if overlapText != "" {
				candidate := overlapText + " " + sentence
				if len(candidate) <= tp.config.MaxSize {
					currentChunk = candidate
				} else {
					currentChunk = sentence
				}
				// Reset currentSentences with overlap sentences + current sentence
				currentSentences = append(splitIntoSentences(overlapText), sentence)
			} else {
				currentChunk = sentence
				currentSentences = []string{sentence}
			}
		} else {
			if currentChunk != "" {
				currentChunk += " " + sentence
			} else {
				currentChunk = sentence
			}
			currentSentences = append(currentSentences, sentence)
		}
	}

	// Add remaining chunk if it meets minimum size
	if currentChunk != "" && len(currentChunk) >= tp.config.MinSize {
		chunk := schema.NewChunk(currentChunk, source, schema.ChunkTypeSentence)
		chunks = append(chunks, chunk)
	}

	// If no chunks were created but we have content, create one chunk regardless of MinSize
	if len(chunks) == 0 && strings.TrimSpace(content) != "" {
		chunk := schema.NewChunk(strings.TrimSpace(content), source, schema.ChunkTypeSentence)
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// getOverlapSentences gets as many sentences as possible from the end of a sentence list within overlap limit.
// It ensures at least one sentence is returned if Overlap > 0 and sentences are available.
func (tp *TextParser) getOverlapSentences(sentences []string) string {
	if len(sentences) == 0 || tp.config.Overlap <= 0 {
		return ""
	}

	var overlapSentences []string
	currentLen := 0

	// Go backwards from the end
	for i := len(sentences) - 1; i >= 0; i-- {
		sent := sentences[i]
		// If this is the first sentence we're adding (the very last one in the list),
		// we add it anyway to ensure at least one connecting sentence exists.
		if len(overlapSentences) > 0 && currentLen+len(sent)+1 > tp.config.Overlap {
			break
		}
		overlapSentences = append([]string{sent}, overlapSentences...)
		currentLen += len(sent) + 1
	}

	return strings.Join(overlapSentences, " ")
}

// chunkByFixedSize splits text into fixed-size chunks with overlap
func (tp *TextParser) chunkByFixedSize(content, source string) ([]*schema.Chunk, error) {
	chunks := make([]*schema.Chunk, 0)
	contentRunes := []rune(content)
	step := tp.config.MaxSize - tp.config.Overlap
	if step <= 0 {
		step = tp.config.MaxSize
	}

	for i := 0; i < len(contentRunes); i += step {
		end := i + tp.config.MaxSize
		if end > len(contentRunes) {
			end = len(contentRunes)
		}

		chunkContent := string(contentRunes[i:end])
		chunkContent = strings.TrimSpace(chunkContent)

		if len(chunkContent) >= tp.config.MinSize {
			chunk := schema.NewChunk(chunkContent, source, schema.ChunkTypeText)
			chunks = append(chunks, chunk)
		}

		// Break if we've reached the end
		if end >= len(contentRunes) {
			break
		}
	}

	return chunks, nil
}

// splitLargeUnit splits a single oversized paragraph/sentence into fixed windows with overlap.
func (tp *TextParser) splitLargeUnit(unit, source string, chunkType schema.ChunkType) []*schema.Chunk {
	var out []*schema.Chunk
	runes := []rune(strings.TrimSpace(unit))
	if len(runes) == 0 {
		return out
	}

	step := tp.config.MaxSize - tp.config.Overlap
	if step <= 0 {
		step = tp.config.MaxSize
	}

	for i := 0; i < len(runes); i += step {
		end := i + tp.config.MaxSize
		if end > len(runes) {
			end = len(runes)
		}
		part := strings.TrimSpace(string(runes[i:end]))
		if part != "" {
			out = append(out, schema.NewChunk(part, source, chunkType))
		}
		if end >= len(runes) {
			break
		}
	}
	return out
}

// chunkBySemantic splits text using semantic boundaries (simplified version)
func (tp *TextParser) chunkBySemantic(content, source string) ([]*schema.Chunk, error) {
	// For now, use paragraph-based chunking as a baseline
	// A full semantic chunking would require embeddings and similarity analysis
	return tp.chunkByParagraph(content, source)
}

// getLastNChars returns the last N characters of a string
func getLastNChars(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[len(runes)-n:])
}

// splitIntoSentences splits text into sentences preserving punctuation
func splitIntoSentences(text string) []string {
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

// SplitIntoWords splits text into words for keyword extraction
func SplitIntoWords(text string) []string {
	// Split by whitespace and punctuation
	words := make([]string, 0)
	currentWord := ""

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			currentWord += string(r)
		} else {
			if currentWord != "" {
				words = append(words, strings.ToLower(currentWord))
				currentWord = ""
			}
		}
	}

	if currentWord != "" {
		words = append(words, strings.ToLower(currentWord))
	}

	return words
}

// RemoveStopWords removes common stop words from a word list
func RemoveStopWords(words []string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "must": true,
		"can": true, "this": true, "that": true, "these": true, "those": true,
		"i": true, "you": true, "he": true, "she": true, "it": true,
		"we": true, "they": true, "what": true, "which": true, "who": true,
		"when": true, "where": true, "why": true, "how": true,
	}

	filtered := make([]string, 0)
	for _, word := range words {
		if !stopWords[strings.ToLower(word)] && len(word) > 2 {
			filtered = append(filtered, word)
		}
	}

	return filtered
}
