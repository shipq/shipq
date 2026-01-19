package inifile

import (
	"bufio"
	"io"
	"os"
	"strings"
)

// File represents a parsed INI file.
type File struct {
	Sections []Section
}

// Section represents a named section in an INI file.
type Section struct {
	Name   string     // e.g., "database", "crud.users"
	Values []KeyValue // preserves order
}

// KeyValue represents a key-value pair.
type KeyValue struct {
	Key   string
	Value string
}

// Parse reads an INI file from the given reader.
func Parse(r io.Reader) (*File, error) {
	f := &File{}
	var currentSection *Section

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.ToLower(strings.Trim(line, "[]"))
			f.Sections = append(f.Sections, Section{Name: name})
			currentSection = &f.Sections[len(f.Sections)-1]
			continue
		}

		// Key-value pair
		if currentSection == nil {
			continue // Ignore keys before any section
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // Ignore lines without =
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		currentSection.Values = append(currentSection.Values, KeyValue{Key: key, Value: value})
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
