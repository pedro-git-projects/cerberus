package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
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

func normalizeString(value interface{}) string {
	strValue, ok := value.(string)
	if !ok {
		return fmt.Sprintf("%v", value) // Convert non-string types
	}

	// Trim extra JSON-style quotes if they exist
	return strings.Trim(strValue, `"`)
}

func extractNestedString(value interface{}) string {
	// If it's already a string, return it directly
	strValue, ok := value.(string)
	if ok {
		return strValue
	}

	// If it's a map, extract the first string field
	if jsonMap, ok := value.(map[string]interface{}); ok {
		if myProperty, exists := jsonMap["myProperty"]; exists {
			if str, ok := myProperty.(string); ok {
				return str
			}
		}
	}

	// Convert JSON object to string for comparison
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value) // Fallback if not JSON
	}

	return string(jsonBytes)
}
