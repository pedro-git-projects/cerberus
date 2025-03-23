package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/pedro-git-projects/flow-sentry/app"
)

func main() {
	app := app.New()
	app.DeployBpmn()

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}
	suitesPath := filepath.Join(cwd, "", "testsuites.toml")

	suites, err := app.LoadTestSuites(suitesPath)
	if err != nil {
		log.Fatalf("Error loading test suites: %v", err)
	}

	app.RunTestSuites(suites)
}
