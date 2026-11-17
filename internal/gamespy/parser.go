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

	raw = strings.TrimRight(raw, "\x00")
	parts := strings.Split(raw, "\\")
	// Skip leading empty element if starts with '\'
	start := 0
	if len(parts) > 0 && strings.Trim(parts[0], "\x00 \t\r\n") == "" {
		start = 1
	}

	for i := start; i < len(parts); {
		k := strings.ToLower(strings.Trim(parts[i], "\x00 \t\r\n"))
		if k == "" {
			i++
			continue
		}

		v := ""
		if i+1 < len(parts) {
			v = strings.Trim(parts[i+1], "\x00 \t\r\n")
		}

		// Valueless flag tokens in GameSpy protocol: if followed by a non-empty token
		// (single backslash delimiter instead of '\\'), do not consume the next key as a value.
		if isValuelessFlagKey(k) && v != "" {
			m.KV[k] = ""
			i++
			continue
		}

		// Keys with optional values (\list\gamename\ut, \statechanged\gamemode\exiting, \status\final\)
		if isOptionalValueKey(k) && isReservedProtocolKey(strings.ToLower(v)) {
			m.KV[k] = ""
			i++
			continue
		}

		m.KV[k] = v
		i += 2
	}

	return m
}

func isValuelessFlagKey(k string) bool {
	switch k {
	case "final", "basic", "info", "rules", "players", "packets":
		return true
	default:
		return false
	}
}

func isOptionalValueKey(k string) bool {
	switch k {
	case "list", "statechanged", "status":
		return true
	default:
		return false
	}
}

func isReservedProtocolKey(k string) bool {
	switch k {
	case "gamename", "gamemode", "final", "queryid", "validate", "secure", "echo", "echo_reply", "location", "where", "list", "basic", "info", "status":
		return true
	default:
		return false
	}
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
