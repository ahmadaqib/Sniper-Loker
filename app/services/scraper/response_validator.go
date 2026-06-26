package scraper

import (
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type ValidationResult struct {
	Valid  bool
	Reason string
}

func IsValidJobPage(statusCode int, doc *goquery.Document) ValidationResult {
	if statusCode != http.StatusOK {
		return ValidationResult{Reason: "unexpected status code"}
	}
	if doc == nil {
		return ValidationResult{Reason: "empty document"}
	}

	text := strings.ToLower(strings.Join(strings.Fields(doc.Text()), " "))
	if IsChallengePage(text) || strings.Contains(text, "captcha") || strings.Contains(text, "access denied") || strings.Contains(text, "too many requests") {
		return ValidationResult{Reason: "blocked or challenge page"}
	}
	if doc.Find("a[href*='lowongan'], a[href*='job'], .job, .job-list, .job-card, [class*='job']").Length() == 0 {
		return ValidationResult{Reason: "no job markers found"}
	}

	return ValidationResult{Valid: true}
}

func IsChallengePage(body string) bool {
	if len(body) < 800 {
		return true
	}

	signatures := []string{
		"cf-challenge-running",
		"cf_chl_opt",
		"just a moment...",
		"enable javascript and cookies to continue",
		"access denied",
	}
	lower := strings.ToLower(body)
	for _, signature := range signatures {
		if strings.Contains(lower, strings.ToLower(signature)) {
			return true
		}
	}

	return strings.Contains(body, "Ray ID") && strings.Contains(body, "Cloudflare") ||
		strings.Contains(lower, "robot") && strings.Contains(lower, "captcha")
}
