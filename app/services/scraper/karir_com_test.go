package scraper

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestKarirComSourceExtractJobsFromJSONLD(t *testing.T) {
	html := `<html><body><script type="application/ld+json">
		{"@type":"JobPosting","title":"Finance Staff","hiringOrganization":{"name":"Beta"},"jobLocation":{"address":{"addressLocality":"Bandung"}},"url":"/job/finance-staff"}
	</script><a href="/job/finance-staff">job</a></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	jobs := NewKarirComSource(nil, nil).extractJobs(doc)
	if len(jobs) != 1 {
		t.Fatalf("expected one job, got %d", len(jobs))
	}
	if jobs[0].Title != "Finance Staff" || jobs[0].Company != "Beta" {
		t.Fatalf("unexpected job: %+v", jobs[0])
	}
}
