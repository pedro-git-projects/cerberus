package main

import (
	"github.com/pedro-git-projects/flow-sentry/app"
)

func main() {
	app := app.New()
	app.DeployBpmn()
	app.RunTestSuites()
}
