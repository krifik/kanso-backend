package parser_test

import (
	"testing"
	"kanso/backend/internal/infrastructure/parser"
)

func TestParseBankReceipt_JagoQRIS(t *testing.T) {
	text := "Bank Jago: Pembayaran QRIS ke Kopi Kenangan sebesar Rp 45.000 berhasil pada 23/10/2026."
	res := parser.ParseBankReceipt(text)

	if res.BankOrSource != "Bank Jago" {
		t.Errorf("Expected Bank Jago, got %s", res.BankOrSource)
	}
	if res.Amount != 45000 {
		t.Errorf("Expected 45000, got %f", res.Amount)
	}
	if res.Category != "Ritual Kopi & Kafe" {
		t.Errorf("Expected Ritual Kopi & Kafe, got %s", res.Category)
	}
}

func TestParseBankReceipt_BCA(t *testing.T) {
	text := "m-BCA: Transaksi di Supermarket Indomaret sebesar Rp 125.000 berhasil."
	res := parser.ParseBankReceipt(text)

	if res.BankOrSource != "BCA" {
		t.Errorf("Expected BCA, got %s", res.BankOrSource)
	}
	if res.Amount != 125000 {
		t.Errorf("Expected 125000, got %f", res.Amount)
	}
	if res.Category != "Bahan Dapur & Sayur Segar" {
		t.Errorf("Expected Bahan Dapur & Sayur Segar, got %s", res.Category)
	}
}
