package gamespy

import (
	"testing"
)

func TestGenerateValidate(t *testing.T) {
	// Kingpin cipher is QFWxY2
	val0 := GenerateValidate("QFWxY2", "TXKOAT", 0)
	if val0 == "" {
		t.Fatalf("expected non-empty validation response")
	}

	val1 := GenerateValidate("Z5Nfb0", "ABCDEF", 1) // UT cipher Z5Nfb0
	if val1 == "" {
		t.Fatalf("expected non-empty validation response for enctype 1")
	}

	val2 := GenerateValidate("Z5Nfb0", "ABCDEF", 2)
	if val2 == "" {
		t.Fatalf("expected non-empty validation response for enctype 2")
	}
}

func TestParseMessage_EdgeCases(t *testing.T) {
	// 1. Null-terminated C packet
	m1 := ParseMessage("\\gamename\\ut\\validate\\Sen5c0WR\x00")
	if m1.Get("gamename") != "ut" || m1.Get("validate") != "Sen5c0WR" {
		t.Fatalf("failed to strip C null terminator: %+v", m1.KV)
	}

	// 2. Coalesced TCP stream with single backslash after \final
	m2 := ParseMessage("\\gamename\\ut\\validate\\Sen5c0WR\\final\\list\\cmp\\gamename\\ut\\final\\")
	if !m2.Has("list") || m2.Get("list") != "cmp" || m2.Get("gamename") != "ut" {
		t.Fatalf("failed to parse coalesced packet after \\final: %+v", m2.KV)
	}

	// 3. Uncompressed \list\gamename\ut\final\ (single slash after \list)
	m3 := ParseMessage("\\list\\gamename\\ut\\final\\")
	if !m3.Has("list") || m3.Get("gamename") != "ut" {
		t.Fatalf("failed to parse uncompressed \\list\\gamename\\ut: %+v", m3.KV)
	}

	// 4. Appendix B response with single slash after \final
	m4 := ParseMessage("\\gamename\\GMTest\\gamever\\1.0\\location\\1\\final\\queryid\\1.1\\validate\\D324c2eA")
	if m4.Get("validate") != "D324c2eA" || m4.Get("queryid") != "1.1" {
		t.Fatalf("failed to parse Appendix B packet: %+v", m4.KV)
	}
}
