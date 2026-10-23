package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMTransactionResult is the structured output returned by Gemini LLM
type LLMTransactionResult struct {
	IsTransaction          bool     `json:"is_transaction"`
	ProviderCode           *string  `json:"provider_code"`
	Direction              *string  `json:"direction"`
	Amount                 *float64 `json:"amount"`
	AdminFee               *float64 `json:"admin_fee"`
	MerchantOrCounterparty *string  `json:"merchant_or_counterparty"`
	SuggestedCategory      *string  `json:"suggested_category"`
	ReferenceNumber        *string  `json:"reference_number"`
	AccountSuffix          *string  `json:"account_suffix"`
	TransactionTime        *string  `json:"transaction_time"` // e.g. "2026-10-18T21:46:00+07:00" or ISO-8601 string if mentioned
}

type LLMParserService interface {
	ParseEmailMutation(ctx context.Context, cleanBody string) (*LLMTransactionResult, string, error)
}

type GeminiLLMParserService struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewGeminiLLMParserService(apiKey, model string) *GeminiLLMParserService {
	if model == "" {
		model = "gemini-2.5-flash-lite"
	}
	return &GeminiLLMParserService{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// System prompt defining strict guidelines for Indonesian bank/e-wallet email mutation parsing
const mutationSystemPrompt = `You are a financial transaction extractor specialized in Indonesian banking and e-wallet notification emails (e.g., BCA, Mandiri, BRI, BNI, BSI, CIMB Niaga, PermataBank, Danamon, Panin, BTN, Bank Mega, OCBC, Maybank, Sinarmas, Bank Jago, SeaBank, Jenius/BTPN, blu by BCA Digital, Bank Aladin, NeoBank, Superbank, GoPay, OVO, DANA, ShopeePay, LinkAja, AstraPay, i.saku, Flip).
Analyze the provided sanitized email text and extract structured transaction data according to the schema.

Rules:
1. is_transaction: true if this is an actual debit/credit transaction confirmation (transfer, QRIS payment, e-commerce, bill pay, incoming transfer, top-up). false for promotional emails, OTP codes, monthly statements, login alerts, or security notices.
2. provider_code: must be the standard banking code or e-wallet code (e.g. BCA, MANDIRI, BRI, BNI, BSI, CIMB, PERMATA, DANAMON, PANIN, BTN, MEGA, OCBC, MAYBANK, JAGO, SEABANK, JENIUS, BLU, ALADIN, NEOBANK, SUPERBANK, GOPAY, OVO, DANA, SHOPEEPAY, LINKAJA, ASTRAPAY, ISAKU, FLIP, OTHER).
3. direction: must be one of:
   - "IN" (Incoming transfer, top-up received, salary, refund)
   - "OUT" (Payment, expense, outgoing transfer, QRIS, withdrawal)
   - "INTERNAL_TRANSFER" (Transfer between user's own accounts or pockets)
4. amount: numeric value in IDR (exclude admin fee). If multiple amounts exist, extract the principal transaction amount.
5. admin_fee: numeric transfer/admin fee (default to 0 if not explicitly mentioned).
6. merchant_or_counterparty: name of store, recipient, or sender (e.g. "Indomaret", "PT Tokopedia", "John Doe").
7. suggested_category: concise category in Indonesian/English (e.g. "Food & Drink", "Groceries", "Transfer", "Utilities", "Shopping", "Transport").
8. reference_number: reference ID, journal number, or transaction ID if present.
9. account_suffix: the last 3-4 digits of the bank account or card mentioned (e.g. "7890"), or null if unavailable.
10. transaction_time: The date/time when transaction actually took place as stated in the email body (e.g., "18 October 2026, 21:46 WIB"), converted into ISO-8601 string format (e.g. "2026-10-18T21:46:00+07:00" or "YYYY-MM-DDTHH:MM:SSZ"). If no specific time or year is given, use the stated date or return null.`

// Gemini API request payload structs
type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	ResponseMimeType string                 `json:"responseMimeType"`
	ResponseSchema   map[string]interface{} `json:"responseSchema"`
	Temperature      float64                `json:"temperature"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent         `json:"systemInstruction,omitempty"`
	Contents          []geminiContent        `json:"contents"`
	GenerationConfig  geminiGenerationConfig `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (s *GeminiLLMParserService) ParseEmailMutation(ctx context.Context, cleanBody string) (*LLMTransactionResult, string, error) {
	if s.apiKey == "" {
		// Mock fallback for local testing if API key is not yet set in environment
		return fallbackMockParse(cleanBody)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", s.model, s.apiKey)

	schema := map[string]interface{}{
		"type": "OBJECT",
		"properties": map[string]interface{}{
			"is_transaction": map[string]interface{}{
				"type": "BOOLEAN",
			},
			"provider_code": map[string]interface{}{
				"type": "STRING",
				"enum": []string{"BCA", "MANDIRI", "BRI", "BNI", "BSI", "JAGO", "SEABANK", "GOPAY", "OVO", "DANA", "SHOPEEPAY", "OTHER"},
			},
			"direction": map[string]interface{}{
				"type": "STRING",
				"enum": []string{"IN", "OUT", "INTERNAL_TRANSFER"},
			},
			"amount": map[string]interface{}{
				"type": "NUMBER",
			},
			"admin_fee": map[string]interface{}{
				"type": "NUMBER",
			},
			"merchant_or_counterparty": map[string]interface{}{
				"type": "STRING",
			},
			"suggested_category": map[string]interface{}{
				"type": "STRING",
			},
			"reference_number": map[string]interface{}{
				"type": "STRING",
			},
			"account_suffix": map[string]interface{}{
				"type": "STRING",
			},
			"transaction_time": map[string]interface{}{
				"type": "STRING",
			},
		},
		"required": []string{"is_transaction"},
	}

	reqBody := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: mutationSystemPrompt}},
		},
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: cleanBody}},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			ResponseMimeType: "application/json",
			ResponseSchema:   schema,
			Temperature:      0.1,
		},
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal gemini request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("gemini api request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response body: %w", err)
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
		return nil, string(respBytes), fmt.Errorf("failed to parse gemini response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, string(respBytes), fmt.Errorf("gemini api error (code %d): %s", geminiResp.Error.Code, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, string(respBytes), errors.New("empty response candidates from gemini")
	}

	rawResultJSON := geminiResp.Candidates[0].Content.Parts[0].Text
	var parsed LLMTransactionResult
	if err := json.Unmarshal([]byte(rawResultJSON), &parsed); err != nil {
		return nil, rawResultJSON, fmt.Errorf("failed to unmarshal structured transaction json: %w", err)
	}

	return &parsed, rawResultJSON, nil
}

// fallbackMockParse provides deterministic parsing for automated offline tests or when API key is unconfigured
func fallbackMockParse(cleanBody string) (*LLMTransactionResult, string, error) {
	upper := strings.ToUpper(cleanBody)
	isTx := strings.Contains(upper, "TRANSAKSI") || strings.Contains(upper, "TRANSFER") || strings.Contains(upper, "PEMBAYARAN") || strings.Contains(upper, "BERHASIL")

	res := &LLMTransactionResult{
		IsTransaction: isTx,
	}

	if isTx {
		prov := "OTHER"
		if strings.Contains(upper, "BCA") {
			prov = "BCA"
		} else if strings.Contains(upper, "JAGO") {
			prov = "JAGO"
		} else if strings.Contains(upper, "MANDIRI") {
			prov = "MANDIRI"
		} else if strings.Contains(upper, "GOPAY") {
			prov = "GOPAY"
		}
		res.ProviderCode = &prov

		direction := "OUT"
		if strings.Contains(upper, "DITERIMA") || strings.Contains(upper, "MASUK") {
			direction = "IN"
		}
		res.Direction = &direction

		amt := 50000.0
		res.Amount = &amt

		cat := "General"
		res.SuggestedCategory = &cat

		m := "Merchant / Counterparty"
		res.MerchantOrCounterparty = &m

		ref := "REF-MOCK-123456"
		res.ReferenceNumber = &ref
	}

	raw, _ := json.Marshal(res)
	return res, string(raw), nil
}
