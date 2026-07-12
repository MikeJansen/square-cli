// Package client implements a thin HTTP client for the Square Connect API.
// It handles authentication, the Square-Version header, base-URL selection,
// and structured error reporting, while leaving request/response bodies as
// raw JSON so callers retain full fidelity of the API payload.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Options configures a Client.
type Options struct {
	BaseURL     string
	AccessToken string
	APIVersion  string
	UserAgent   string
	Verbose     bool
	HTTPClient  *http.Client
}

// Client is a configured Square API client.
type Client struct {
	baseURL     string
	accessToken string
	apiVersion  string
	userAgent   string
	verbose     bool
	http        *http.Client
	logf        func(format string, args ...any)
}

// New constructs a Client from Options.
func New(o Options) *Client {
	hc := o.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	ua := o.UserAgent
	if ua == "" {
		ua = "square-cli"
	}
	return &Client{
		baseURL:     strings.TrimRight(o.BaseURL, "/"),
		accessToken: o.AccessToken,
		apiVersion:  o.APIVersion,
		userAgent:   ua,
		verbose:     o.Verbose,
		http:        hc,
		logf:        func(string, ...any) {},
	}
}

// SetLogger installs a logging function used for verbose request tracing.
func (c *Client) SetLogger(logf func(format string, args ...any)) {
	if logf != nil {
		c.logf = logf
	}
}

// Response is the result of an API call.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Request describes a single API call.
type Request struct {
	Method string
	Path   string // e.g. "/v2/payments"; leading slash optional
	Query  url.Values
	Body   io.Reader
}

// Do executes an API request and returns the raw response. Non-2xx responses
// are returned as an *APIError.
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	if c.accessToken == "" {
		return nil, fmt.Errorf("no access token configured (set SQUARE_ACCESS_TOKEN, use --access-token, or configure a profile)")
	}

	u := c.baseURL + "/" + strings.TrimLeft(req.Path, "/")
	if len(req.Query) > 0 {
		u += "?" + req.Query.Encode()
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, u, req.Body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.accessToken)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", c.userAgent)
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if c.apiVersion != "" {
		httpReq.Header.Set("Square-Version", c.apiVersion)
	}

	if c.verbose {
		c.logf("%s %s", req.Method, u)
		if c.apiVersion != "" {
			c.logf("Square-Version: %s", c.apiVersion)
		}
	}

	start := time.Now()
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if c.verbose {
		c.logf("-> %d (%s, %d bytes)", resp.StatusCode, time.Since(start).Round(time.Millisecond), len(body))
	}

	out := &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: body}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, parseAPIError(resp.StatusCode, body)
	}
	return out, nil
}

// APIError represents a non-2xx Square API response.
type APIError struct {
	StatusCode int
	Errors     []SquareError `json:"errors"`
	Raw        []byte        `json:"-"`
}

// SquareError mirrors the Square error object.
type SquareError struct {
	Category string `json:"category,omitempty"`
	Code     string `json:"code,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Field    string `json:"field,omitempty"`
}

func (e *APIError) Error() string {
	if len(e.Errors) == 0 {
		return fmt.Sprintf("HTTP %d", e.StatusCode)
	}
	parts := make([]string, 0, len(e.Errors))
	for _, se := range e.Errors {
		msg := se.Code
		if se.Detail != "" {
			msg += ": " + se.Detail
		}
		if se.Field != "" {
			msg += " (field: " + se.Field + ")"
		}
		parts = append(parts, msg)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, strings.Join(parts, "; "))
}

func parseAPIError(status int, body []byte) *APIError {
	e := &APIError{StatusCode: status, Raw: body}
	var wrapper struct {
		Errors []SquareError `json:"errors"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil {
		e.Errors = wrapper.Errors
	}
	return e
}

// BodyReader is a convenience helper that wraps a byte slice as an io.Reader,
// returning nil when the slice is empty so no body is sent.
func BodyReader(b []byte) io.Reader {
	if len(b) == 0 {
		return nil
	}
	return bytes.NewReader(b)
}
