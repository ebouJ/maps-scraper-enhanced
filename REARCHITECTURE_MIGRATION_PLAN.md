# Maps Scraper (`maps-scraper`) Migration Plan

**Project:** `maps-scraper`  
**Version:** 1.0  
**Date:** July 10, 2025  

## 1. Executive Summary

This document details the migration plan for the `maps-scraper` service. The impact on this service is less extensive than on the API and frontend projects but is critical for ensuring data integrity. The primary goal is to ensure that all scraped data is correctly associated with the `organization` that initiated the scraping job. This involves changes to how jobs are initiated and how data is written to the database or passed to other APIs.

## 2. Job Initiation & Tenant Context

Currently, a scraping job might be tied to an agency or client. This needs to be standardized to a single `organization_id`.

### Step 2.1: Update Job Input
- **Command-Line Arguments**: If the scraper is run via the command line, it must accept a mandatory `--organization-id` flag.
  ```bash
  # Old command
  ./maps-scraper --query="restaurants in new york" --client-id="some-uuid"

  # New command
  ./maps-scraper --query="restaurants in new york" --organization-id="some-uuid"
  ```
- **API Trigger**: If the scraper is triggered via an internal API call (e.g., from the `go-lead-api`), the request body for initiating a job must include the `organization_id`.

### Step 2.2: Internal Context
- **Configuration Struct**: The main configuration or job struct within the Go application must be updated to hold the `OrganizationID`.
  ```go
  type ScraperJob struct {
      Query          string
      OrganizationID uuid.UUID // Changed from ClientID/AgencyID
      // ... other fields
  }
  ```

## 3. Data Output & Storage

This is the most critical part of the scraper's migration. The output data must be correctly tagged.

### Step 3.1: Direct Database Writes
- **Update `INSERT` Statements**: If the scraper writes directly to the PostgreSQL database, all `INSERT` statements must be modified.
  - The target tables (e.g., `leads`, `contacts`) will have an `organization_id` column.
  - The `INSERT` statement must populate this column with the `OrganizationID` from the job context.
  ```go
  // Old logic
  db.Exec("INSERT INTO leads (name, client_id) VALUES (?, ?)", lead.Name, job.ClientID)

  // New logic
  db.Exec("INSERT INTO leads (name, organization_id) VALUES (?, ?)", lead.Name, job.OrganizationID)
  ```

### Step 3.2: API-Based Data Output
- **Update API Client**: The scraper uses a client in the `goleadapi/` directory to communicate with the `go-lead-api`. This client must be updated.
  - The function for creating a lead (or other record) must be modified to send the `organization_id` in the request body.
  - The API endpoint it calls will be the newly refactored endpoint on the `go-lead-api` which expects an `organization_id`.

## 4. Billing & Usage Tracking

While the scraper itself may not directly call Lago, it generates the raw data that constitutes a billable event. The responsibility for recording the usage should lie with the service that receives the data.

- **No Direct Lago Integration**: It is recommended to **not** put Lago integration logic directly into the scraper. The scraper's job is to scrape.
- **Responsibility of the API**: The `go-lead-api` (or whichever service receives the scraped data) is responsible for calling the `FeatureUsageTracker`. When it receives a batch of 100 leads from the scraper for a specific organization, *it* should be the one to record `100` units of the `lead_generation` feature for that organization.
- **This simplifies the scraper's role and centralizes billing logic.**

## 5. Testing

- **Update Unit Tests**: Any unit tests for data processing functions should be updated to check for the presence and correctness of the `OrganizationID`.
- **Integration Tests**: Create an integration test that runs a small scraping job and verifies that the data written to a test database contains the correct `organization_id`.

By focusing on these key areas, the `maps-scraper` can be effectively integrated into the new architecture, ensuring that all data it generates is correctly owned and tracked.
