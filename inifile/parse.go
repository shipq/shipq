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
}

// Write serializes the INI file to the given writer.
func (f *File) Write(w io.Writer) error {
	for i, section := range f.Sections {
		// Write section header
		if _, err := fmt.Fprintf(w, "[%s]\n", section.Name); err != nil {
			return err
		}

		// Write key-value pairs
		for _, kv := range section.Values {
			if _, err := fmt.Fprintf(w, "%s = %s\n", kv.Key, kv.Value); err != nil {
				return err
			}
		}

		// Add blank line between sections (but not after the last one)
		if i < len(f.Sections)-1 {
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
