package mclogs

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client provides access to the mclo.gs API.
type Client struct {
	// BaseURL is the base endpoint for the API.
	// Default is "https://api.mclo.gs".
	BaseURL string
	// HTTPClient is used to make requests.
	HTTPClient *http.Client
}

// NewClient creates a new mclo.gs API client.
func NewClient() *Client {
	return &Client{
		BaseURL:    "https://api.mclo.gs",
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// PasteResponse represents the response from uploading a log.
type PasteResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
	URL     string `json:"url"`
	Raw     string `json:"raw"`
	Error   string `json:"error,omitempty"`
}

// InsightsResponse represents the response from retrieving log insights.
type InsightsResponse struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Version  string   `json:"version"`
	Title    string   `json:"title"`
	Analysis Analysis `json:"analysis"`
	// In error cases the API may return:
	// { "success": false, "error": "Log not found." }
	Error string `json:"error,omitempty"`
}

// Analysis contains parsed information from the log.
type Analysis struct {
	Problems    []Problem     `json:"problems"`
	Information []Information `json:"information"`
}

// Problem represents a detected problem in the log.
type Problem struct {
	Message   string     `json:"message"`
	Counter   int        `json:"counter"`
	Entry     LogEntry   `json:"entry"`
	Solutions []Solution `json:"solutions"`
}

// Information represents additional details parsed from the log.
type Information struct {
	Message string   `json:"message"`
	Counter int      `json:"counter"`
	Label   string   `json:"label"`
	Value   string   `json:"value"`
	Entry   LogEntry `json:"entry"`
}

// LogEntry represents a single log entry.
type LogEntry struct {
	Level  int       `json:"level"`
	Time   *string   `json:"time"` // may be null
	Prefix string    `json:"prefix"`
	Lines  []LogLine `json:"lines"`
}

// LogLine represents a line within a log entry.
type LogLine struct {
	Number  int    `json:"number"`
	Content string `json:"content"`
}

// Solution represents a possible solution for a problem.
type Solution struct {
	Message string `json:"message"`
}

// Limits represents the API storage limits.
type Limits struct {
	StorageTime int `json:"storageTime"`
	MaxLength   int `json:"maxLength"`
	MaxLines    int `json:"maxLines"`
}

// PasteLog uploads the given log content to mclo.gs and returns the PasteResponse.
func (c *Client) PasteLog(content string) (*PasteResponse, error) {
	endpoint := c.BaseURL + "/1/log"
	data := url.Values{}
	data.Set("content", content)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pr PasteResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}
	if !pr.Success {
		return nil, errors.New(pr.Error)
	}
	return &pr, nil
}

// GetRawLog retrieves the raw log content by its id.
func (c *Client) GetRawLog(id string) (string, error) {
	endpoint := c.BaseURL + "/1/raw/" + id
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// If the content type is text/plain, assume success.
	if ct := resp.Header.Get("Content-Type"); strings.HasPrefix(ct, "text/plain") {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	// Otherwise, try to decode an error.
	var errResp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		return "", err
	}
	return "", errors.New(errResp.Error)
}

// GetInsights retrieves parsed insights for the log with the given id.
func (c *Client) GetInsights(id string) (*InsightsResponse, error) {
	endpoint := c.BaseURL + "/1/insights/" + id
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// In case of an error response (JSON with success:false)
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, err
		}
		return nil, errors.New(errResp.Error)
	}

	var insights InsightsResponse
	if err := json.NewDecoder(resp.Body).Decode(&insights); err != nil {
		return nil, err
	}
	// If an error string is set, report it.
	if insights.Error != "" {
		return nil, errors.New(insights.Error)
	}
	return &insights, nil
}

// AnalyseLog analyses the provided log content without saving it and returns the insights.
func (c *Client) AnalyseLog(content string) (*InsightsResponse, error) {
	endpoint := c.BaseURL + "/1/analyse"
	data := url.Values{}
	data.Set("content", content)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle non-OK responses.
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, err
		}
		return nil, errors.New(errResp.Error)
	}

	var insights InsightsResponse
	if err := json.NewDecoder(resp.Body).Decode(&insights); err != nil {
		return nil, err
	}
	if insights.Error != "" {
		return nil, errors.New(insights.Error)
	}
	return &insights, nil
}

// CheckLimits retrieves the current storage limits for logs.
func (c *Client) CheckLimits() (*Limits, error) {
	endpoint := c.BaseURL + "/1/limits"
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var limits Limits
	if err := json.NewDecoder(resp.Body).Decode(&limits); err != nil {
		return nil, err
	}
	return &limits, nil
}
