package service

import (
	"context"
	"testing"
)

func TestGeminiLLMParserFallback(t *testing.T) {
	// Test without API key to verify fallback/mock parsing
	parser := NewGeminiLLMParserService("", "gemini-2.5-flash-lite")

	body := "Transaksi Berhasil BCA Transfer ke Rekening 1234 sebesar Rp 50.000"
	res, raw, err := parser.ParseEmailMutation(context.Background(), body)
	if err != nil {
		t.Fatalf("Expected no error from fallback parser, got: %v", err)
	}

	if !res.IsTransaction {
		t.Errorf("Expected IsTransaction=true")
	}

	if res.ProviderCode == nil || *res.ProviderCode != "BCA" {
		t.Errorf("Expected ProviderCode BCA, got %v", res.ProviderCode)
	}

	if raw == "" {
		t.Errorf("Expected non-empty raw JSON string")
	}
}
