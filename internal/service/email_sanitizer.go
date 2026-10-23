package service

import (
	"html"
	"regexp"
	"strings"
)

var (
	// HTML tag and element stripping patterns
	scriptRegex  = regexp.MustCompile(`(?is)<script.*?</script>`)
	styleRegex   = regexp.MustCompile(`(?is)<style.*?</style>`)
	base64Regex  = regexp.MustCompile(`(?is)data:image/[^;]+;base64,[a-zA-Z0-9+/=]+`)
	htmlTagRegex = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRegex   = regexp.MustCompile(`[ \t\r\f]+`)
	newlineRegex = regexp.MustCompile(`\n{3,}`)

	// Account Number Masking pattern (10 to 16 digits)
	// Example: "1234567890" -> "******7890"
	accountNumRegex = regexp.MustCompile(`\b\d{10,16}\b`)
)

// SanitizeEmailBody cleans raw HTML email bodies and masks account numbers for privacy.
// It enforces a maximum character limit of 1500 characters.
func SanitizeEmailBody(rawHTMLOrText string) string {
	if rawHTMLOrText == "" {
		return ""
	}

	// 1. Remove <script> and <style> blocks
	cleaned := scriptRegex.ReplaceAllString(rawHTMLOrText, " ")
	cleaned = styleRegex.ReplaceAllString(cleaned, " ")

	// 2. Remove base64 data URIs
	cleaned = base64Regex.ReplaceAllString(cleaned, " ")

	// 3. Replace common HTML breaks with newlines before stripping tags
	cleaned = strings.ReplaceAll(cleaned, "<br>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "<br/>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "<br />", "\n")
	cleaned = strings.ReplaceAll(cleaned, "</p>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "</div>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "</tr>", "\n")

	// 4. Strip all remaining HTML tags
	cleaned = htmlTagRegex.ReplaceAllString(cleaned, " ")

	// 5. Unescape HTML entities (e.g. &nbsp;, &amp;)
	cleaned = html.UnescapeString(cleaned)

	// 6. Normalize whitespaces and newlines
	lines := strings.Split(cleaned, "\n")
	var trimmedLines []string
	for _, line := range lines {
		t := strings.TrimSpace(spaceRegex.ReplaceAllString(line, " "))
		if t != "" {
			trimmedLines = append(trimmedLines, t)
		}
	}
	cleaned = strings.Join(trimmedLines, "\n")
	cleaned = newlineRegex.ReplaceAllString(cleaned, "\n\n")

	// 7. Privacy Masking: Mask full account numbers keeping only the last 4 digits
	cleaned = accountNumRegex.ReplaceAllStringFunc(cleaned, func(match string) string {
		if len(match) <= 4 {
			return match
		}
		prefixLen := len(match) - 4
		suffix := match[prefixLen:]
		return strings.Repeat("*", prefixLen) + suffix
	})

	// 8. Truncate to maximum 1500 characters (~350 tokens)
	if len(cleaned) > 1500 {
		cleaned = cleaned[:1500]
	}

	return strings.TrimSpace(cleaned)
}
