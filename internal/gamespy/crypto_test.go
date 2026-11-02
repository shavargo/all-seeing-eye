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
