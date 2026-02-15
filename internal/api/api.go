package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func NotifyStudyReady(apiUrl, studyID string) error {
	payload := map[string]string{
		"study_id": studyID,
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
		return fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	return nil
}
