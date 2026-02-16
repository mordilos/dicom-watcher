package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func NotifyStudyReady(apiUrl, tenantID, studyID, model string) error {
	payload := map[string]string{
		"tenant": tenantID,
		"study":  studyID,
		"model":  model,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(apiUrl, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("API request failed with status code: %d", resp.StatusCode)
		return fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	log.Printf("Notification for tenant: %s - study: %s sent!", tenantID, studyID)

	return nil
}
