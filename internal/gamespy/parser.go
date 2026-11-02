package gamespy

import (
	"fmt"
	"strings"
)

// Message represents a parsed GameSpy \key\value\ payload.
type Message struct {
	KV map[string]string
}

// ParseMessage parses a raw GameSpy delimited string into key-value pairs.
func ParseMessage(raw string) *Message {
	m := &Message{
		KV: make(map[string]string),
	}

	parts := strings.Split(raw, "\\")
	// Skip leading empty element if starts with '\'
	start := 0
	if len(parts) > 0 && parts[0] == "" {
		start = 1
	}

	for i := start; i+1 < len(parts); i += 2 {
		k := strings.ToLower(strings.TrimSpace(parts[i]))
		v := strings.TrimSpace(parts[i+1])
		if k != "" {
			m.KV[k] = v
		}
	}

	return m
}

// Get returns the value for key or empty string.
func (m *Message) Get(key string) string {
	return m.KV[strings.ToLower(key)]
}

// Has checks if key exists.
func (m *Message) Has(key string) bool {
	_, ok := m.KV[strings.ToLower(key)]
	return ok
}

// BuildKeyValue constructs a raw \key\value string from pairs.
func BuildKeyValue(pairs ...string) string {
	var sb strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		sb.WriteString(fmt.Sprintf("\\%s\\%s", pairs[i], pairs[i+1]))
	}
	return sb.String()
}
