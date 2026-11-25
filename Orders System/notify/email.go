package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"orders/httpx"
	"os"
	"time"
)

var client = httpx.NewClient(5 * time.Second)

func SendOrderConfirmation(ctx context.Context, to, item string, cost float64) error {

	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("Resend api key is not set")
	}

	body, err := json.Marshal(map[string]any{
		"from":    "onboarding@resend.dev",
		"to":      []string{to},
		"subject": "Order Confirmation",
		"html":    fmt.Sprintf("<p>Thanks for ordering <b>%s</b>. Total: %.2f</p>", item, cost),
	})

	if err != nil {
		return err
	}

	newReq := func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	resp, err := httpx.Do(ctx, client, newReq)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("email API returned %d: %s", resp.StatusCode, msg)
	}

	return nil
}
