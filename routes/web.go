package routes

import (
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/support"

	"Goravel-learn/app/facades"
	"Goravel-learn/app/http/controllers"
)

func Web() {
	facades.Route().Get("/", func(ctx http.Context) http.Response {
		return ctx.Response().View().Make("welcome.html", map[string]any{
			"version": support.Version,
		})
	})

	facades.Route().Static("public", "./public")

	userController := controllers.NewUserController()
	facades.Route().Get("/users", userController.Index)

	jobController := controllers.NewJobController()
	facades.Route().Get("/api/jobs", jobController.Index)
	facades.Route().Get("/ws/jobs", jobController.WebSocket)
}
