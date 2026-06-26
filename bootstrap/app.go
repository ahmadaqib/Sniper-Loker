package bootstrap

import (
	contractsfoundation "github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/foundation"

	appconsole "Goravel-learn/app/console"
	"Goravel-learn/app/facades"
	"Goravel-learn/config"
	"Goravel-learn/routes"
)

func Boot() contractsfoundation.Application {
	app := foundation.Setup().
		WithMigrations(Migrations).
		WithRouting(func() {
			routes.Web()
			routes.Grpc()
		}).
		WithProviders(Providers).
		WithConfig(config.Boot).
		Create()

	kernel := appconsole.Kernel{}
	facades.Artisan().Register(kernel.Commands())
	facades.Schedule().Register(kernel.Schedule())

	return app
}
