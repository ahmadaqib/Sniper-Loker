package scraper

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestLokerIDSourceExtractJobs(t *testing.T) {
	html := `
		<html>
			<body>
				<div class="job-list">
					<article class="job-card">
						<h3 class="job-title"><a href="/lowongan/backend-engineer-123">Backend Engineer</a></h3>
						<div class="company-name">Acme Indonesia</div>
						<div class="location">Jakarta</div>
						<p class="description">Build APIs</p>
					</article>
				</div>
			</body>
		</html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	source := NewLokerIDSource(nil, nil)
	jobs := source.extractJobs(doc)

	if len(jobs) != 1 {
		t.Fatalf("expected one job, got %d", len(jobs))
	}
	if jobs[0].Title != "Backend Engineer" {
		t.Fatalf("unexpected title: %q", jobs[0].Title)
	}
	if jobs[0].SourceURL != "https://www.loker.id/lowongan/backend-engineer-123" {
		t.Fatalf("unexpected source url: %q", jobs[0].SourceURL)
	}
}
