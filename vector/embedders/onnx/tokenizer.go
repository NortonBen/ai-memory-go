// Package onnx - Tokenizer for Harrier-OSS-v1-270m (Gemma3 / SentencePiece BPE)
// Loads tokenizer.json (HuggingFace format) and encodes text to token IDs.
package onnx

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"unicode/utf8"
)

const (
	// Harrier/Gemma3 special token IDs
	harrierBOSID   = 2    // <bos>
	harrierEOSID   = 1    // <eos>
	harrierPadID   = 0    // <pad>
	harrierUnkID   = 3    // <unk>
	harrierMaxLen  = 8192 // practical cap per chunk
)

// tokenizerJSON is the top-level structure of a HuggingFace tokenizer.json file.
// The merges field can be either:
//   - []string          (e.g. "ab c" — older HF format)
//   - [][]string        (e.g. ["ab", "c"] — newer Gemma3 format)
//
// We unmarshal into json.RawMessage and handle both.
type tokenizerJSON struct {
	Model struct {
		Type   string            `json:"type"`
		Vocab  map[string]int    `json:"vocab"`
		Merges json.RawMessage   `json:"merges"`
	} `json:"model"`
	AddedTokens []struct {
		ID      int    `json:"id"`
		Content string `json:"content"`
		Special bool   `json:"special"`
	} `json:"added_tokens"`
}

// Tokenizer encodes text into token IDs for Harrier/Gemma3.
type Tokenizer struct {
	vocab        map[string]int
	merges       map[bpePair]int // merge priority (lower = higher priority)
	reverseVocab map[int]string
	addedTokens  map[string]int
}

type bpePair struct{ a, b string }

// NewTokenizerFromFile loads a HuggingFace tokenizer.json and builds the BPE tokenizer.
func NewTokenizerFromFile(path string) (*Tokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tokenizer: read file: %w", err)
	}
	return NewTokenizerFromJSON(data)
}

// NewTokenizerFromJSON builds a Tokenizer from raw tokenizer.json bytes.
func NewTokenizerFromJSON(data []byte) (*Tokenizer, error) {
	var tj tokenizerJSON
	if err := json.Unmarshal(data, &tj); err != nil {
		return nil, fmt.Errorf("tokenizer: parse json: %w", err)
	}

	merges, err := parseMerges(tj.Model.Merges)
	if err != nil {
		return nil, fmt.Errorf("tokenizer: parse merges: %w", err)
	}

	mergeMap := make(map[bpePair]int, len(merges))
	for i, parts := range merges {
		mergeMap[bpePair{parts[0], parts[1]}] = i
	}

	rev := make(map[int]string, len(tj.Model.Vocab))
	for tok, id := range tj.Model.Vocab {
		rev[id] = tok
	}

	added := make(map[string]int)
	for _, at := range tj.AddedTokens {
		added[at.Content] = at.ID
		tj.Model.Vocab[at.Content] = at.ID
		rev[at.ID] = at.Content
	}

	return &Tokenizer{
		vocab:        tj.Model.Vocab,
		merges:       mergeMap,
		reverseVocab: rev,
		addedTokens:  added,
	}, nil
}

// parseMerges handles both HuggingFace merge formats:
//   - []string    → each entry is "a b" (space-separated pair)
//   - [][]string  → each entry is ["a", "b"] (2-element array, Gemma3 style)
func parseMerges(raw json.RawMessage) ([][2]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	// Try [][]string first (Gemma3 / newer HF format)
	var arr2 [][]string
	if err := json.Unmarshal(raw, &arr2); err == nil {
		result := make([][2]string, 0, len(arr2))
		for _, pair := range arr2 {
			if len(pair) == 2 {
				result = append(result, [2]string{pair[0], pair[1]})
			}
		}
		return result, nil
	}

	// Fall back to []string ("a b" format)
	var strArr []string
	if err := json.Unmarshal(raw, &strArr); err != nil {
		return nil, fmt.Errorf("cannot parse merges as [][]string or []string")
	}
	result := make([][2]string, 0, len(strArr))
	for _, s := range strArr {
		parts := strings.SplitN(s, " ", 2)
		if len(parts) == 2 {
			result = append(result, [2]string{parts[0], parts[1]})
		}
	}
	return result, nil
}

// Encode tokenizes text and returns token IDs with optional BOS/EOS.
// For document side: no instruction prefix.
// For query side:    prepend the instruction via Harrier prompt format before calling.
func (t *Tokenizer) Encode(text string, addBOS, addEOS bool) []int32 {
	ids := t.bpeEncode(text)

	result := make([]int32, 0, len(ids)+2)
	if addBOS {
		result = append(result, int32(harrierBOSID))
	}
	for _, id := range ids {
		result = append(result, int32(id))
	}
	if addEOS {
		result = append(result, int32(harrierEOSID))
	}
	return result
}

// bpeEncode applies byte-pair encoding to the text.
func (t *Tokenizer) bpeEncode(text string) []int {
	// 1. Pre-tokenise: split on whitespace boundaries and byte-encode each piece
	words := preTokenize(text)

	var out []int
	for _, word := range words {
		chars := splitToChars(word)
		merged := t.applyBPE(chars)
		for _, tok := range merged {
			if id, ok := t.vocab[tok]; ok {
				out = append(out, id)
			} else {
				// byte fallback
				for _, b := range []byte(tok) {
					byteStr := byteToHex(b)
					if id2, ok2 := t.vocab[byteStr]; ok2 {
						out = append(out, id2)
					} else {
						out = append(out, harrierUnkID)
					}
				}
			}
		}
	}
	return out
}

// applyBPE greedily merges token pairs according to learned merge rules.
func (t *Tokenizer) applyBPE(symbols []string) []string {
	if len(symbols) <= 1 {
		return symbols
	}

	for {
		bestPri := math.MaxInt64
		bestIdx := -1
		for i := 0; i < len(symbols)-1; i++ {
			p := bpePair{symbols[i], symbols[i+1]}
			if pri, ok := t.merges[p]; ok && pri < bestPri {
				bestPri = pri
				bestIdx = i
			}
		}
		if bestIdx == -1 {
			break
		}
		merged := symbols[bestIdx] + symbols[bestIdx+1]
		newSym := make([]string, 0, len(symbols)-1)
		newSym = append(newSym, symbols[:bestIdx]...)
		newSym = append(newSym, merged)
		newSym = append(newSym, symbols[bestIdx+2:]...)
		symbols = newSym
	}
	return symbols
}

// preTokenize splits text at word boundaries (space-prefixed GPT-style).
func preTokenize(text string) []string {
	var words []string
	var buf strings.Builder

	runes := []rune(text)
	for i, r := range runes {
		if r == ' ' && i > 0 {
			if buf.Len() > 0 {
				words = append(words, buf.String())
				buf.Reset()
			}
			buf.WriteRune('▁') // SentencePiece space symbol
		} else if r == '\n' || r == '\t' {
			if buf.Len() > 0 {
				words = append(words, buf.String())
				buf.Reset()
			}
			if r == '\n' {
				buf.WriteRune('▁')
			}
		} else {
			buf.WriteRune(r)
		}
		_ = utf8.RuneLen(r)
	}
	if buf.Len() > 0 {
		words = append(words, buf.String())
	}
	return words
}

// splitToChars splits a word into individual UTF-8 characters for BPE.
func splitToChars(word string) []string {
	chars := make([]string, 0, len(word))
	for _, r := range word {
		chars = append(chars, string(r))
	}
	return chars
}

// byteToHex returns the HuggingFace byte-level vocab key for a byte (e.g. "<0xE2>").
func byteToHex(b byte) string {
	return fmt.Sprintf("<0x%02X>", b)
}

// BuildAttentionMask returns a mask of 1s for real tokens and 0s for padding.
func BuildAttentionMask(ids []int32, maxLen int) (paddedIDs []int32, mask []int32) {
	seqLen := len(ids)
	if seqLen > maxLen {
		ids = ids[:maxLen]
		seqLen = maxLen
	}

	paddedIDs = make([]int32, maxLen)
	mask = make([]int32, maxLen)
	copy(paddedIDs, ids)
	for i := 0; i < seqLen; i++ {
		mask[i] = 1
	}
	for i := seqLen; i < maxLen; i++ {
		paddedIDs[i] = int32(harrierPadID)
		mask[i] = 0
	}
	return paddedIDs, mask
}

// FormatQueryInstruct wraps a query with the Harrier instruction prefix
// (required on query side for retrieval tasks).
func FormatQueryInstruct(task, query string) string {
	if task == "" {
		task = "Retrieve semantically similar text"
	}
	return "Instruct: " + task + "\nQuery: " + query
}
