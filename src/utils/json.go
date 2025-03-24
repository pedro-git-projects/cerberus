package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func GetJSON(client *http.Client, url string, target interface{}, token string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("HTTP request failed: %d - %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, target)
}

func NormalizeString(value interface{}) string {
	strValue, ok := value.(string)
	if !ok {
		return fmt.Sprintf("%v", value) // Convert non-string types
	}

	// Trim extra JSON-style quotes if they exist
	return strings.Trim(strValue, `"`)
}

func ExtractNestedString(value interface{}) string {
	// If it's already a string, return it directly
	strValue, ok := value.(string)
	if ok {
		return strings.Trim(strValue, `"`) // Remove unnecessary quotes
	}

	// If it's a map, extract the first string field
	if jsonMap, ok := value.(map[string]interface{}); ok {
		if myProperty, exists := jsonMap["myProperty"]; exists {
			if str, ok := myProperty.(string); ok {
				return strings.Trim(str, `"`)
			}
		}
	}

	// Convert JSON object to string for comparison
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value) // If not JSON, return as-is
	}

	return string(jsonBytes)
}
