package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/entities"
	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/worker"
)

// startExecutionListenerWorker starts a job worker for the execution listener.
// Ensure the JobType matches your BPMN configuration; here we use "completionStatus".
func (app *App) startExecutionListenerWorker() {
	workerInstance := app.client.NewJobWorker().
		JobType("completionStatus"). // This should match the job type set in your BPMN execution listener.
		Handler(app.executionListenerHandler).
		Open()
	log.Println("Execution listener worker started for job type 'completionStatus'")
	// Store the worker instance if you need to close it later.
	app.listener.executionWorker = workerInstance
}

// executionListenerHandler handles jobs for the execution listener.
func (app *App) executionListenerHandler(client worker.JobClient, job entities.Job) {
	var vars map[string]interface{}
	if err := json.Unmarshal([]byte(job.Variables), &vars); err != nil {
		log.Printf("Error unmarshaling execution listener job variables: %v", err)
		return
	}
	log.Printf("Execution listener job received with variables: %v", vars)
	// Push the variables into our encapsulated channel.
	app.listener.executionChan <- vars

	// Complete the job to release the lock.
	if _, err := client.NewCompleteJobCommand().JobKey(job.Key).Send(context.Background()); err != nil {
		log.Printf("Error completing execution listener job: %v", err)
	}
}

// subscribeToExecutionListener waits for a notification on the encapsulated channel.
func (app *App) subscribeToExecutionListener(timeout time.Duration) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case vars := <-app.listener.executionChan:
		return vars, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("timed out waiting for execution listener notification")
	}
}

func (app *App) subscribeToExecutionListenerAfterThreshold(threshold time.Time, timeout time.Duration, token string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case vars := <-app.listener.executionChan:
			// Assume the job variables include "processInstanceKey"
			piKeyRaw, ok := vars["processInstanceKey"]
			if !ok {
				log.Printf("❌ Received notification without processInstanceKey: %v", vars)
				continue // or return error if desired
			}
			// Zeebe uses numeric keys; adjust conversion as needed.
			piKeyFloat, ok := piKeyRaw.(float64)
			if !ok {
				log.Printf("❌ processInstanceKey is not a number: %v", piKeyRaw)
				continue
			}
			piKey := int64(piKeyFloat)

			// Use your operate service to fetch the process instance details.
			instance, err := app.operate.FetchProcessInstance(piKey, token)
			if err != nil {
				log.Printf("❌ Error fetching process instance %d: %v", piKey, err)
				continue
			}
			// Parse the start date. Adjust the layout string if your Operate API returns a different format.
			layout := "2006-01-02 15:04:05"
			startTime, err := time.Parse(layout, instance.StartDate)
			if err != nil {
				log.Printf("❌ Error parsing startDate (%s): %v", instance.StartDate, err)
				continue
			}
			// Check whether this instance was started after the threshold.
			if startTime.After(threshold) {
				return vars, nil
			} else {
				log.Printf("🔎 Ignoring notification for instance %d started at %v (threshold: %v)", piKey, startTime, threshold)
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for execution listener notification after threshold")
		}
	}
}
