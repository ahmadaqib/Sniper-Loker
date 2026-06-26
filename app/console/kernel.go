package console

import (
	"fmt"
	"os"
	"strconv"

	"Goravel-learn/app/console/commands"
	"Goravel-learn/app/facades"
	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/schedule"
)

type Kernel struct{}

func (kernel Kernel) Commands() []console.Command {
	return []console.Command{
		&commands.ScrapeJobs{},
	}
}

func (kernel Kernel) Schedule() []schedule.Event {
	interval := 5
	if value, err := strconv.Atoi(os.Getenv("SCRAPER_INTERVAL_MINUTES")); err == nil && value > 0 {
		interval = value
	}

	return []schedule.Event{
		facades.Schedule().Command("scrape_jobs").Cron(fmt.Sprintf("*/%d * * * *", interval)).SkipIfStillRunning(),
	}
}
