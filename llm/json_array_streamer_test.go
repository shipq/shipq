package llm

import (
	"reflect"
	"testing"
)

type testItem struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// ── Happy path: complete array, character-by-character ─────────────────────────

func TestJSONArrayStreamer_HappyPath_CharByChar(t *testing.T) {
	var got []testItem
	s := NewJSONArrayStreamer(func(item testItem) {
		got = append(got, item)
	})

	input := `[{"name":"a","value":1},{"name":"b","value":2},{"name":"c","value":3}]`
	for _, ch := range input {
		s.Feed(string(ch))
	}

	if err := s.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	want := []testItem{
		{Name: "a", Value: 1},
		{Name: "b", Value: 2},
		{Name: "c", Value: 3},
	}
	if !reflect.DeepEqual(s.All(), want) {
		t.Errorf("All: got %+v, want %+v", s.All(), want)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("onItem calls: got %+v, want %+v", got, want)
	}
}

// ── Chunked delivery (realistic LLM deltas) ──────────────────────────────────

func TestJSONArrayStreamer_ChunkedDelivery(t *testing.T) {
	var got []testItem
	s := NewJSONArrayStreamer(func(item testItem) {
		got = append(got, item)
	})

	chunks := []string{
		`[{"name":"al`,
		`pha","val`,
		`ue":10},`,
		`{"name":"beta`,
		`","value":20}`,
		`]`,
	}
	for _, chunk := range chunks {
		s.Feed(chunk)
	}

	if err := s.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	want := []testItem{
		{Name: "alpha", Value: 10},
		{Name: "beta", Value: 20},
	}
	if !reflect.DeepEqual(s.All(), want) {
		t.Errorf("All: got %+v, want %+v", s.All(), want)
	}
}

// ── Markdown fence stripping ──────────────────────────────────────────────────

func TestJSONArrayStreamer_MarkdownFence(t *testing.T) {
	var got []testItem
	s := NewJSONArrayStreamer(func(item testItem) {
		got = append(got, item)
	})

	input := "```json\n[{\"name\":\"x\",\"value\":42}]\n```"
	s.Feed(input)

	if err := s.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	want := []testItem{{Name: "x", Value: 42}}
	if !reflect.DeepEqual(s.All(), want) {
		t.Errorf("All: got %+v, want %+v", s.All(), want)
	}
}

// ── Leading text before '[' ───────────────────────────────────────────────────

func TestJSONArrayStreamer_LeadingText(t *testing.T) {
	var got []testItem
	s := NewJSONArrayStreamer(func(item testItem) {
		got = append(got, item)
	})

	input := "Here are the results:\n[{\"name\":\"r1\",\"value\":1},{\"name\":\"r2\",\"value\":2}]"
	s.Feed(input)

	if err := s.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if len(s.All()) != 2 {
		t.Errorf("expected 2 items, got %d", len(s.All()))
	}
}

// ── Nested objects and arrays ─────────────────────────────────────────────────

type nestedItem struct {
	Name string            `json:"name"`
	Tags []string          `json:"tags"`
	Meta map[string]string `json:"meta"`
}

func TestJSONArrayStreamer_NestedObjectsAndArrays(t *testing.T) {
	var got []nestedItem
	s := NewJSONArrayStreamer(func(item nestedItem) {
		got = append(got, item)
	})

	input := `[{"name":"a","tags":["x","y"],"meta":{"k":"v"}},{"name":"b","tags":[],"meta":{}}]`
	for _, ch := range input {
		s.Feed(string(ch))
	}

	if err := s.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if len(s.All()) != 2 {
		t.Fatalf("expected 2 items, got %d", len(s.All()))
	}
	if s.All()[0].Name != "a" || len(s.All()[0].Tags) != 2 {
		t.Errorf("item[0]: got %+v", s.All()[0])
	}
	if s.All()[0].Meta["k"] != "v" {
		t.Errorf("item[0].Meta: got %+v", s.All()[0].Meta)
	}
}

// ── Strings with special characters (escapes, brackets, braces) ───────────────

func TestJSONArrayStreamer_StringsWithSpecialChars(t *testing.T) {
	type stringItem struct {
		Text string `json:"text"`
	}

	var got []stringItem
	s := NewJSONArrayStreamer(func(item stringItem) {
		got = append(got, item)
	})

	input := `[{"text":"contains {braces} and [brackets]"},{"text":"escaped \"quotes\" and \\backslash"},{"text":"comma, inside"}]`
	for _, ch := range input {
		s.Feed(string(ch))
	}

	if err := s.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if len(s.All()) != 3 {
		t.Fatalf("expected 3 items, got %d: %+v", len(s.All()), s.All())
	}
	if s.All()[0].Text != "contains {braces} and [brackets]" {
		t.Errorf("item[0].Text: got %q", s.All()[0].Text)
	}
	if s.All()[1].Text != `escaped "quotes" and \backslash` {
		t.Errorf("item[1].Text: got %q", s.All()[1].Text)
	}
	if s.All()[2].Text != "comma, inside" {
		t.Errorf("item[2].Text: got %q", s.All()[2].Text)
	}
}

// ── Empty array ───────────────────────────────────────────────────────────────

func TestJSONArrayStreamer_EmptyArray(t *testing.T) {
	var count int
	s := NewJSONArrayStreamer(func(_ testItem) {
		count++
	})

	s.Feed("[]")

	if err := s.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if len(s.All()) != 0 {
		t.Errorf("expected 0 items, got %d", len(s.All()))
	}
	if count != 0 {
		t.Errorf("expected 0 onItem calls, got %d", count)
	}
}

// ── No bracket found ──────────────────────────────────────────────────────────

func TestJSONArrayStreamer_NoBracketFound(t *testing.T) {
	s := NewJSONArrayStreamer(func(_ testItem) {})

	s.Feed("This response has no JSON array at all.")

	if err := s.Err(); err == nil {
		t.Fatal("expected error for missing bracket")
	}
}

// ── Single element ────────────────────────────────────────────────────────────

func TestJSONArrayStreamer_SingleElement(t *testing.T) {
	var got []testItem
	s := NewJSONArrayStreamer(func(item testItem) {
		got = append(got, item)
	})

	s.Feed(`[{"name":"only","value":99}]`)

	if err := s.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if len(s.All()) != 1 {
		t.Fatalf("expected 1 item, got %d", len(s.All()))
	}
	if s.All()[0].Name != "only" || s.All()[0].Value != 99 {
		t.Errorf("item: got %+v", s.All()[0])
	}
}

// ── Partial stream (ends mid-element) ─────────────────────────────────────────

func TestJSONArrayStreamer_PartialStream(t *testing.T) {
	var got []testItem
	s := NewJSONArrayStreamer(func(item testItem) {
		got = append(got, item)
	})

	s.Feed(`[{"name":"complete","value":1},{"name":"incom`)

	if len(s.All()) != 1 {
		t.Fatalf("expected 1 complete item, got %d", len(s.All()))
	}
	if s.All()[0].Name != "complete" {
		t.Errorf("item: got %+v", s.All()[0])
	}
}

// ── Incremental delivery: items arrive as they complete ───────────────────────

func TestJSONArrayStreamer_IncrementalCallbacks(t *testing.T) {
	var callOrder []int
	s := NewJSONArrayStreamer(func(item testItem) {
		callOrder = append(callOrder, item.Value)
	})

	s.Feed(`[{"name":"a","value":1}`)
	if len(callOrder) != 1 || callOrder[0] != 1 {
		t.Fatalf("after first element: got %v, want [1]", callOrder)
	}

	s.Feed(`,{"name":"b","value":2}`)
	if len(callOrder) != 2 || callOrder[1] != 2 {
		t.Fatalf("after second element: got %v, want [1 2]", callOrder)
	}

	s.Feed(`,{"name":"c","value":3}]`)
	if len(callOrder) != 3 || callOrder[2] != 3 {
		t.Fatalf("after third element: got %v, want [1 2 3]", callOrder)
	}
}

// ── Nil onItem callback (just collects) ───────────────────────────────────────

func TestJSONArrayStreamer_NilCallback(t *testing.T) {
	s := NewJSONArrayStreamer[testItem](nil)

	s.Feed(`[{"name":"a","value":1},{"name":"b","value":2}]`)

	if err := s.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if len(s.All()) != 2 {
		t.Errorf("expected 2 items, got %d", len(s.All()))
	}
}

// ── Array of primitive values ─────────────────────────────────────────────────

func TestJSONArrayStreamer_PrimitiveStrings(t *testing.T) {
	var got []string
	s := NewJSONArrayStreamer(func(item string) {
		got = append(got, item)
	})

	s.Feed(`["hello","world","foo"]`)

	if err := s.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	want := []string{"hello", "world", "foo"}
	if !reflect.DeepEqual(s.All(), want) {
		t.Errorf("All: got %+v, want %+v", s.All(), want)
	}
}

func TestJSONArrayStreamer_PrimitiveInts(t *testing.T) {
	var got []int
	s := NewJSONArrayStreamer(func(item int) {
		got = append(got, item)
	})

	s.Feed(`[10, 20, 30]`)

	if err := s.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	want := []int{10, 20, 30}
	if !reflect.DeepEqual(s.All(), want) {
		t.Errorf("All: got %+v, want %+v", s.All(), want)
	}
}

// ── Nested arrays as elements ─────────────────────────────────────────────────

func TestJSONArrayStreamer_NestedArrayElements(t *testing.T) {
	var got [][]int
	s := NewJSONArrayStreamer(func(item []int) {
		got = append(got, item)
	})

	s.Feed(`[[1,2,3],[4,5],[6]]`)

	if err := s.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if len(s.All()) != 3 {
		t.Fatalf("expected 3 items, got %d: %+v", len(s.All()), s.All())
	}
	want0 := []int{1, 2, 3}
	want1 := []int{4, 5}
	want2 := []int{6}
	if !reflect.DeepEqual(s.All()[0], want0) {
		t.Errorf("item[0]: got %+v, want %+v", s.All()[0], want0)
	}
	if !reflect.DeepEqual(s.All()[1], want1) {
		t.Errorf("item[1]: got %+v, want %+v", s.All()[1], want1)
	}
	if !reflect.DeepEqual(s.All()[2], want2) {
		t.Errorf("item[2]: got %+v, want %+v", s.All()[2], want2)
	}
}

// ── Whitespace and newlines in the array ──────────────────────────────────────

func TestJSONArrayStreamer_PrettyPrinted(t *testing.T) {
	var got []testItem
	s := NewJSONArrayStreamer(func(item testItem) {
		got = append(got, item)
	})

	input := `[
  {
    "name": "alpha",
    "value": 1
  },
  {
    "name": "beta",
    "value": 2
  }
]`
	for _, ch := range input {
		s.Feed(string(ch))
	}

	if err := s.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if len(s.All()) != 2 {
		t.Fatalf("expected 2 items, got %d", len(s.All()))
	}
	if s.All()[0].Name != "alpha" || s.All()[1].Name != "beta" {
		t.Errorf("items: got %+v", s.All())
	}
}
