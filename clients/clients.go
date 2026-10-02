package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"proj/model"
)

const (
	DefaultResults = 100
	MaxResults     = 5000
	DefaultBaseURL = "https://randomuser.me/api/"

	maxResponseBytes = 10 << 20
	maxErrorBytes    = 4 << 10
	maxAttempts      = 3
)

type apiResponse struct {
	Results []model.User `json:"results"`
}

// randomuser.me строит страницу по паре (seed, page). Если seed не передать,
// каждый запрос вернёт новых людей и page станет бесполезным.
func FetchUsersFrom(ctx context.Context, c *http.Client, baseURL, seed string, page, results int) ([]model.User, error) {
	if seed == "" {
		return nil, fmt.Errorf("seed обязателен")
	}
	if c == nil {
		c = http.DefaultClient
	}
	if results < 1 {
		results = DefaultResults
	}
	if results > MaxResults {
		results = MaxResults
	}
	if page < 1 {
		page = 1
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("некорректный baseURL: %w", err)
	}

	q := url.Values{}
	q.Set("seed", seed)
	q.Set("results", strconv.Itoa(results))
	q.Set("page", strconv.Itoa(page))
	q.Set("nat", "gb")
	base.RawQuery = q.Encode()

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		users, retry, err := fetchPage(ctx, c, base.String())
		if err == nil {
			return users, nil
		}
		lastErr = err

		if !retry {
			return nil, err
		}
		if attempt == maxAttempts {
			break
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
	return nil, fmt.Errorf("страница %d не получена за %d попытки: %w", page, maxAttempts, lastErr)
}

// retry говорит, стоит ли повторять запрос: 5xx и 429 — временные.
func fetchPage(ctx context.Context, c *http.Client, endpoint string) ([]model.User, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
		io.Copy(io.Discard, resp.Body)

		retry := resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests

		if msg := strings.TrimSpace(string(body)); msg != "" {
			return nil, retry, fmt.Errorf("API вернул %d: %s", resp.StatusCode, msg)
		}
		return nil, retry, fmt.Errorf("API вернул %d", resp.StatusCode)
	}

	var v apiResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&v); err != nil {
		return nil, false, err
	}
	return v.Results, false, nil
}
