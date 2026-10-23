package service

import (
	"strings"
	"testing"
)

func TestSanitizeEmailBody(t *testing.T) {
	rawHTML := `
		<html>
			<head>
				<style>body { font-family: sans-serif; }</style>
				<script>console.log("malicious");</script>
			</head>
			<body>
				<h1>Transaksi Berhasil</h1>
				<p>Transfer dari rekening 1234567890 ke 9876543210123456</p>
				<div>Jumlah: Rp 50.000,00</div>
				<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==" />
				<br/>
				Terima kasih! &amp; Salam hangat.
			</body>
		</html>
	`

	sanitized := SanitizeEmailBody(rawHTML)

	// 1. Should not contain script or style
	if strings.Contains(sanitized, "sans-serif") || strings.Contains(sanitized, "malicious") {
		t.Errorf("Sanitizer should have stripped script and style content")
	}

	// 2. Should not contain base64 image data
	if strings.Contains(sanitized, "base64") {
		t.Errorf("Sanitizer should have stripped base64 data URIs")
	}

	// 3. Should decode HTML entities
	if strings.Contains(sanitized, "&amp;") || !strings.Contains(sanitized, "&") {
		t.Errorf("Sanitizer should unescape HTML entities like &amp;")
	}

	// 4. Privacy masking:
	// "1234567890" (10 digits) -> "******7890" (6 asterisks + 4 digits)
	if !strings.Contains(sanitized, "******7890") {
		t.Errorf("Account number 1234567890 was not properly masked. Got: %s", sanitized)
	}
	// "9876543210123456" (16 digits) -> "************3456" (12 asterisks + 4 digits)
	if !strings.Contains(sanitized, "************3456") {
		t.Errorf("Account number 9876543210123456 was not properly masked. Got: %s", sanitized)
	}

	// 5. Test character limit truncation
	veryLongHTML := strings.Repeat("A", 2000)
	truncated := SanitizeEmailBody(veryLongHTML)
	if len(truncated) > 1500 {
		t.Errorf("Sanitized output should be at most 1500 characters, got %d", len(truncated))
	}
}
