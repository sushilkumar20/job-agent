package domain

import "context"

// PageFetcher retrieves a web page and returns its readable text. The usecase
// declares the capability; an adapter in external/ owns the HTTP, the network
// safety checks, and the markup handling.
type PageFetcher interface {
	Fetch(ctx context.Context, rawURL string) (string, error)
}
