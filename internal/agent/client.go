package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

func getAPIKey() (string, error) {
	key := os.Getenv("AUTHGRAPH_API_KEY")
	if key == "" {
		return "", fmt.Errorf("AUTHGRAPH_API_KEY environment variable is required")
	}
	return key, nil
}

func getBaseURL() string {
	url := os.Getenv("AUTHGRAPH_BASE_URL")
	if url == "" {
		return "https://api.authgraph.dev"
	}
	return url
}

func apiRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	apiKey, err := getAPIKey()
	if err != nil {
		return nil, err
	}

	url := getBaseURL() + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "authgraph-agent/0.1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil {
			msg := errResp.Message
			if msg == "" {
				msg = errResp.Error
			}
			if msg != "" {
				return nil, fmt.Errorf("%d: %s", resp.StatusCode, msg)
			}
		}
		return nil, fmt.Errorf("%d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
