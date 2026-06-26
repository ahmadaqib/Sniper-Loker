package routes

import (
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/support"

	"Goravel-learn/app/http/controllers"
	"Goravel-learn/app/facades"
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
}
