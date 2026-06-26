package scraper

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var mosaicProviderData = regexp.MustCompile(`(?s)window\.mosaic\.providerData\s*=\s*(\{.*?\});`)

func extractNextDataJobs(doc *goquery.Document, baseURL, sourceName string) []ScrapedJob {
	script := strings.TrimSpace(doc.Find("script#__NEXT_DATA__").First().Text())
	if script == "" {
		return nil
	}
	var payload any
	if err := json.Unmarshal([]byte(script), &payload); err != nil {
		return nil
	}
	return jobsFromGenericJSON(payload, baseURL, sourceName)
}

func extractMosaicJobs(doc *goquery.Document, baseURL, sourceName string) []ScrapedJob {
	var jobs []ScrapedJob
	doc.Find("script").Each(func(_ int, script *goquery.Selection) {
		matches := mosaicProviderData.FindStringSubmatch(script.Text())
		if len(matches) != 2 {
			return
		}
		var payload any
		if err := json.Unmarshal([]byte(matches[1]), &payload); err != nil {
			return
		}
		jobs = append(jobs, jobsFromGenericJSON(payload, baseURL, sourceName)...)
	})
	return jobs
}

func jobsFromGenericJSON(value any, baseURL, sourceName string) []ScrapedJob {
	switch typed := value.(type) {
	case []any:
		var jobs []ScrapedJob
		for _, item := range typed {
			jobs = append(jobs, jobsFromGenericJSON(item, baseURL, sourceName)...)
		}
		return jobs
	case map[string]any:
		if job, ok := genericJobFromMap(typed, baseURL, sourceName); ok {
			return []ScrapedJob{job}
		}
		var jobs []ScrapedJob
		for key, nested := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "job") || strings.Contains(lower, "result") || lower == "data" || lower == "props" || lower == "pageprops" {
				jobs = append(jobs, jobsFromGenericJSON(nested, baseURL, sourceName)...)
			}
		}
		return jobs
	default:
		return nil
	}
}

func genericJobFromMap(data map[string]any, baseURL, sourceName string) (ScrapedJob, bool) {
	title := firstString(data, "title", "jobTitle", "position", "name", "job_title")
	company := firstNestedString(data, []string{"company", "companyName", "company_name", "hiringOrganization", "companyDisplayName"}, []string{"name", "displayName", "companyName"})
	if company == "" {
		company = firstString(data, "company", "companyName", "company_name", "companyDisplayName")
	}
	if title == "" || company == "" {
		return ScrapedJob{}, false
	}

	location := firstNestedString(data, []string{"location", "jobLocation", "locations"}, []string{"name", "city", "displayName", "addressLocality"})
	if location == "" {
		location = firstString(data, "location", "formattedLocation", "locationName", "city")
	}
	sourceURL := firstString(data, "url", "jobUrl", "job_url", "link", "shareUrl")
	sourceURL = absolutizeURL(baseURL, sourceURL)

	return ScrapedJob{
		Title:       cleanText(title),
		Company:     cleanText(company),
		Location:    cleanText(location),
		Description: cleanText(firstString(data, "description", "snippet", "summary")),
		ApplyURL:    sourceURL,
		SourceURL:   sourceURL,
		Source:      sourceName,
		ExternalID:  firstNonEmpty(firstString(data, "id", "jobId", "job_id", "externalId"), externalIDFromURL(sourceURL)),
		Raw:         map[string]any{"source_selector": "embedded_json"},
	}, true
}

func firstString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := asString(data[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstNestedString(data map[string]any, parents []string, children []string) string {
	for _, parent := range parents {
		switch value := data[parent].(type) {
		case map[string]any:
			if found := firstString(value, children...); found != "" {
				return found
			}
		case []any:
			for _, item := range value {
				if mapped, ok := item.(map[string]any); ok {
					if found := firstString(mapped, children...); found != "" {
						return found
					}
				}
			}
		case string:
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
