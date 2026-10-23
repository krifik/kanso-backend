package parser

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ParsedReceipt struct {
	BankOrSource string    `json:"bank_or_source"`
	Merchant     string    `json:"merchant"`
	Amount       float64   `json:"amount"`
	Description  string    `json:"description"`
	Category     string    `json:"category"`
	SuggestedPod string    `json:"suggested_pod"`
	TransactedAt time.Time `json:"transacted_at"`
	IsRecognized bool      `json:"is_recognized"`
}

var (
	// Clean numbers like "Rp 150.000", "Rp150,000.00", "IDR 50.000", "Rp. 45.000"
	amountRegex = regexp.MustCompile(`(?i)(?:rp\.?|idr)\s*([\d\.,]+)`)

	// Bank identifiers
	jagoRegex     = regexp.MustCompile(`(?i)(?:bank\s+jago|kantong\s+jago|jago)`)
	bcaRegex      = regexp.MustCompile(`(?i)(?:m-bca|klikbca|bca|bank\s+central\s+asia)`)
	permataRegex  = regexp.MustCompile(`(?i)(?:permatabank|permata\s*me|permata|permatanet|bank\s+permata)`)
	mandiriRegex  = regexp.MustCompile(`(?i)(?:mandiri|livin|bank\s+mandiri)`)
	briRegex      = regexp.MustCompile(`(?i)(?:bri|brimo|bank\s+rakyat\s+indonesia)`)
	bniRegex      = regexp.MustCompile(`(?i)(?:bni|bni\s+mobile|bank\s+negara\s+indonesia)`)
	bsiRegex      = regexp.MustCompile(`(?i)(?:bsi|bank\s+syariah\s+indonesia)`)
	cimbRegex     = regexp.MustCompile(`(?i)(?:cimb|octo|cimb\s+niaga)`)
	seabankRegex  = regexp.MustCompile(`(?i)(?:seabank|sea\s+bank)`)
	jeniusRegex   = regexp.MustCompile(`(?i)(?:jenius|btpn)`)
	bluRegex      = regexp.MustCompile(`(?i)(?:blu|bca\s+digital)`)
	danamonRegex  = regexp.MustCompile(`(?i)(?:danamon|d-bank)`)
	ocbcRegex     = regexp.MustCompile(`(?i)(?:ocbc|ocbc\s+nisp)`)
	paninRegex    = regexp.MustCompile(`(?i)(?:panin)`)
	btnRegex      = regexp.MustCompile(`(?i)(?:btn|bank\s+tabungan\s+negara)`)
	megaRegex     = regexp.MustCompile(`(?i)(?:bank\s+mega|mega)`)
	superbankRegex= regexp.MustCompile(`(?i)(?:superbank)`)
	aladinRegex   = regexp.MustCompile(`(?i)(?:aladin|bank\s+aladin)`)
	neobankRegex  = regexp.MustCompile(`(?i)(?:neo\s*bank|banc\s+neo\s+commerce)`)

	// Transaction types
	qrisRegex  = regexp.MustCompile(`(?i)(?:qris\s+(?:ke|di)|pembayaran\s+qris|transaksi\s+di|transaksi\s+qris)\s+([^\n\r,\.]+)`)
	gopayRegex = regexp.MustCompile(`(?i)(?:gofood|gopay|goride|shopeepay|ovo|dana)\s*[-:]?\s*([^\n\r,\.]+)`)
	debitRegex = regexp.MustCompile(`(?i)(?:transfer\s+ke|debit\s+atm|debet|pembayaran\s+ke)\s+([^\n\r,\.]+)`)
)

func ParseBankReceipt(rawText string) ParsedReceipt {
	res := ParsedReceipt{
		TransactedAt: time.Now(),
		Merchant:     "Transaksi Bank / QRIS",
		Description:  "Pengeluaran Otomatis",
		Category:     "Lainnya",
		SuggestedPod: "01", // Default to daily ritual / routine
		BankOrSource: "",
	}

	lower := strings.ToLower(rawText)

	// Filter out marketing, promo, bill reminder, newsletter emails from regex fallback
	marketingKeywords := []string{
		"agoda", "ktik.me", "newsletter", "unsubscribe", "berhenti berlangganan",
		"diskon", "promo", "voucher", "cashback hingga", "top price drops",
		"ingat tagihan", "jatuh tempo", "sebelum jatuh tempo", "lunasi tagihan",
		"jobstreet", "redbus", "tiket.com", "tokopedia promo", "shopee promo",
		"pengumuman", "privacy policy", "syarat dan ketentuan",
	}
	for _, kw := range marketingKeywords {
		if strings.Contains(lower, kw) {
			res.IsRecognized = false
			res.Amount = 0
			return res
		}
	}

	// 1. Detect Bank & E-Wallet
	if jagoRegex.MatchString(lower) {
		res.BankOrSource = "Bank Jago"
	} else if bcaRegex.MatchString(lower) {
		res.BankOrSource = "BCA"
	} else if mandiriRegex.MatchString(lower) {
		res.BankOrSource = "Bank Mandiri"
	} else if briRegex.MatchString(lower) {
		res.BankOrSource = "Bank BRI"
	} else if bniRegex.MatchString(lower) {
		res.BankOrSource = "Bank BNI"
	} else if bsiRegex.MatchString(lower) {
		res.BankOrSource = "BSI"
	} else if permataRegex.MatchString(lower) {
		res.BankOrSource = "PermataBank"
	} else if cimbRegex.MatchString(lower) {
		res.BankOrSource = "CIMB Niaga"
	} else if seabankRegex.MatchString(lower) {
		res.BankOrSource = "SeaBank"
	} else if jeniusRegex.MatchString(lower) {
		res.BankOrSource = "Jenius BTPN"
	} else if bluRegex.MatchString(lower) {
		res.BankOrSource = "blu by BCA"
	} else if danamonRegex.MatchString(lower) {
		res.BankOrSource = "Bank Danamon"
	} else if ocbcRegex.MatchString(lower) {
		res.BankOrSource = "OCBC"
	} else if paninRegex.MatchString(lower) {
		res.BankOrSource = "Panin Bank"
	} else if btnRegex.MatchString(lower) {
		res.BankOrSource = "Bank BTN"
	} else if megaRegex.MatchString(lower) {
		res.BankOrSource = "Bank Mega"
	} else if superbankRegex.MatchString(lower) {
		res.BankOrSource = "Superbank"
	} else if aladinRegex.MatchString(lower) {
		res.BankOrSource = "Bank Aladin"
	} else if neobankRegex.MatchString(lower) {
		res.BankOrSource = "NeoBank"
	} else if strings.Contains(lower, "gopay") || strings.Contains(lower, "gojek") || strings.Contains(lower, "gofood") {
		res.BankOrSource = "GoPay"
	} else if strings.Contains(lower, "ovo") {
		res.BankOrSource = "OVO"
	} else if strings.Contains(lower, "dana") {
		res.BankOrSource = "DANA"
	} else if strings.Contains(lower, "shopee") {
		res.BankOrSource = "ShopeePay"
	} else if strings.Contains(lower, "linkaja") {
		res.BankOrSource = "LinkAja"
	} else if strings.Contains(lower, "astrapay") {
		res.BankOrSource = "AstraPay"
	} else if strings.Contains(lower, "i-saku") || strings.Contains(lower, "isaku") {
		res.BankOrSource = "i.saku"
	} else if strings.Contains(lower, "flip") {
		res.BankOrSource = "Flip"
	}

	if res.BankOrSource == "" {
		// Do not recognize random emails as financial transactions
		res.IsRecognized = false
		res.Amount = 0
		return res
	}

	// 2. Extract Amount
	if matches := amountRegex.FindStringSubmatch(rawText); len(matches) > 1 {
		rawAmt := matches[1]
		cleaned := cleanAmountString(rawAmt)
		if amt, err := strconv.ParseFloat(cleaned, 64); err == nil && amt > 0 {
			res.Amount = amt
			res.IsRecognized = true
		}
	}

	// 3. Extract Merchant / Recipient
	if qrisMatches := qrisRegex.FindStringSubmatch(rawText); len(qrisMatches) > 1 {
		merchant := strings.TrimSpace(qrisMatches[1])
		if merchant != "" {
			res.Merchant = merchant
			res.Description = "QRIS - " + merchant
			res.IsRecognized = true
		}
	} else if gopayMatches := gopayRegex.FindStringSubmatch(rawText); len(gopayMatches) > 1 {
		merchant := strings.TrimSpace(gopayMatches[1])
		if merchant != "" {
			res.Merchant = merchant
			res.Description = res.BankOrSource + ": " + merchant
		}
		res.IsRecognized = true
	} else if debitMatches := debitRegex.FindStringSubmatch(rawText); len(debitMatches) > 1 {
		target := strings.TrimSpace(debitMatches[1])
		if target != "" {
			res.Merchant = target
			res.Description = "Transfer/Debit ke " + target
			res.IsRecognized = true
		}
	}

	// 4. Smart Category & Pocket Pod Mapping
	switch {
	case strings.Contains(lower, "kopi") || strings.Contains(lower, "coffee") || strings.Contains(lower, "cafe") ||
		strings.Contains(lower, "kenangan") || strings.Contains(lower, "starbucks") || strings.Contains(lower, "fore") ||
		strings.Contains(lower, "janji jiwa") || strings.Contains(lower, "point coffee"):
		res.Category = "Ritual Kopi & Kafe"
		res.SuggestedPod = "01"
		if res.Merchant == "Transaksi Bank / QRIS" {
			res.Merchant = "Kedai Kopi Pilihan"
		}

	case strings.Contains(lower, "supermarket") || strings.Contains(lower, "indomaret") || strings.Contains(lower, "alfamart") ||
		strings.Contains(lower, "sayur") || strings.Contains(lower, "buah") || strings.Contains(lower, "pasar") ||
		strings.Contains(lower, "dapur") || strings.Contains(lower, "hypermart"):
		res.Category = "Bahan Dapur & Sayur Segar"
		res.SuggestedPod = "01"
		if res.Merchant == "Transaksi Bank / QRIS" {
			res.Merchant = "Belanja Pasar & Dapur"
		}

	case strings.Contains(lower, "tiket") || strings.Contains(lower, "hotel") || strings.Contains(lower, "traveloka") ||
		strings.Contains(lower, "tiket.com") || strings.Contains(lower, "kereta") || strings.Contains(lower, "pesawat") ||
		strings.Contains(lower, "liburan") || strings.Contains(lower, "villa"):
		res.Category = "Liburan & Jalan Berdua"
		res.SuggestedPod = "02"

	case strings.Contains(lower, "dokter") || strings.Contains(lower, "apotek") || strings.Contains(lower, "klinik") ||
		strings.Contains(lower, "kimia farma") || strings.Contains(lower, "halodoc") || strings.Contains(lower, "darurat") ||
		strings.Contains(lower, "bengkel") || strings.Contains(lower, "servis"):
		res.Category = "Kesehatan & Dana Jaga-jaga"
		res.SuggestedPod = "03"

	default:
		res.Category = "Kebutuhan Rumah Tangga"
		res.SuggestedPod = "01"
	}

	return res
}

func cleanAmountString(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, ".") && strings.Contains(s, ",") {
		lastDot := strings.LastIndex(s, ".")
		lastComma := strings.LastIndex(s, ",")
		if lastComma > lastDot {
			s = strings.ReplaceAll(s, ".", "")
			s = strings.ReplaceAll(s, ",", ".")
		} else {
			s = strings.ReplaceAll(s, ",", "")
		}
	} else if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		if len(parts) > 1 && len(parts[len(parts)-1]) == 3 {
			s = strings.ReplaceAll(s, ".", "")
		}
	} else if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		if len(parts) > 1 && len(parts[len(parts)-1]) == 3 {
			s = strings.ReplaceAll(s, ",", "")
		} else {
			s = strings.ReplaceAll(s, ",", ".")
		}
	}
	return s
}
