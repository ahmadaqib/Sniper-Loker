package scraper

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestExtractNextDataJobs(t *testing.T) {
	html := `<html><body><script id="__NEXT_DATA__" type="application/json">
		{"props":{"pageProps":{"jobs":[{"title":"Product Manager","company":{"name":"Gamma"},"location":{"name":"Jakarta"},"url":"/jobs/product-manager","id":"pm-1"}]}}}
	</script><a href="/jobs/product-manager">job</a></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	jobs := extractNextDataJobs(doc, "https://glints.com", GlintsSourceName)
	if len(jobs) != 1 {
		t.Fatalf("expected one job, got %d", len(jobs))
	}
	if jobs[0].ExternalID != "pm-1" {
		t.Fatalf("unexpected external id: %q", jobs[0].ExternalID)
	}
}
