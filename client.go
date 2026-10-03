// Package mista is the official Go client for the Mista Messaging, Verify and
// Voice APIs. API reference: https://docs.mista.io
//
//	client := mista.NewClient(os.Getenv("MISTA_API_TOKEN"))
//	msg, err := client.SMS.Send(ctx, &mista.SendSMSParams{
//		To: "250780000001", SenderID: "YourBrand", Message: "Hello",
//	})
package mista

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Version is the SDK version, sent in the User-Agent header.
const Version = "0.2.0"

// DefaultBaseURL is the production API host.
const DefaultBaseURL = "https://api.mista.io"

// Client talks to the Mista API. Create one with NewClient and reuse it; it is
// safe for concurrent use.
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
	maxRetries int
	sleep      func(context.Context, time.Duration) error

	SMS           *SMSService
	Campaigns     *CampaignsService
	Logs          *LogsService
	Account       *AccountService
	ContactGroups *ContactGroupsService
	Contacts      *ContactsService
	Verify        *VerifyService
	Voice         *VoiceService
	Webhooks      *WebhooksService
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API host (default https://api.mista.io).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient uses a custom *http.Client (proxies, transports, tracing).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

// WithTimeout sets the per-request timeout (default 30s). Applied to the
// client's *http.Client, so set it after WithHTTPClient.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		clone := *c.httpClient
		clone.Timeout = d
		c.httpClient = &clone
	}
}

// WithMaxRetries sets how often rate-limited requests (any method) and
// network/5xx failures (GET only) are retried. Default 2; 0 disables retries.
func WithMaxRetries(n int) Option {
	return func(c *Client) { c.maxRetries = n }
}

// NewClient returns a client. An empty token falls back to the
// MISTA_API_TOKEN environment variable.
func NewClient(token string, opts ...Option) *Client {
	if token == "" {
		token = os.Getenv("MISTA_API_TOKEN")
	}
	c := &Client{
		token:      token,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		maxRetries: 2,
		sleep:      sleepContext,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.SMS = &SMSService{c}
	c.Campaigns = &CampaignsService{c}
	c.Logs = &LogsService{c}
	c.Account = &AccountService{c}
	c.ContactGroups = &ContactGroupsService{c}
	c.Contacts = &ContactsService{c}
	c.Verify = &VerifyService{c}
	c.Voice = &VoiceService{client: c, Calls: &CallsService{c}}
	c.Webhooks = &WebhooksService{c}
	return c
}

type envelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// do sends a request and decodes the envelope's "data" into out (if non-nil).
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out interface{}) error {
	if c.token == "" {
		return ErrMissingToken
	}
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}
	u := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		u += "?" + encoded
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "mista-go/"+Version)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if method == http.MethodGet && attempt < c.maxRetries {
				if err := c.sleep(ctx, backoff(attempt)); err != nil {
					return err
				}
				continue
			}
			return &ConnectionError{Err: err}
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return &ConnectionError{Err: err}
		}

		retryable := resp.StatusCode == http.StatusTooManyRequests ||
			(resp.StatusCode >= 500 && method == http.MethodGet)
		if retryable && attempt < c.maxRetries {
			if err := c.sleep(ctx, retryDelay(attempt, resp.Header)); err != nil {
				return err
			}
			continue
		}
		return decode(resp, raw, out)
	}
}

func decode(resp *http.Response, raw []byte, out interface{}) error {
	var env envelope
	isEnvelope := json.Unmarshal(raw, &env) == nil && env.Status != ""
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || (isEnvelope && env.Status == "error") {
		return newAPIError(resp.StatusCode, raw, resp.Header)
	}
	if out == nil {
		return nil
	}
	data := json.RawMessage(raw)
	if isEnvelope {
		data = env.Data
	}
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, out)
}

func backoff(attempt int) time.Duration {
	base := 500 * time.Millisecond << attempt
	if base > 8*time.Second || base <= 0 {
		base = 8 * time.Second
	}
	return base/2 + time.Duration(rand.Int63n(int64(base/2)+1))
}

func retryDelay(attempt int, h http.Header) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(h.Get("Retry-After"))); err == nil && s > 0 {
		if s > 60 {
			s = 60
		}
		return time.Duration(s) * time.Second
	}
	return backoff(attempt)
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func seg(s string) string { return url.PathEscape(s) }

func setInt(q url.Values, key string, v int) {
	if v > 0 {
		q.Set(key, strconv.Itoa(v))
	}
}

func setString(q url.Values, key, v string) {
	if v != "" {
		q.Set(key, v)
	}
}
