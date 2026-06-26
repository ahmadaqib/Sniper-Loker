package commands

import (
	"context"
	"fmt"
	"time"

	"Goravel-learn/app/services"
	"Goravel-learn/app/services/scraper"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
)

type ScrapeJobs struct{}

func (receiver *ScrapeJobs) Signature() string {
	return "scrape_jobs"
}

func (receiver *ScrapeJobs) Description() string {
	return "Scrape configured job sources and persist deduplicated jobs."
}

func (receiver *ScrapeJobs) Extend() command.Extend {
	return command.Extend{
		Category: "scraper",
		Flags: []command.Flag{
			&command.StringFlag{Name: "keyword", Usage: "Keyword to scrape", Value: ""},
			&command.StringFlag{Name: "location", Usage: "Location to scrape", Value: ""},
			&command.BoolFlag{Name: "strict", Usage: "Return an error when all sources fail", Value: true},
		},
	}
}

func (receiver *ScrapeJobs) Handle(ctx console.Context) error {
	runCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	service, err := services.NewDefaultScraperService(runCtx)
	if err != nil {
		ctx.Error(fmt.Sprintf("init scraper service failed: %v", err))
		return err
	}

	summary := service.Scrape(runCtx, scraper.SearchQuery{
		Keyword:  ctx.Option("keyword"),
		Location: ctx.Option("location"),
	})

	for _, result := range summary.SourceResults {
		if result.Error != "" {
			ctx.Error(fmt.Sprintf("%s failed: %s", result.Source, result.Error))
			continue
		}
		ctx.Info(fmt.Sprintf(
			"%s fetched=%d inserted=%d updated=%d duplicated=%d",
			result.Source,
			result.Fetched,
			result.Inserted,
			result.Updated,
			result.Duplicate,
		))
	}

	if ctx.OptionBool("strict") {
		return summary.Err()
	}

	return nil
}
