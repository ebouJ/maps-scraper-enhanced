package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gosom/scrapemate"

	"github.com/gosom/google-maps-scraper/gmaps"
	"github.com/gosom/google-maps-scraper/goleadapi"
)

// NewResultWriter creates a new result writer with optional go-lead-api integration
func NewResultWriter(db *sql.DB, apiConfig *goleadapi.Config) scrapemate.ResultWriter {
	writer := &resultWriter{
		db: db,
	}
	
	// Initialize go-lead-api client if configuration is provided
	if apiConfig != nil {
		writer.apiClient = goleadapi.NewClient(*apiConfig)
		writer.apiEnabled = true
		
		log.Printf("Go-lead-api integration enabled for result writer (URL: %s)", 
			apiConfig.BaseURL)
	}
	
	return writer
}

// NewResultWriterWithoutAPI creates a result writer without API integration (backward compatibility)
func NewResultWriterWithoutAPI(db *sql.DB) scrapemate.ResultWriter {
	return NewResultWriter(db, nil)
}

type resultWriter struct {
	db         *sql.DB
	apiClient  *goleadapi.Client
	apiEnabled bool
}

func (r *resultWriter) Run(ctx context.Context, in <-chan scrapemate.Result) error {
	const maxBatchSize = 50

	buff := make([]*gmaps.Entry, 0, 50)
	lastSave := time.Now().UTC()

	for result := range in {
		entry, ok := result.Data.(*gmaps.Entry)

		if !ok {
			return errors.New("invalid data type")
		}

		buff = append(buff, entry)

		if len(buff) >= maxBatchSize || time.Now().UTC().Sub(lastSave) >= time.Minute {
			err := r.batchSave(ctx, buff)
			if err != nil {
				return err
			}

			buff = buff[:0]
		}
	}

	if len(buff) > 0 {
		err := r.batchSave(ctx, buff)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *resultWriter) batchSave(ctx context.Context, entries []*gmaps.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	q := `INSERT INTO results
		(data)
		VALUES
		`
	elements := make([]string, 0, len(entries))
	args := make([]interface{}, 0, len(entries))

	for i, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}

		elements = append(elements, fmt.Sprintf("($%d)", i+1))
		args = append(args, data)
	}

	q += strings.Join(elements, ", ")
	q += " ON CONFLICT DO NOTHING"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	// Process entries with go-lead-api after successful database save
	if r.apiEnabled && r.apiClient != nil {
		r.processWithAPI(entries)
	}

	return nil
}

// processWithAPI sends entries to the go-lead-api for async processing
func (r *resultWriter) processWithAPI(entries []*gmaps.Entry) {
	for _, entry := range entries {
		// Skip entries that don't meet minimum criteria for API processing
		if !r.shouldProcessWithAPI(entry) {
			continue
		}

		// Process asynchronously to avoid blocking database operations
		go func(e *gmaps.Entry) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := r.apiClient.QualifyLead(ctx, e)
			if err != nil {
				// Log error but don't fail the main operation
				log.Printf("Go-lead-api processing error for entry %s: %v", e.Title, err)
				return
			}

			// Log successful qualification (optional)
			if result != nil && result.Success {
				log.Printf("Lead qualified: %s (score: %d, quality: %s)", 
					e.Title, result.Qualification.Score, result.Qualification.QualityLevel)
			}
		}(entry)
	}
}

// shouldProcessWithAPI determines if an entry should be processed by the API
func (r *resultWriter) shouldProcessWithAPI(entry *gmaps.Entry) bool {
	// Basic filtering criteria - customize as needed
	if entry == nil || entry.Title == "" {
		return false
	}

	// Skip entries without contact information
	if entry.Phone == "" && entry.WebSite == "" && len(entry.Emails) == 0 {
		return false
	}

	// Skip entries with very low review counts (likely not established businesses)
	if entry.ReviewCount < 3 {
		return false
	}

	return true
}

// Close gracefully shuts down the result writer and API client
func (r *resultWriter) Close() error {
	if r.apiEnabled && r.apiClient != nil {
		if err := r.apiClient.Close(); err != nil {
			log.Printf("Error closing go-lead-api client: %v", err)
			return err
		}
		
		log.Printf("Go-lead-api client closed successfully")
	}
	
	return nil
}
