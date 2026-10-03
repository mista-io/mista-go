package mista

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// WebhookSignatureHeader is the header that carries the webhook signature.
const WebhookSignatureHeader = "Mista-Signature"

// DefaultWebhookTolerance is the default maximum age of a webhook.
const DefaultWebhookTolerance = 5 * time.Minute

// ErrWebhookSignature is returned (wrapped) by VerifyWebhook when a request was
// not signed by Mista with the given secret.
var ErrWebhookSignature = errors.New("mista: invalid webhook signature")

const webhooksPath = "/api/v3/webhooks/delivery-reports"

// WebhooksService manages the delivery report webhook: Mista POSTs to your URL
// when a message is delivered or fails.
type WebhooksService struct{ client *Client }

// Get returns the current registration. URL is empty when no webhook is set.
func (s *WebhooksService) Get(ctx context.Context) (*DeliveryReportWebhook, error) {
	return call[DeliveryReportWebhook](ctx, s.client, http.MethodGet, webhooksPath, nil, nil)
}

// Set registers or changes the webhook URL. The result includes the signing secret.
func (s *WebhooksService) Set(ctx context.Context, p *SetWebhookParams) (*DeliveryReportWebhook, error) {
	if p == nil || p.URL == "" {
		return nil, fmt.Errorf("%w: URL is required", ErrInvalidParams)
	}
	return call[DeliveryReportWebhook](ctx, s.client, http.MethodPut, webhooksPath, nil, p)
}

// Delete stops delivery reports and forgets the secret.
func (s *WebhooksService) Delete(ctx context.Context) (*DeliveryReportWebhook, error) {
	return call[DeliveryReportWebhook](ctx, s.client, http.MethodDelete, webhooksPath, nil, nil)
}

// Test sends a signed webhook.test event to the registered URL now and reports how it answered.
func (s *WebhooksService) Test(ctx context.Context) (*WebhookTestResult, error) {
	return call[WebhookTestResult](ctx, s.client, http.MethodPost, webhooksPath+"/test", nil, nil)
}

// VerifyOption configures VerifyWebhook.
type VerifyOption func(*verifyConfig)

type verifyConfig struct {
	tolerance time.Duration
	now       func() time.Time
}

// WithTolerance sets the maximum age of a webhook (default 5 minutes). 0 disables the check.
func WithTolerance(d time.Duration) VerifyOption {
	return func(c *verifyConfig) { c.tolerance = d }
}

// VerifyWebhook checks the Mista-Signature header of a delivery report webhook
// and returns the decoded event. Pass the raw request body exactly as received.
// Errors from a bad or missing signature wrap ErrWebhookSignature.
//
//	body, _ := io.ReadAll(r.Body)
//	event, err := mista.VerifyWebhook(body, r.Header.Get(mista.WebhookSignatureHeader), secret)
func VerifyWebhook(payload []byte, signatureHeader, secret string, opts ...VerifyOption) (*WebhookEvent, error) {
	cfg := verifyConfig{tolerance: DefaultWebhookTolerance, now: time.Now}
	for _, opt := range opts {
		opt(&cfg)
	}
	if secret == "" {
		return nil, fmt.Errorf("%w: missing webhook secret", ErrWebhookSignature)
	}
	if signatureHeader == "" {
		return nil, fmt.Errorf("%w: missing %s header", ErrWebhookSignature, WebhookSignatureHeader)
	}

	var timestamp int64 = -1
	var signatures []string
	for _, part := range strings.Split(signatureHeader, ",") {
		key, value, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch key {
		case "t":
			if t, err := strconv.ParseInt(value, 10, 64); err == nil {
				timestamp = t
			}
		case "v1":
			if value != "" {
				signatures = append(signatures, value)
			}
		}
	}
	if timestamp < 0 || len(signatures) == 0 {
		return nil, fmt.Errorf("%w: malformed %s header", ErrWebhookSignature, WebhookSignatureHeader)
	}

	if cfg.tolerance > 0 {
		age := cfg.now().Sub(time.Unix(timestamp, 0))
		if age > cfg.tolerance || age < -cfg.tolerance {
			return nil, fmt.Errorf("%w: timestamp is outside the tolerance window", ErrWebhookSignature)
		}
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "."))
	mac.Write(payload)
	expected := []byte(hex.EncodeToString(mac.Sum(nil)))
	matched := false
	for _, signature := range signatures {
		if hmac.Equal(expected, []byte(signature)) {
			matched = true
			break
		}
	}
	if !matched {
		return nil, fmt.Errorf("%w: signature does not match", ErrWebhookSignature)
	}

	event := new(WebhookEvent)
	if err := json.Unmarshal(payload, event); err != nil {
		return nil, fmt.Errorf("mista: webhook body is not valid JSON: %w", err)
	}
	return event, nil
}
