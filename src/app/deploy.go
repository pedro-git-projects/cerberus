package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// DeployBpmn deploys the default BPMN file.
func (app *App) DeployBpmn() {
	app.zeebe.DeployWorkflow(app.config.BpmnPath)
}

// loadAllTestSuites loads all test suites from the suites directory.
func (app *App) loadAllTestSuites() ([]TestSuite, error) {
	var allSuites []TestSuite
	files, err := os.ReadDir(app.config.SuitesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read suites directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".toml" {
			continue
		}
		filePath := filepath.Join(app.config.SuitesPath, file.Name())
		suites, err := app.loadTestSuites(filePath)
		if err != nil {
			log.Printf("Error loading file %s: %v", file.Name(), err)
			continue
		}
		allSuites = append(allSuites, suites...)
	}

	return allSuites, nil
}

// LoadSelectedTestSuites loads test suites that match the provided process IDs.
func (app *App) LoadSelectedTestSuites(selected []string) ([]TestSuite, error) {
	allSuites, err := app.loadAllTestSuites()
	if err != nil {
		return nil, err
	}
	var selectedSuites []TestSuite
	selectedMap := make(map[string]struct{})
	for _, s := range selected {
		s = strings.TrimSpace(s)
		selectedMap[s] = struct{}{}
	}
	for _, suite := range allSuites {
		if _, ok := selectedMap[suite.ProcessID]; ok {
			selectedSuites = append(selectedSuites, suite)
		}
	}
	return selectedSuites, nil
}

// DeployWorkflowsFromSuites deploys workflows defined in each test suite.
func (app *App) DeployWorkflowsFromSuites() {
	suites, err := app.loadAllTestSuites()
	if err != nil {
		log.Fatalf("failed to load test suites: %v", err)
	}

	for _, suite := range suites {
		if suite.WorkflowDeploy == "" {
			continue
		}
		// Assume workflow file names are relative to the BPMN folder.
		workflowPath := filepath.Join(filepath.Dir(app.config.BpmnPath), suite.WorkflowDeploy)
		app.zeebe.DeployWorkflow(workflowPath)
	}
}

// DeployAllWorkflows deploys all BPMN files from the workflows directory.
func (app *App) DeployAllWorkflows() error {
	bpmnDir := filepath.Dir(app.config.BpmnPath)
	files, err := os.ReadDir(bpmnDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".bpmn" {
			continue
		}
		workflowPath := filepath.Join(bpmnDir, file.Name())
		app.zeebe.DeployWorkflow(workflowPath)
	}
	return nil
}

// DeployArbitraryWorkflows deploys specified BPMN files from the workflows directory.
func (app *App) DeployArbitraryWorkflows(workflowFiles []string) error {
	bpmnDir := filepath.Dir(app.config.BpmnPath)
	for _, wf := range workflowFiles {
		wf = strings.TrimSpace(wf)
		if wf == "" {
			continue
		}
		workflowPath := filepath.Join(bpmnDir, wf)
		app.zeebe.DeployWorkflow(workflowPath)
	}
	return nil
}
