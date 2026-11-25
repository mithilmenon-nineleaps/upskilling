package httpx

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

const maxRetries = 3

func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func Do(ctx context.Context, client *http.Client, newReq func() (*http.Request, error)) (*http.Response, error) {

	var lastError error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := newReq()
		if err != nil {
			return nil, err // a bug in our own code, retrying won't help
		}
		resp, err := client.Do(req.WithContext(ctx))
		if err == nil && !shouldRetry(resp.StatusCode) {
			return resp, nil
		}

		if err != nil {
			lastError = err
		} else {
			lastError = fmt.Errorf("server returned status %d", resp.StatusCode)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if attempt == maxRetries {
			break
		}
		select {
		case <-time.After(backoff(attempt)):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("gave up after %d retries: %w", maxRetries, lastError)
}

func shouldRetry(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func backoff(attempt int) time.Duration {
	d := 500 * time.Millisecond << attempt
	return d/2 + time.Duration(rand.Int63n(int64(d/2)))
}
