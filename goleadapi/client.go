package goleadapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gosom/google-maps-scraper/gmaps"
)

// Client provides a simple HTTP client for communicating with go-lead-api
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	timeout    time.Duration
}

// Config holds the configuration for the go-lead-api client
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// BusinessData represents the simplified business data structure for API requests
type BusinessData struct {
	Title       string  `json:"title"`
	Category    string  `json:"category"`
	Website     string  `json:"web_site,omitempty"`
	Email       string  `json:"email,omitempty"`
	Phone       string  `json:"phone,omitempty"`
	Rating      float64 `json:"review_rating,omitempty"`
	ReviewCount int     `json:"review_count,omitempty"`
	Address     string  `json:"address,omitempty"`
}

// QualifyRequest represents the request structure for lead qualification
type QualifyRequest struct {
	BusinessData BusinessData `json:"businessData"`
}

// QualificationResponse represents the response from the lead qualification API
type QualificationResponse struct {
	Success       bool `json:"success"`
	Qualification struct {
		Score                    int      `json:"score"`
		QualityLevel            string   `json:"qualityLevel"`
		Reasoning               string   `json:"reasoning"`
		Confidence              float64  `json:"confidence"`
		NextSteps               []string `json:"nextSteps"`
		ContactMethods          []string `json:"contactMethods"`
		MarketingRecommendations []string `json:"marketingRecommendations"`
	} `json:"qualification"`
	Timestamp string `json:"timestamp"`
	Error     string `json:"error,omitempty"`
}

// NewClient creates a new go-lead-api client
func NewClient(config Config) *Client {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &Client{
		baseURL: config.BaseURL,
		apiKey:  config.APIKey,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		timeout: config.Timeout,
	}
}

// QualifyLead sends a lead to go-lead-api for AI qualification
func (c *Client) QualifyLead(ctx context.Context, entry *gmaps.Entry) (*QualificationResponse, error) {
	if entry == nil {
		return nil, fmt.Errorf("entry cannot be nil")
	}

	// Transform gmaps.Entry to BusinessData
	businessData := c.transformEntry(entry)

	// Build request
	request := QualifyRequest{
		BusinessData: businessData,
	}

	// Marshal request body
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := c.baseURL + "/api/v1/leads/qualify"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Perform request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var qualResp QualificationResponse
	if err := json.NewDecoder(resp.Body).Decode(&qualResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API errors
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, qualResp.Error)
	}

	if !qualResp.Success {
		return nil, fmt.Errorf("API returned success=false: %s", qualResp.Error)
	}

	return &qualResp, nil
}

// Health checks the go-lead-api service health
func (c *Client) Health(ctx context.Context) error {
	url := c.baseURL + "/api/v1/leads/health"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// transformEntry converts gmaps.Entry to BusinessData for API requests
func (c *Client) transformEntry(entry *gmaps.Entry) BusinessData {
	// Extract first email if available
	var email string
	if len(entry.Emails) > 0 {
		email = entry.Emails[0]
	}

	return BusinessData{
		Title:       entry.Title,
		Category:    entry.Category,
		Website:     entry.WebSite,
		Email:       email,
		Phone:       entry.Phone,
		Rating:      entry.ReviewRating,
		ReviewCount: entry.ReviewCount,
		Address:     entry.Address,
	}
}

// Close gracefully shuts down the client (no-op for simple HTTP client)
func (c *Client) Close() error {
	// HTTP client doesn't need explicit cleanup
	return nil
}