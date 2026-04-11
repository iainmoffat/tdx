package tdx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is a thin typed wrapper around net/http for the TeamDynamix Web API.
type Client struct {
	base          *url.URL
	token         string
	http          *http.Client
	maxRetries    int
	retryAfterCap time.Duration
	userAgent     string
}

// NewClient validates the base URL and returns a ready client.
func NewClient(baseURL, token string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("base url must be absolute: %q", baseURL)
	}
	return &Client{
		base:          u,
		token:         token,
		http:          &http.Client{Timeout: 30 * time.Second},
		maxRetries:    3,
		retryAfterCap: 30 * time.Second,
		userAgent:     "tdx/0.1",
	}, nil
}

// Do performs an authenticated request and returns the response body on 2xx.
// On 429 it honours Retry-After up to retryAfterCap, retrying up to maxRetries.
// On 401 it returns ErrUnauthorized. On other non-2xx it returns an *APIError.
// body may be nil.
func (c *Client) Do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.doOnce(ctx, method, path, bodyBytes)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			wait := parseRetryAfter(resp.Header.Get("Retry-After"), c.retryAfterCap)
			_ = resp.Body.Close()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			lastErr = &APIError{Status: resp.StatusCode, Message: "rate limited"}
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("%w: %s", ErrUnauthorized, strings.TrimSpace(string(respBody)))
		}
		return nil, &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(respBody))}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("request failed after retries")
	}
	return nil, lastErr
}

func (c *Client) doOnce(ctx context.Context, method, path string, bodyBytes []byte) (*http.Response, error) {
	full := c.base.ResolveReference(&url.URL{Path: strings.TrimLeft(path, "/")})
	// Preserve the base path if present.
	if c.base.Path != "" && !strings.HasPrefix(path, "/") {
		full = c.base.ResolveReference(&url.URL{Path: path})
	}
	var reader io.Reader
	if bodyBytes != nil {
		reader = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, method, full.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}

func parseRetryAfter(h string, cap time.Duration) time.Duration {
	if h == "" {
		return 1 * time.Second
	}
	if n, err := strconv.Atoi(strings.TrimSpace(h)); err == nil {
		d := time.Duration(n) * time.Second
		if d > cap {
			return cap
		}
		return d
	}
	return 1 * time.Second
}

// Ping makes a cheap authenticated call to verify the token is valid.
// It calls GET /TDWebApi/api/time/types and discards the body; only the
// status matters. All TeamDynamix tenants mount the Web API under
// /TDWebApi/, so callers pass the tenant root (e.g. https://ufl.teamdynamix.com/)
// as the base URL and the client adds the /TDWebApi/ prefix here.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Do(ctx, http.MethodGet, "/TDWebApi/api/time/types", nil)
	return err
}
