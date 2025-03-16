package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func getJSON(url string, target interface{}, token string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Debugging: Check HTTP status
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("HTTP request failed: %d - %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Debugging: Print raw response
	//	fmt.Println("Raw Response:", string(body))

	return json.Unmarshal(body, target)
}
