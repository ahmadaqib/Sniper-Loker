package main

import (
	"Goravel-learn/bootstrap"
)

func main() {
	app := bootstrap.Boot()

	app.Start()
}
