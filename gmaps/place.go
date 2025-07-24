package gmaps

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gosom/scrapemate"
	"github.com/playwright-community/playwright-go"

	"github.com/gosom/google-maps-scraper/exiter"
)

type PlaceJobOptions func(*PlaceJob)

type PlaceJob struct {
	scrapemate.Job

	UsageInResultststs  bool
	ExtractEmail        bool
	ExitMonitor         exiter.Exiter
	ExtractExtraReviews bool
}

func NewPlaceJob(parentID, langCode, u string, extractEmail, extraExtraReviews bool, opts ...PlaceJobOptions) *PlaceJob {
	const (
		defaultPrio       = scrapemate.PriorityMedium
		defaultMaxRetries = 3
	)

	job := PlaceJob{
		Job: scrapemate.Job{
			ID:         uuid.New().String(),
			ParentID:   parentID,
			Method:     "GET",
			URL:        u,
			URLParams:  map[string]string{"hl": langCode},
			MaxRetries: defaultMaxRetries,
			Priority:   defaultPrio,
		},
	}

	job.UsageInResultststs = true
	job.ExtractEmail = extractEmail
	job.ExtractExtraReviews = extraExtraReviews

	for _, opt := range opts {
		opt(&job)
	}

	return &job
}

func WithPlaceJobExitMonitor(exitMonitor exiter.Exiter) PlaceJobOptions {
	return func(j *PlaceJob) {
		j.ExitMonitor = exitMonitor
	}
}

func (j *PlaceJob) Process(_ context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	defer func() {
		resp.Document = nil
		resp.Body = nil
		resp.Meta = nil
	}()

	raw, ok := resp.Meta["json"].([]byte)
	if !ok {
		return nil, nil, fmt.Errorf("could not convert to []byte")
	}

	entry, err := EntryFromJSON(raw)
	if err != nil {
		return nil, nil, err
	}

	entry.ID = j.ParentID

	if entry.Link == "" {
		entry.Link = j.GetURL()
	}

	allReviewsRaw, ok := resp.Meta["reviews_raw"].(fetchReviewsResponse)
	if ok && len(allReviewsRaw.pages) > 0 {
		entry.AddExtraReviews(allReviewsRaw.pages)
	}

	if j.ExtractEmail && entry.IsWebsiteValidForEmail() {
		opts := []EmailExtractJobOptions{}
		if j.ExitMonitor != nil {
			opts = append(opts, WithEmailJobExitMonitor(j.ExitMonitor))
		}

		emailJob := NewEmailJob(j.ID, &entry, opts...)

		j.UsageInResultststs = false

		return nil, []scrapemate.IJob{emailJob}, nil
	} else if j.ExitMonitor != nil {
		j.ExitMonitor.IncrPlacesCompleted(1)
	}

	return &entry, nil, err
}

func (j *PlaceJob) BrowserActions(ctx context.Context, page playwright.Page) scrapemate.Response {
	var resp scrapemate.Response

	pageResponse, err := page.Goto(j.GetURL(), playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		resp.Error = err

		return resp
	}

	if err = clickRejectCookiesIfRequired(page); err != nil {
		resp.Error = err

		return resp
	}

	const defaultTimeout = 5000

	err = page.WaitForURL(page.URL(), playwright.PageWaitForURLOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(defaultTimeout),
	})
	if err != nil {
		resp.Error = err

		return resp
	}

	resp.URL = pageResponse.URL()
	resp.StatusCode = pageResponse.Status()
	resp.Headers = make(http.Header, len(pageResponse.Headers()))

	for k, v := range pageResponse.Headers() {
		resp.Headers.Add(k, v)
	}

	raw, err := j.extractJSON(page)
	if err != nil {
		resp.Error = err

		return resp
	}

	if resp.Meta == nil {
		resp.Meta = make(map[string]any)
	}

	resp.Meta["json"] = raw

	if j.ExtractExtraReviews {
		reviewCount := j.getReviewCount(raw)
		if reviewCount > 8 { // we have more reviews
			params := fetchReviewsParams{
				page:        page,
				mapURL:      page.URL(),
				reviewCount: reviewCount,
			}

			reviewFetcher := newReviewFetcher(params)

			reviewData, err := reviewFetcher.fetch(ctx)
			if err != nil {
				return resp
			}

			resp.Meta["reviews_raw"] = reviewData
		}
	}

	return resp
}

func (j *PlaceJob) extractJSON(page playwright.Page) ([]byte, error) {
	rawI, err := page.Evaluate(js)
	if err != nil {
		return nil, err
	}

	var raw string
	switch v := rawI.(type) {
	case string:
		raw = v
	case nil:
		// Try alternative extraction methods
		return j.extractJSONFallback(page)
	default:
		// Log the actual type for debugging
		fmt.Printf("DEBUG: APP_INITIALIZATION_STATE[3][6] returned type %T, value: %v\n", rawI, rawI)
		
		// Try to convert to string anyway
		raw = fmt.Sprintf("%v", rawI)
		if raw == "" || raw == "<nil>" {
			return j.extractJSONFallback(page)
		}
	}

	if raw == "" {
		return nil, fmt.Errorf("could not convert to string")
	}

	const prefix = `)]}'`

	raw = strings.TrimSpace(strings.TrimPrefix(raw, prefix))

	return []byte(raw), nil
}

// extractJSONFallback tries alternative methods to extract data
func (j *PlaceJob) extractJSONFallback(page playwright.Page) ([]byte, error) {
	// Try alternative JavaScript paths
	alternatives := []string{
		`window.APP_INITIALIZATION_STATE && window.APP_INITIALIZATION_STATE[3] && window.APP_INITIALIZATION_STATE[3][6]`,
		`window.APP_INITIALIZATION_STATE && window.APP_INITIALIZATION_STATE[2] && window.APP_INITIALIZATION_STATE[2][6]`,
		`window.APP_INITIALIZATION_STATE && window.APP_INITIALIZATION_STATE[4] && window.APP_INITIALIZATION_STATE[4][6]`,
	}
	
	for _, altJS := range alternatives {
		rawI, err := page.Evaluate(altJS)
		if err != nil {
			continue
		}
		
		if rawStr, ok := rawI.(string); ok && rawStr != "" {
			const prefix = `)]}'`
			rawStr = strings.TrimSpace(strings.TrimPrefix(rawStr, prefix))
			return []byte(rawStr), nil
		}
	}
	
	// If all else fails, return error but with better logging
	fmt.Printf("DEBUG: All fallback methods failed for URL: %s\n", j.URL)
	return nil, fmt.Errorf("all extraction methods failed")
}

func (j *PlaceJob) getReviewCount(data []byte) int {
	tmpEntry, err := EntryFromJSON(data, true)
	if err != nil {
		return 0
	}

	return tmpEntry.ReviewCount
}

func (j *PlaceJob) UseInResults() bool {
	return j.UsageInResultststs
}

func ctxWait(ctx context.Context, dur time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(dur):
	}
}

const js = `
function parse() {
	const appState = window.APP_INITIALIZATION_STATE[3];
	if (!appState) {
		return null;
	}

	for (let i = 65; i <= 90; i++) {
		const key = String.fromCharCode(i) + "f";
		if (appState[key] && appState[key][6]) {
		return appState[key][6];
		}
	}

	return null;
}
`
