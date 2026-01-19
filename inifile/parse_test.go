package inifile

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	t.Run("empty file", func(t *testing.T) {
		f, err := Parse(strings.NewReader(""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(f.Sections) != 0 {
			t.Errorf("expected empty sections, got %d", len(f.Sections))
		}
	})

	t.Run("single section with one key", func(t *testing.T) {
		ini := "[database]\nurl = postgres://localhost/db\n"
		f, err := Parse(strings.NewReader(ini))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := f.Get("database", "url"); got != "postgres://localhost/db" {
			t.Errorf("got %q, want %q", got, "postgres://localhost/db")
		}
	})

	t.Run("multiple sections", func(t *testing.T) {
		ini := "[database]\nurl = x\n[paths]\nmigrations = m\n"
		f, err := Parse(strings.NewReader(ini))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := f.Get("database", "url"); got != "x" {
			t.Errorf("database.url: got %q, want %q", got, "x")
		}
		if got := f.Get("paths", "migrations"); got != "m" {
			t.Errorf("paths.migrations: got %q, want %q", got, "m")
		}
	})

	t.Run("ignores hash comments", func(t *testing.T) {
		ini := "# comment\n[section]\nkey = value\n"
		f, err := Parse(strings.NewReader(ini))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := f.Get("section", "key"); got != "value" {
			t.Errorf("got %q, want %q", got, "value")
		}
	})

	t.Run("ignores semicolon comments", func(t *testing.T) {
		ini := "; comment\n[section]\nkey = value\n"
		f, err := Parse(strings.NewReader(ini))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := f.Get("section", "key"); got != "value" {
			t.Errorf("got %q, want %q", got, "value")
		}
	})

	t.Run("ignores empty lines", func(t *testing.T) {
		ini := "[section]\n\n\nkey = value\n\n"
		f, err := Parse(strings.NewReader(ini))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := f.Get("section", "key"); got != "value" {
			t.Errorf("got %q, want %q", got, "value")
		}
	})

	t.Run("trims whitespace from keys and values", func(t *testing.T) {
		ini := "[section]\n  key  =   value with spaces   \n"
		f, err := Parse(strings.NewReader(ini))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := f.Get("section", "key"); got != "value with spaces" {
			t.Errorf("got %q, want %q", got, "value with spaces")
		}
	})

	t.Run("handles values with equals signs", func(t *testing.T) {
		ini := "[section]\nurl = postgres://host?foo=bar&baz=qux\n"
		f, err := Parse(strings.NewReader(ini))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := f.Get("section", "url"); got != "postgres://host?foo=bar&baz=qux" {
			t.Errorf("got %q, want %q", got, "postgres://host?foo=bar&baz=qux")
		}
	})

	t.Run("returns empty string for missing key", func(t *testing.T) {
		ini := "[section]\nkey = value\n"
		f, err := Parse(strings.NewReader(ini))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := f.Get("section", "missing"); got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("returns empty string for missing section", func(t *testing.T) {
		ini := "[section]\nkey = value\n"
		f, err := Parse(strings.NewReader(ini))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := f.Get("other", "key"); got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})
}

func TestSection(t *testing.T) {
	t.Run("returns nil for missing section", func(t *testing.T) {
		f, _ := Parse(strings.NewReader("[a]\nk=v\n"))
		if s := f.Section("b"); s != nil {
			t.Errorf("expected nil, got %v", s)
		}
	})

	t.Run("returns section by name", func(t *testing.T) {
		f, _ := Parse(strings.NewReader("[mysection]\nk=v\n"))
		s := f.Section("mysection")
		if s == nil {
			t.Fatal("expected section, got nil")
		}
		if got := s.Get("k"); got != "v" {
			t.Errorf("got %q, want %q", got, "v")
		}
	})
}

func TestSectionsWithPrefix(t *testing.T) {
	t.Run("finds all matching sections", func(t *testing.T) {
		ini := "[crud]\nscope=org_id\n[crud.users]\nscope=\n[crud.posts]\norder=asc\n[other]\n"
		f, _ := Parse(strings.NewReader(ini))

		sections := f.SectionsWithPrefix("crud.")
		if len(sections) != 2 {
			t.Fatalf("expected 2 sections, got %d", len(sections))
		}

		names := []string{sections[0].Name, sections[1].Name}
		hasUsers := false
		hasPosts := false
		for _, n := range names {
			if n == "crud.users" {
				hasUsers = true
			}
			if n == "crud.posts" {
				hasPosts = true
			}
		}
		if !hasUsers {
			t.Error("expected to find crud.users section")
		}
		if !hasPosts {
			t.Error("expected to find crud.posts section")
		}
	})

	t.Run("returns empty slice if no matches", func(t *testing.T) {
		f, _ := Parse(strings.NewReader("[section]\nk=v\n"))
		sections := f.SectionsWithPrefix("crud.")
		if len(sections) != 0 {
			t.Errorf("expected empty slice, got %d sections", len(sections))
		}
	})
}

func TestCaseSensitivity(t *testing.T) {
	t.Run("section names are case-insensitive", func(t *testing.T) {
		ini := "[DATABASE]\nurl = x\n"
		f, _ := Parse(strings.NewReader(ini))
		if got := f.Get("database", "url"); got != "x" {
			t.Errorf("lowercase lookup: got %q, want %q", got, "x")
		}
		if got := f.Get("DATABASE", "url"); got != "x" {
			t.Errorf("uppercase lookup: got %q, want %q", got, "x")
		}
	})

	t.Run("key names are case-insensitive", func(t *testing.T) {
		ini := "[section]\nURL = x\n"
		f, _ := Parse(strings.NewReader(ini))
		if got := f.Get("section", "url"); got != "x" {
			t.Errorf("lowercase lookup: got %q, want %q", got, "x")
		}
		if got := f.Get("section", "URL"); got != "x" {
			t.Errorf("uppercase lookup: got %q, want %q", got, "x")
		}
	})

	t.Run("values preserve case", func(t *testing.T) {
		ini := "[section]\nkey = MixedCase Value\n"
		f, _ := Parse(strings.NewReader(ini))
		if got := f.Get("section", "key"); got != "MixedCase Value" {
			t.Errorf("got %q, want %q", got, "MixedCase Value")
		}
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("keys before any section are ignored", func(t *testing.T) {
		ini := "orphan = value\n[section]\nkey = v\n"
		f, _ := Parse(strings.NewReader(ini))
		// orphan key should be ignored (no global section)
		if got := f.Get("", "orphan"); got != "" {
			t.Errorf("orphan key: got %q, want empty string", got)
		}
		if got := f.Get("section", "key"); got != "v" {
			t.Errorf("section key: got %q, want %q", got, "v")
		}
	})

	t.Run("duplicate keys keep last value", func(t *testing.T) {
		ini := "[section]\nkey = first\nkey = second\n"
		f, _ := Parse(strings.NewReader(ini))
		if got := f.Get("section", "key"); got != "second" {
			t.Errorf("got %q, want %q", got, "second")
		}
	})

	t.Run("GetAll returns all values for duplicate keys", func(t *testing.T) {
		ini := "[section]\nkey = first\nkey = second\n"
		f, _ := Parse(strings.NewReader(ini))
		got := f.GetAll("section", "key")
		want := []string{"first", "second"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("empty value is valid", func(t *testing.T) {
		ini := "[section]\nkey = \n"
		f, _ := Parse(strings.NewReader(ini))
		if got := f.Get("section", "key"); got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("line without equals is ignored", func(t *testing.T) {
		ini := "[section]\ninvalid line\nkey = value\n"
		f, _ := Parse(strings.NewReader(ini))
		if got := f.Get("section", "key"); got != "value" {
			t.Errorf("got %q, want %q", got, "value")
		}
	})
}

func TestSectionHasKey(t *testing.T) {
	t.Run("returns true for existing key", func(t *testing.T) {
		ini := "[section]\nkey = value\n"
		f, _ := Parse(strings.NewReader(ini))
		s := f.Section("section")
		if s == nil {
			t.Fatal("expected section, got nil")
		}
		if !s.HasKey("key") {
			t.Error("expected HasKey to return true")
		}
	})

	t.Run("returns true for key with empty value", func(t *testing.T) {
		ini := "[section]\nkey = \n"
		f, _ := Parse(strings.NewReader(ini))
		s := f.Section("section")
		if s == nil {
			t.Fatal("expected section, got nil")
		}
		if !s.HasKey("key") {
			t.Error("expected HasKey to return true for key with empty value")
		}
	})

	t.Run("returns false for missing key", func(t *testing.T) {
		ini := "[section]\nkey = value\n"
		f, _ := Parse(strings.NewReader(ini))
		s := f.Section("section")
		if s == nil {
			t.Fatal("expected section, got nil")
		}
		if s.HasKey("missing") {
			t.Error("expected HasKey to return false for missing key")
		}
	})

	t.Run("is case-insensitive", func(t *testing.T) {
		ini := "[section]\nMyKey = value\n"
		f, _ := Parse(strings.NewReader(ini))
		s := f.Section("section")
		if s == nil {
			t.Fatal("expected section, got nil")
		}
		if !s.HasKey("mykey") {
			t.Error("expected HasKey('mykey') to return true")
		}
		if !s.HasKey("MYKEY") {
			t.Error("expected HasKey('MYKEY') to return true")
		}
	})
}

func TestSectionGetAll(t *testing.T) {
	t.Run("returns nil for missing key", func(t *testing.T) {
		ini := "[section]\nkey = value\n"
		f, _ := Parse(strings.NewReader(ini))
		s := f.Section("section")
		if s == nil {
			t.Fatal("expected section, got nil")
		}
		if got := s.GetAll("missing"); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("returns all values in order", func(t *testing.T) {
		ini := "[section]\nkey = first\nkey = second\nkey = third\n"
		f, _ := Parse(strings.NewReader(ini))
		s := f.Section("section")
		if s == nil {
			t.Fatal("expected section, got nil")
		}
		got := s.GetAll("key")
		want := []string{"first", "second", "third"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestFileGetAll(t *testing.T) {
	t.Run("returns nil for missing section", func(t *testing.T) {
		ini := "[section]\nkey = value\n"
		f, _ := Parse(strings.NewReader(ini))
		if got := f.GetAll("missing", "key"); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}
