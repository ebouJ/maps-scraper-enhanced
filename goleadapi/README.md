# Go-Lead-API Client

This package provides a simple HTTP client for communicating with the go-lead-api service for lead qualification.

## Features

- **Simple HTTP Integration**: Clean HTTP POST to go-lead-api for lead qualification
- **Production Ready**: Basic error handling and timeout management
- **Minimal Configuration**: Only requires API URL and key
- **Non-blocking**: Async processing to avoid blocking the scraping pipeline

## Usage

### Basic Configuration

```bash
# Enable go-lead-api integration
--lead-api-enabled=true

# Set API connection details
--lead-api-url="https://your-lead-api.example.com"
--lead-api-key="your-api-key"
--lead-api-timeout=30s
```

### Environment Variables

```bash
export LEAD_API_ENABLED=true
export LEAD_API_URL="https://your-lead-api.example.com"
export LEAD_API_KEY="your-api-key"
```

### Integration

The client automatically integrates with the PostgreSQL result writer. When enabled, scraped leads are sent to go-lead-api for AI qualification after being saved to the database.

## API Endpoints

The client communicates with these go-lead-api endpoints:

- `POST /api/v1/leads/qualify` - Qualify a business lead with AI
- `GET /api/v1/leads/health` - Check API service health

## Architecture

This replaces the previous complex AI integration with a simple HTTP client that:

1. **Database First**: Scraped data is saved to PostgreSQL first
2. **Simple API Call**: Basic HTTP POST to go-lead-api for qualification
3. **Async Processing**: Non-blocking to avoid impacting scraping performance
4. **Clean Separation**: All AI business logic is handled by go-lead-api

## Migration from AI Integration

The old `ai-integration` package has been replaced with this simpler approach:

- **Before**: Complex client with retry logic, rate limiting, metrics
- **After**: Simple HTTP client that delegates to go-lead-api
- **Benefits**: Cleaner architecture, centralized AI logic, easier maintenance

## Configuration

```go
config := goleadapi.Config{
    BaseURL: "https://your-lead-api.example.com",
    APIKey:  "your-api-key",
    Timeout: 30 * time.Second,
}

client := goleadapi.NewClient(config)
```

The client automatically transforms `gmaps.Entry` data to the format expected by go-lead-api.