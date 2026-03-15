package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSONArrayStreamer incrementally parses a JSON array from streamed text
// deltas, calling onItem for each element as soon as it is complete.
//
// It handles common LLM quirks: markdown code fences (```json ... ```)
// and leading prose before the opening '['.
type JSONArrayStreamer[T any] struct {
	onItem func(T)
	items  []T

	buf          strings.Builder
	foundBracket bool
	depth        int
	inString     bool
	escaped      bool
	elementStart int
	err          error
}

// NewJSONArrayStreamer creates a streamer that calls onItem each time a
// complete top-level array element is parsed from the accumulated text.
func NewJSONArrayStreamer[T any](onItem func(T)) *JSONArrayStreamer[T] {
	return &JSONArrayStreamer[T]{onItem: onItem}
}

// Feed accepts a text delta (as delivered by WithOnText) and processes it
// character by character. Fully-parsed array elements are delivered to the
// onItem callback synchronously before Feed returns.
func (s *JSONArrayStreamer[T]) Feed(delta string) {
	for _, ch := range delta {
		if !s.foundBracket {
			if ch == '[' {
				s.foundBracket = true
				s.depth = 1
				s.elementStart = 0
				s.buf.Reset()
			}
			continue
		}

		s.buf.WriteRune(ch)

		if s.inString {
			if s.escaped {
				s.escaped = false
				continue
			}
			if ch == '\\' {
				s.escaped = true
				continue
			}
			if ch == '"' {
				s.inString = false
			}
			continue
		}

		switch ch {
		case '"':
			s.inString = true

		case '{', '[':
			s.depth++

		case '}', ']':
			s.depth--
			if s.depth == 1 {
				// A nested structure closed (e.g. {} or []) — the element
				// includes the closing character we just wrote.
				s.emitUpTo(s.buf.Len())
			} else if s.depth == 0 {
				// The top-level array closed — exclude the ']' itself.
				s.emitUpTo(s.buf.Len() - 1)
				return
			}

		case ',':
			if s.depth == 1 {
				// Separator between elements — exclude the ','.
				s.emitUpTo(s.buf.Len() - 1)
			}
		}
	}
}

// emitUpTo extracts buf[elementStart:end], trims whitespace, unmarshals
// into T, and delivers via onItem. Advances elementStart for the next element.
func (s *JSONArrayStreamer[T]) emitUpTo(end int) {
	raw := strings.TrimSpace(s.buf.String()[s.elementStart:end])

	s.elementStart = s.buf.Len()

	if raw == "" {
		return
	}

	var item T
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return
	}
	s.items = append(s.items, item)
	if s.onItem != nil {
		s.onItem(item)
	}
}

// All returns every item successfully parsed so far, in order.
func (s *JSONArrayStreamer[T]) All() []T {
	return s.items
}

// Err returns a non-nil error if the stream never contained a '['.
func (s *JSONArrayStreamer[T]) Err() error {
	if s.err != nil {
		return s.err
	}
	if !s.foundBracket {
		return fmt.Errorf("llm: JSONArrayStreamer: no JSON array found in stream")
	}
	return nil
}
