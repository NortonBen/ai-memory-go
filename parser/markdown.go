// Package parser - Markdown parser with structure preservation
package parser

import (
	"context"
	"regexp"
	"strings"
)

// MarkdownParser implements parsing for Markdown files with structure preservation
type MarkdownParser struct {
	config *ChunkingConfig
}

// NewMarkdownParser creates a new Markdown parser
func NewMarkdownParser(config *ChunkingConfig) *MarkdownParser {
	if config == nil {
		config = DefaultChunkingConfig()
		config.PreserveStructure = true
	}
	return &MarkdownParser{config: config}
}

// ParseMarkdown parses Markdown content while preserving structure
func (mp *MarkdownParser) ParseMarkdown(ctx context.Context, content string) ([]Chunk, error) {
	// Parse markdown into structured sections
	sections := mp.parseMarkdownSections(content)

	// Convert sections to chunks
	chunks := make([]Chunk, 0)
	for _, section := range sections {
		chunk := mp.sectionToChunk(section, "markdown")
		chunks = append(chunks, *chunk)
	}

	return chunks, nil
}

// MarkdownSection represents a section of markdown content
type MarkdownSection struct {
	Level       int                    // Header level (1-6)
	Title       string                 // Section title
	Content     string                 // Section content
	Type        string                 // Section type: header, list, code, paragraph, table
	Metadata    map[string]interface{} // Additional metadata
	Subsections []*MarkdownSection     // Nested subsections
}

// parseMarkdownSections parses markdown into hierarchical sections
func (mp *MarkdownParser) parseMarkdownSections(content string) []*MarkdownSection {
	lines := strings.Split(content, "\n")
	sections := make([]*MarkdownSection, 0)

	var currentSection *MarkdownSection
	var currentContent strings.Builder

	headerRegex := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	codeBlockRegex := regexp.MustCompile("^```(\\w*)")
	listRegex := regexp.MustCompile(`^[\s]*[-*+]\s+(.+)$`)
	numberedListRegex := regexp.MustCompile(`^[\s]*\d+\.\s+(.+)$`)

	inCodeBlock := false
	codeBlockLang := ""

	for _, line := range lines {
		// Check for code block
		if matches := codeBlockRegex.FindStringSubmatch(line); len(matches) > 0 {
			if !inCodeBlock {
				// Start of code block
				inCodeBlock = true
				codeBlockLang = matches[1]
				if currentContent.Len() > 0 {
					// Save previous content
					if currentSection != nil {
						currentSection.Content = strings.TrimSpace(currentContent.String())
					}
					currentContent.Reset()
				}
				currentContent.WriteString(line + "\n")
			} else {
				// End of code block
				currentContent.WriteString(line + "\n")
				section := &MarkdownSection{
					Type:    "code",
					Content: currentContent.String(),
					Metadata: map[string]interface{}{
						"language": codeBlockLang,
					},
				}
				sections = append(sections, section)
				currentContent.Reset()
				inCodeBlock = false
				codeBlockLang = ""
			}
			continue
		}

		if inCodeBlock {
			currentContent.WriteString(line + "\n")
			continue
		}

		// Check for header
		if matches := headerRegex.FindStringSubmatch(line); len(matches) > 0 {
			// Save previous section
			if currentSection != nil && currentContent.Len() > 0 {
				currentSection.Content = strings.TrimSpace(currentContent.String())
			}

			// Create new section
			level := len(matches[1])
			title := matches[2]
			currentSection = &MarkdownSection{
				Level:    level,
				Title:    title,
				Type:     "header",
				Metadata: make(map[string]interface{}),
			}
			sections = append(sections, currentSection)
			currentContent.Reset()
			continue
		}

		// Check for list items
		if listRegex.MatchString(line) || numberedListRegex.MatchString(line) {
			if currentSection == nil || currentSection.Type != "list" {
				// Save previous content
				if currentSection != nil && currentContent.Len() > 0 {
					currentSection.Content = strings.TrimSpace(currentContent.String())
					currentContent.Reset()
				}

				// Create new list section
				currentSection = &MarkdownSection{
					Type:     "list",
					Metadata: make(map[string]interface{}),
				}
				sections = append(sections, currentSection)
			}
			currentContent.WriteString(line + "\n")
			continue
		}

		// Regular content
		if currentSection == nil {
			currentSection = &MarkdownSection{
				Type:     "paragraph",
				Metadata: make(map[string]interface{}),
			}
			sections = append(sections, currentSection)
		}

		currentContent.WriteString(line + "\n")
	}

	// Save last section
	if currentSection != nil && currentContent.Len() > 0 {
		currentSection.Content = strings.TrimSpace(currentContent.String())
	}

	return sections
}

// sectionToChunk converts a MarkdownSection to a Chunk
func (mp *MarkdownParser) sectionToChunk(section *MarkdownSection, source string) *Chunk {
	content := section.Content
	if section.Title != "" {
		content = section.Title + "\n\n" + content
	}

	chunk := NewChunk(content, source, ChunkTypeMarkdown)

	// Add section metadata
	chunk.Metadata["section_type"] = section.Type
	if section.Level > 0 {
		chunk.Metadata["header_level"] = section.Level
	}
	if section.Title != "" {
		chunk.Metadata["title"] = section.Title
	}

	// Merge additional metadata
	for k, v := range section.Metadata {
		chunk.Metadata[k] = v
	}

	return chunk
}

// ExtractMarkdownLinks extracts all links from markdown content
func ExtractMarkdownLinks(content string) []map[string]string {
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+)\)`)
	matches := linkRegex.FindAllStringSubmatch(content, -1)

	links := make([]map[string]string, 0)
	for _, match := range matches {
		if len(match) >= 3 {
			links = append(links, map[string]string{
				"text": match[1],
				"url":  match[2],
			})
		}
	}

	return links
}

// ExtractMarkdownImages extracts all images from markdown content
func ExtractMarkdownImages(content string) []map[string]string {
	imageRegex := regexp.MustCompile(`!\[([^\]]*)\]\(([^\)]+)\)`)
	matches := imageRegex.FindAllStringSubmatch(content, -1)

	images := make([]map[string]string, 0)
	for _, match := range matches {
		if len(match) >= 3 {
			images = append(images, map[string]string{
				"alt": match[1],
				"url": match[2],
			})
		}
	}

	return images
}

// ExtractMarkdownCodeBlocks extracts all code blocks with their languages
func ExtractMarkdownCodeBlocks(content string) []map[string]string {
	codeBlockRegex := regexp.MustCompile("(?s)```(\\w*)\\n(.*?)```")
	matches := codeBlockRegex.FindAllStringSubmatch(content, -1)

	codeBlocks := make([]map[string]string, 0)
	for _, match := range matches {
		if len(match) >= 3 {
			codeBlocks = append(codeBlocks, map[string]string{
				"language": match[1],
				"code":     match[2],
			})
		}
	}

	return codeBlocks
}

// ExtractMarkdownHeaders extracts all headers with their levels
func ExtractMarkdownHeaders(content string) []map[string]interface{} {
	headerRegex := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	matches := headerRegex.FindAllStringSubmatch(content, -1)

	headers := make([]map[string]interface{}, 0)
	for _, match := range matches {
		if len(match) >= 3 {
			headers = append(headers, map[string]interface{}{
				"level": len(match[1]),
				"text":  match[2],
			})
		}
	}

	return headers
}

// MarkdownToPlainText converts markdown to plain text by removing formatting
func MarkdownToPlainText(content string) string {
	// Remove code blocks
	codeBlockRegex := regexp.MustCompile("(?s)```.*?```")
	text := codeBlockRegex.ReplaceAllString(content, "")

	// Remove inline code
	inlineCodeRegex := regexp.MustCompile("`[^`]+`")
	text = inlineCodeRegex.ReplaceAllString(text, "")

	// Remove images
	imageRegex := regexp.MustCompile(`!\[([^\]]*)\]\(([^\)]+)\)`)
	text = imageRegex.ReplaceAllString(text, "$1")

	// Remove links but keep text
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+)\)`)
	text = linkRegex.ReplaceAllString(text, "$1")

	// Remove headers
	headerRegex := regexp.MustCompile(`(?m)^#{1,6}\s+`)
	text = headerRegex.ReplaceAllString(text, "")

	// Remove bold and italic
	boldItalicRegex := regexp.MustCompile(`[*_]{1,3}([^*_]+)[*_]{1,3}`)
	text = boldItalicRegex.ReplaceAllString(text, "$1")

	// Remove horizontal rules
	hrRegex := regexp.MustCompile(`(?m)^[-*_]{3,}$`)
	text = hrRegex.ReplaceAllString(text, "")

	return strings.TrimSpace(text)
}
