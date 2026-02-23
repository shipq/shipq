package inifile

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestSet(t *testing.T) {
	t.Run("set key in existing section", func(t *testing.T) {
		ini := "[section]\nkey1 = value1\n"
		f, _ := Parse(strings.NewReader(ini))
		f.Set("section", "key2", "value2")
		if got := f.Get("section", "key2"); got != "value2" {
			t.Errorf("got %q, want %q", got, "value2")
		}
		// Original key should still exist
		if got := f.Get("section", "key1"); got != "value1" {
			t.Errorf("got %q, want %q", got, "value1")
		}
	})

	t.Run("set key in new section", func(t *testing.T) {
		ini := "[existing]\nkey = value\n"
		f, _ := Parse(strings.NewReader(ini))
		f.Set("newsection", "newkey", "newvalue")
		if got := f.Get("newsection", "newkey"); got != "newvalue" {
			t.Errorf("got %q, want %q", got, "newvalue")
		}
		// Original section should still exist
		if got := f.Get("existing", "key"); got != "value" {
			t.Errorf("got %q, want %q", got, "value")
		}
	})

	t.Run("overwrite existing key", func(t *testing.T) {
		ini := "[section]\nkey = original\n"
		f, _ := Parse(strings.NewReader(ini))
		f.Set("section", "key", "updated")
		if got := f.Get("section", "key"); got != "updated" {
			t.Errorf("got %q, want %q", got, "updated")
		}
		// Should not duplicate the key
		s := f.Section("section")
		if s == nil {
			t.Fatal("expected section, got nil")
		}
		count := 0
		for _, kv := range s.Values {
			if kv.Key == "key" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected 1 occurrence of key, got %d", count)
		}
	})

	t.Run("set key in empty file", func(t *testing.T) {
		f := &File{}
		f.Set("section", "key", "value")
		if got := f.Get("section", "key"); got != "value" {
			t.Errorf("got %q, want %q", got, "value")
		}
	})

	t.Run("section and key names are normalized to lowercase", func(t *testing.T) {
		f := &File{}
		f.Set("SECTION", "KEY", "value")
		if got := f.Get("section", "key"); got != "value" {
			t.Errorf("got %q, want %q", got, "value")
		}
	})

	t.Run("value preserves case", func(t *testing.T) {
		f := &File{}
		f.Set("section", "key", "MixedCase Value")
		if got := f.Get("section", "key"); got != "MixedCase Value" {
			t.Errorf("got %q, want %q", got, "MixedCase Value")
		}
	})
}

func TestWrite(t *testing.T) {
	t.Run("write empty file", func(t *testing.T) {
		f := &File{}
		var buf bytes.Buffer
		err := f.Write(&buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if buf.String() != "" {
			t.Errorf("expected empty string, got %q", buf.String())
		}
	})

	t.Run("write single section with one key", func(t *testing.T) {
		f := &File{}
		f.Set("section", "key", "value")
		var buf bytes.Buffer
		err := f.Write(&buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "[section]\nkey = value\n"
		if buf.String() != want {
			t.Errorf("got %q, want %q", buf.String(), want)
		}
	})

	t.Run("write multiple sections", func(t *testing.T) {
		f := &File{}
		f.Set("section1", "key1", "value1")
		f.Set("section2", "key2", "value2")
		var buf bytes.Buffer
		err := f.Write(&buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "[section1]\nkey1 = value1\n\n[section2]\nkey2 = value2\n"
		if buf.String() != want {
			t.Errorf("got %q, want %q", buf.String(), want)
		}
	})

	t.Run("write section with multiple keys", func(t *testing.T) {
		f := &File{}
		f.Set("section", "key1", "value1")
		f.Set("section", "key2", "value2")
		var buf bytes.Buffer
		err := f.Write(&buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "[section]\nkey1 = value1\nkey2 = value2\n"
		if buf.String() != want {
			t.Errorf("got %q, want %q", buf.String(), want)
		}
	})
}

func TestWriteFile(t *testing.T) {
	t.Run("write to file", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test.ini")

		f := &File{}
		f.Set("db", "url", "postgres://localhost/test")
		err := f.WriteFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Read back and verify
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		want := "[db]\nurl = postgres://localhost/test\n"
		if string(content) != want {
			t.Errorf("got %q, want %q", string(content), want)
		}
	})
}

func TestRoundTrip(t *testing.T) {
	t.Run("parse and write preserves structure", func(t *testing.T) {
		original := "[database]\nurl = postgres://localhost/db\npool_size = 10\n\n[paths]\nmigrations = ./migrations\n"
		f, err := Parse(strings.NewReader(original))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var buf bytes.Buffer
		err = f.Write(&buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Parse again and compare values
		f2, err := Parse(strings.NewReader(buf.String()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := f2.Get("database", "url"); got != "postgres://localhost/db" {
			t.Errorf("database.url: got %q, want %q", got, "postgres://localhost/db")
		}
		if got := f2.Get("database", "pool_size"); got != "10" {
			t.Errorf("database.pool_size: got %q, want %q", got, "10")
		}
		if got := f2.Get("paths", "migrations"); got != "./migrations" {
			t.Errorf("paths.migrations: got %q, want %q", got, "./migrations")
		}
	})

	t.Run("round-trip with Set modification", func(t *testing.T) {
		original := "[db]\nurl = old_value\n"
		f, _ := Parse(strings.NewReader(original))
		f.Set("db", "url", "new_value")

		var buf bytes.Buffer
		f.Write(&buf)

		f2, _ := Parse(strings.NewReader(buf.String()))
		if got := f2.Get("db", "url"); got != "new_value" {
			t.Errorf("got %q, want %q", got, "new_value")
		}

		// Ensure no duplicate keys after round-trip
		s := f2.Section("db")
		count := 0
		for _, kv := range s.Values {
			if kv.Key == "url" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected 1 occurrence of url, got %d", count)
		}
	})

	t.Run("full file round-trip through WriteFile and ParseFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "roundtrip.ini")

		f := &File{}
		f.Set("section1", "key1", "value1")
		f.Set("section1", "key2", "value2")
		f.Set("section2", "key3", "value3")

		err := f.WriteFile(path)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		f2, err := ParseFile(path)
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		if got := f2.Get("section1", "key1"); got != "value1" {
			t.Errorf("section1.key1: got %q, want %q", got, "value1")
		}
		if got := f2.Get("section1", "key2"); got != "value2" {
			t.Errorf("section1.key2: got %q, want %q", got, "value2")
		}
		if got := f2.Get("section2", "key3"); got != "value3" {
			t.Errorf("section2.key3: got %q, want %q", got, "value3")
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
