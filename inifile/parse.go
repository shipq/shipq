package inifile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// File represents a parsed INI file.
type File struct {
	Preamble []string  // comment / blank lines before the first section
	Sections []Section // ordered sections
}

// Section represents a named section in an INI file.
type Section struct {
	Name       string     // e.g., "database", "crud.users"
	Values     []KeyValue // preserves order of key=value pairs
	PreLines   []string   // comment / blank lines that appear before the section header
	IntraLines []Line     // interleaved comments, blanks and kv-pairs inside the section
}

// KeyValue represents a key-value pair.
type KeyValue struct {
	Key   string
	Value string
}

// Line is a single line inside a section body.
// If IsKV is true, KVIndex references the position in the parent Section.Values
// slice. Otherwise, Comment holds the raw line text.
type Line struct {
	IsKV    bool
	KVIndex int    // index into Section.Values (valid only when IsKV is true)
	Comment string // raw comment or blank line (valid only when IsKV is false)
}

// Parse reads an INI file from the given reader.
func Parse(r io.Reader) (*File, error) {
	f := &File{}
	var currentSection *Section
	// pendingLines buffers comment/blank lines so they can be attached to
	// whichever construct follows: a section header (PreLines), a key-value
	// pair (IntraLines of the current section), or end-of-file.
	var pendingLines []string

	// flushPendingAsIntra appends buffered comment/blank lines into the
	// current section's IntraLines and clears the buffer.
	flushPendingAsIntra := func() {
		if currentSection == nil {
			return
		}
		for _, pl := range pendingLines {
			currentSection.IntraLines = append(currentSection.IntraLines, Line{Comment: pl})
		}
		pendingLines = nil
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimSpace(raw)

		// Empty lines and comments -- always buffer.
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			pendingLines = append(pendingLines, raw)
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.ToLower(strings.Trim(line, "[]"))
			sec := Section{Name: name}
			if currentSection == nil {
				f.Preamble = append(f.Preamble, pendingLines...)
			} else {
				sec.PreLines = pendingLines
			}
			pendingLines = nil
			f.Sections = append(f.Sections, sec)
			currentSection = &f.Sections[len(f.Sections)-1]
			continue
		}

		// Key-value pair (or unrecognized line)
		if currentSection == nil {
			pendingLines = append(pendingLines, raw)
			continue
		}

		// Flush any buffered comments/blanks as belonging to this section.
		flushPendingAsIntra()

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			currentSection.IntraLines = append(currentSection.IntraLines, Line{Comment: raw})
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		currentSection.Values = append(currentSection.Values, KeyValue{Key: key, Value: value})
		currentSection.IntraLines = append(currentSection.IntraLines, Line{IsKV: true, KVIndex: len(currentSection.Values) - 1})
	}

	// Trailing buffered lines: attach to last section's IntraLines if one
	// exists, otherwise to the file preamble.
	if currentSection != nil {
		flushPendingAsIntra()
	} else {
		f.Preamble = append(f.Preamble, pendingLines...)
	}

	return f, scanner.Err()
}

// ParseFile reads and parses an INI file from disk.
func ParseFile(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Section returns the section with the given name (case-insensitive).
func (f *File) Section(name string) *Section {
	name = strings.ToLower(name)
	for i := range f.Sections {
		if f.Sections[i].Name == name {
			return &f.Sections[i]
		}
	}
	return nil
}

// Get returns the last value for a key in a section.
func (f *File) Get(section, key string) string {
	s := f.Section(section)
	if s == nil {
		return ""
	}
	return s.Get(key)
}

// GetAll returns all values for a key in a section.
func (f *File) GetAll(section, key string) []string {
	s := f.Section(section)
	if s == nil {
		return nil
	}
	return s.GetAll(key)
}

// SectionsWithPrefix returns sections whose names start with prefix.
func (f *File) SectionsWithPrefix(prefix string) []Section {
	prefix = strings.ToLower(prefix)
	var result []Section
	for _, s := range f.Sections {
		if strings.HasPrefix(s.Name, prefix) {
			result = append(result, s)
		}
	}
	return result
}

// Get returns the last value for a key (case-insensitive).
func (s *Section) Get(key string) string {
	key = strings.ToLower(key)
	var result string
	for _, kv := range s.Values {
		if kv.Key == key {
			result = kv.Value
		}
	}
	return result
}

// GetAll returns all values for a key (case-insensitive).
func (s *Section) GetAll(key string) []string {
	key = strings.ToLower(key)
	var result []string
	for _, kv := range s.Values {
		if kv.Key == key {
			result = append(result, kv.Value)
		}
	}
	return result
}

// HasKey returns true if the section contains the given key.
func (s *Section) HasKey(key string) bool {
	key = strings.ToLower(key)
	for _, kv := range s.Values {
		if kv.Key == key {
			return true
		}
	}
	return false
}

// Set sets a key-value pair in the specified section.
// If the section doesn't exist, it is created.
// If the key already exists, its value is replaced (destructive overwrite).
func (f *File) Set(section, key, value string) {
	section = strings.ToLower(section)
	key = strings.ToLower(key)

	// Find or create the section
	var s *Section
	for i := range f.Sections {
		if f.Sections[i].Name == section {
			s = &f.Sections[i]
			break
		}
	}
	if s == nil {
		f.Sections = append(f.Sections, Section{Name: section})
		s = &f.Sections[len(f.Sections)-1]
	}

	// Find and update existing key, or append new one
	for i := range s.Values {
		if s.Values[i].Key == key {
			s.Values[i].Value = value
			return
		}
	}
	s.Values = append(s.Values, KeyValue{Key: key, Value: value})
	s.IntraLines = append(s.IntraLines, Line{IsKV: true, KVIndex: len(s.Values) - 1})
}

// Write serializes the INI file to the given writer, preserving comments and
// blank lines that were present in the original input.
func (f *File) Write(w io.Writer) error {
	// Write file preamble (comments/blanks before the first section).
	for _, line := range f.Preamble {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}

	for i, section := range f.Sections {
		// Write inter-section comments/blanks that precede this section header.
		for _, line := range section.PreLines {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}

		// Write section header.
		if _, err := fmt.Fprintf(w, "[%s]\n", section.Name); err != nil {
			return err
		}

		if len(section.IntraLines) > 0 {
			// Use IntraLines to preserve original ordering of comments and kv pairs.
			for _, l := range section.IntraLines {
				if l.IsKV {
					kv := section.Values[l.KVIndex]
					if _, err := fmt.Fprintf(w, "%s = %s\n", kv.Key, kv.Value); err != nil {
						return err
					}
				} else {
					if _, err := fmt.Fprintln(w, l.Comment); err != nil {
						return err
					}
				}
			}
		} else {
			// Fallback for sections built programmatically (no IntraLines).
			for _, kv := range section.Values {
				if _, err := fmt.Fprintf(w, "%s = %s\n", kv.Key, kv.Value); err != nil {
					return err
				}
			}
		}

		// Add blank line between sections (but not after the last one) when
		// there are no PreLines on the next section to provide spacing.
		if i < len(f.Sections)-1 && len(f.Sections[i+1].PreLines) == 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteFile writes the INI file to the specified path.
func (f *File) WriteFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := f.Write(file); err != nil {
		return err
	}

	return file.Sync()
}
