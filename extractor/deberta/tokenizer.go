// Package deberta implements SentencePiece Unigram tokenization for DeBERTa-v3 models.
// It loads HuggingFace tokenizer.json files and produces token IDs and word-alignment maps
// for use with NER ONNX models.
package deberta

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

var negInf = math.Inf(-1)

const spmSpace = '▁' // U+2581 — SentencePiece word-boundary marker

// tokenizerJSON mirrors the fields of a HuggingFace tokenizer.json for
// Unigram (SentencePiece) models such as DeBERTa-v3.
type tokenizerJSON struct {
	Model struct {
		Type         string              `json:"type"`
		Vocab        [][]json.RawMessage `json:"vocab"` // [[token_str, score_float64], ...]
		UnkID        int                 `json:"unk_id"`
		ByteFallback bool                `json:"byte_fallback"`
	} `json:"model"`
	AddedTokens []struct {
		ID      int    `json:"id"`
		Content string `json:"content"`
		Special bool   `json:"special"`
	} `json:"added_tokens"`
}

// vocabEntry holds one SentencePiece vocabulary item.
type vocabEntry struct {
	token string
	runes []rune  // pre-computed for Viterbi comparison
	score float64 // log-probability from SPM model
	id    int
}

// Tokenizer implements SentencePiece Unigram tokenisation for DeBERTa-v3.
// It is safe to use from multiple goroutines (read-only after construction).
type Tokenizer struct {
	vocabByID  []vocabEntry           // indexed by token ID
	byFirst    map[rune][]*vocabEntry // candidates grouped by first rune (Viterbi speedup)
	byByte     map[byte]int           // byte-fallback: byte value → token ID of "<0xXX>"
	maxTokLen  int                    // max token length in runes

	ClsID int
	SepID int
	PadID int
	UnkID int
}

// NewTokenizerFromFile loads a HuggingFace tokenizer.json that contains a
// SentencePiece Unigram model (DeBERTa-v3, ALBERT, mDeBERTa, etc.).
func NewTokenizerFromFile(path string) (*Tokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("deberta tokenizer: read %q: %w", path, err)
	}
	return parseTokenizerJSON(data)
}

func parseTokenizerJSON(data []byte) (*Tokenizer, error) {
	var tj tokenizerJSON
	if err := json.Unmarshal(data, &tj); err != nil {
		return nil, fmt.Errorf("deberta tokenizer: unmarshal: %w", err)
	}
	if tj.Model.Type != "Unigram" {
		return nil, fmt.Errorf("deberta tokenizer: expected Unigram model, got %q (only SentencePiece Unigram is supported)", tj.Model.Type)
	}

	// Build vocab slice (ID order)
	entries := make([]vocabEntry, len(tj.Model.Vocab))
	byFirst := make(map[rune][]*vocabEntry)
	byByte := make(map[byte]int)
	maxLen := 0

	for i, pair := range tj.Model.Vocab {
		if len(pair) < 2 {
			continue
		}
		var tok string
		if err := json.Unmarshal(pair[0], &tok); err != nil {
			return nil, fmt.Errorf("deberta tokenizer: vocab[%d] token: %w", i, err)
		}
		var score float64
		if err := json.Unmarshal(pair[1], &score); err != nil {
			return nil, fmt.Errorf("deberta tokenizer: vocab[%d] score: %w", i, err)
		}
		runes := []rune(tok)
		entries[i] = vocabEntry{token: tok, runes: runes, score: score, id: i}

		if len(runes) > 0 {
			byFirst[runes[0]] = append(byFirst[runes[0]], &entries[i])
		}
		if len(runes) > maxLen {
			maxLen = len(runes)
		}

		// Detect byte-fallback tokens: <0xXX>
		if len(tok) == 6 && strings.HasPrefix(tok, "<0x") && strings.HasSuffix(tok, ">") {
			var b byte
			if n, _ := fmt.Sscanf(tok[1:5], "0x%02X", &b); n == 1 {
				byByte[b] = i
			}
		}
	}

	t := &Tokenizer{
		vocabByID: entries,
		byFirst:   byFirst,
		byByte:    byByte,
		maxTokLen: maxLen,
		UnkID:     tj.Model.UnkID,
	}

	// Override special tokens from added_tokens section
	// Default IDs (common for DeBERTa-v3): [PAD]=0, [CLS]=1, [SEP]=2, [UNK]=unk_id
	t.PadID = 0
	t.ClsID = 1
	t.SepID = 2
	for _, at := range tj.AddedTokens {
		switch at.Content {
		case "[CLS]", "<s>":
			t.ClsID = at.ID
		case "[SEP]", "</s>":
			t.SepID = at.ID
		case "[PAD]", "<pad>":
			t.PadID = at.ID
		case "[UNK]", "<unk>":
			t.UnkID = at.ID
		}
	}

	return t, nil
}

// Encode tokenizes text and returns (tokenIDs, wordIDs).
//
//   - tokenIDs: slice of token IDs with optional [CLS] and [SEP] added.
//   - wordIDs:  parallel slice mapping each token to its original word index
//     (0-based). Special tokens ([CLS], [SEP]) are mapped to -1.
//
// Word indices are determined by the ▁ prefix (SentencePiece word boundary).
func (t *Tokenizer) Encode(text string, addCLS, addSEP bool) (tokenIDs []int32, wordIDs []int) {
	normalized := normalizeSPM(text)
	rawIDs := t.viterbi(normalized)

	// CLS
	if addCLS {
		tokenIDs = append(tokenIDs, int32(t.ClsID))
		wordIDs = append(wordIDs, -1)
	}

	// Content tokens with word alignment
	wordIdx := -1
	for _, id := range rawIDs {
		var tok string
		if id >= 0 && id < len(t.vocabByID) {
			tok = t.vocabByID[id].token
		}
		// New word: token starts with ▁ (or it's the very first content token)
		if wordIdx == -1 || strings.HasPrefix(tok, string(spmSpace)) {
			wordIdx++
		}
		tokenIDs = append(tokenIDs, int32(id))
		wordIDs = append(wordIDs, wordIdx)
	}

	// SEP
	if addSEP {
		tokenIDs = append(tokenIDs, int32(t.SepID))
		wordIDs = append(wordIDs, -1)
	}

	return tokenIDs, wordIDs
}

// BuildAttentionMask pads tokenIDs to maxLen and returns (paddedIDs, mask).
// Positions beyond the real tokens are filled with PadID and mask=0.
func (t *Tokenizer) BuildAttentionMask(ids []int32, wordIDs []int, maxLen int) (
	paddedIDs []int32,
	paddedWordIDs []int,
	mask []int32,
) {
	seqLen := len(ids)
	if seqLen > maxLen {
		ids = ids[:maxLen]
		wordIDs = wordIDs[:maxLen]
		seqLen = maxLen
	}

	paddedIDs = make([]int32, maxLen)
	paddedWordIDs = make([]int, maxLen)
	mask = make([]int32, maxLen)

	copy(paddedIDs, ids)
	copy(paddedWordIDs, wordIDs)
	for i := 0; i < seqLen; i++ {
		mask[i] = 1
	}
	for i := seqLen; i < maxLen; i++ {
		paddedIDs[i] = int32(t.PadID)
		paddedWordIDs[i] = -1
		mask[i] = 0
	}
	return
}

// ─── Viterbi (SentencePiece Unigram) ──────────────────────────────────────────

// viterbi implements the Viterbi algorithm for SentencePiece Unigram decoding.
// It returns the maximum-score tokenisation of the (already normalized) rune slice.
func (t *Tokenizer) viterbi(runes []rune) []int {
	n := len(runes)
	if n == 0 {
		return nil
	}

	// dp[i] = best log-score for the first i runes
	dp := make([]float64, n+1)
	// back[i] = {start, tokenID} for the token ending at position i
	type bpEntry struct{ start, id int }
	bp := make([]bpEntry, n+1)

	dp[0] = 0
	for i := 1; i <= n; i++ {
		dp[i] = negInf
		bp[i] = bpEntry{-1, t.UnkID}
	}

	for end := 1; end <= n; end++ {
		// Iterate over possible start positions j
		for j := max0(end-t.maxTokLen, 0); j < end; j++ {
			fragment := runes[j:end]
			candidates := t.byFirst[fragment[0]]
			for _, e := range candidates {
				if len(e.runes) != len(fragment) {
					continue
				}
				if !runesEq(e.runes, fragment) {
					continue
				}
				score := dp[j] + e.score
				if score > dp[end] {
					dp[end] = score
					bp[end] = bpEntry{j, e.id}
				}
			}
		}

		// Byte fallback: consume exactly one rune if no token covers runes[end-1]
		if math.IsInf(dp[end], -1) {
			j := end - 1
			ch := runes[j]
			bestByteScore := negInf
			bestByteID := t.UnkID

			for _, b := range []byte(string(ch)) {
				if tokID, ok := t.byByte[b]; ok {
					s := dp[j] + t.vocabByID[tokID].score
					if s > bestByteScore {
						bestByteScore = s
						bestByteID = tokID
					}
				}
			}
			if !math.IsInf(bestByteScore, -1) {
				dp[end] = bestByteScore
				bp[end] = bpEntry{j, bestByteID}
			} else {
				// Hard unknown: advance one rune with large penalty
				dp[end] = dp[j] - 100.0
				bp[end] = bpEntry{j, t.UnkID}
			}
		}
	}

	// Backtrack
	var ids []int
	pos := n
	for pos > 0 {
		entry := bp[pos]
		ids = append(ids, entry.id)
		pos = entry.start
	}
	// Reverse
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
	return ids
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// normalizeSPM converts text into the SentencePiece surface form:
// all whitespace → ▁, then ▁ prepended at the start.
func normalizeSPM(text string) []rune {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	runes := []rune(text)
	out := make([]rune, 0, len(runes)+1)
	out = append(out, spmSpace) // leading word marker

	for _, r := range runes {
		switch r {
		case ' ', '\t', '\n', '\r':
			out = append(out, spmSpace)
		default:
			out = append(out, r)
		}
	}

	// Collapse consecutive ▁▁ into single ▁
	compact := out[:0]
	prev := rune(0)
	for _, r := range out {
		if r == spmSpace && prev == spmSpace {
			continue
		}
		compact = append(compact, r)
		prev = r
	}
	return compact
}

func runesEq(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func max0(a, b int) int {
	if a > b {
		return a
	}
	return b
}
