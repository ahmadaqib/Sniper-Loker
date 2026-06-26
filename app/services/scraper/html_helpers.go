package scraper

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func firstNonEmptySelection(parent *goquery.Selection, selectors []string) *goquery.Selection {
	for _, selector := range selectors {
		match := parent.Find(selector).First()
		if cleanText(match.Text()) != "" || match.AttrOr("href", "") != "" {
			return match
		}
	}
	return parent.Find("__missing__")
}

func cleanText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func absolutizeURL(baseURL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return raw
	}
	return base.ResolveReference(parsed).String()
}

func externalIDFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	path := strings.Trim(parsed.Path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func extractJSONLDJobs(doc *goquery.Document, baseURL, sourceName string) []ScrapedJob {
	var jobs []ScrapedJob
	doc.Find("script[type='application/ld+json']").Each(func(_ int, script *goquery.Selection) {
		var payload any
		if err := json.Unmarshal([]byte(script.Text()), &payload); err != nil {
			return
		}
		jobs = append(jobs, jobsFromStructuredJSON(payload, baseURL, sourceName)...)
	})
	return jobs
}

func jobsFromStructuredJSON(value any, baseURL, sourceName string) []ScrapedJob {
	switch typed := value.(type) {
	case []any:
		var jobs []ScrapedJob
		for _, item := range typed {
			jobs = append(jobs, jobsFromStructuredJSON(item, baseURL, sourceName)...)
		}
		return jobs
	case map[string]any:
		if graph, ok := typed["@graph"]; ok {
			return jobsFromStructuredJSON(graph, baseURL, sourceName)
		}

		jobType := strings.ToLower(asString(typed["@type"]))
		if strings.Contains(jobType, "jobposting") {
			return []ScrapedJob{jobFromJSONLD(typed, baseURL, sourceName)}
		}

		var jobs []ScrapedJob
		for _, candidate := range []string{"itemListElement", "jobs", "jobPosts", "jobPostings", "data", "results"} {
			if nested, ok := typed[candidate]; ok {
				jobs = append(jobs, jobsFromStructuredJSON(nested, baseURL, sourceName)...)
			}
		}
		return jobs
	default:
		return nil
	}
}

func jobFromJSONLD(data map[string]any, baseURL, sourceName string) ScrapedJob {
	company := ""
	if org, ok := data["hiringOrganization"].(map[string]any); ok {
		company = asString(org["name"])
	}
	location := ""
	if loc, ok := data["jobLocation"].(map[string]any); ok {
		if address, ok := loc["address"].(map[string]any); ok {
			location = strings.Join(nonEmptyStrings(
				asString(address["addressLocality"]),
				asString(address["addressRegion"]),
				asString(address["addressCountry"]),
			), ", ")
		}
	}

	sourceURL := absolutizeURL(baseURL, asString(data["url"]))
	return ScrapedJob{
		Title:       cleanText(asString(data["title"])),
		Company:     cleanText(company),
		Location:    cleanText(location),
		Description: cleanText(asString(data["description"])),
		ApplyURL:    sourceURL,
		SourceURL:   sourceURL,
		Source:      sourceName,
		ExternalID:  externalIDFromURL(sourceURL),
		Raw: map[string]any{
			"source_selector": "json_ld",
		},
	}
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(typed, 'f', -1, 64), "0"), ".")
	default:
		return ""
	}
}

func nonEmptyStrings(values ...string) []string {
	var filtered []string
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			filtered = append(filtered, strings.TrimSpace(value))
		}
	}
	return filtered
}
