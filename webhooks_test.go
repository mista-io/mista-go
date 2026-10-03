package mista

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testSecret = "whsec_test"

var testNow = time.Unix(1790000000, 0)

const testEvent = `{"id":"evt_1","type":"message.failed","created_at":"2026-10-03T08:00:00+02:00","data":{"uid":"m1","to":"250780000001","from":"YourBrand","status":"Undelivered","status_detail":"Undelivered (handset unreachable)","cost":"1","sms_count":1,"campaign_uid":null,"sent_at":"2026-10-03T07:59:50+02:00","updated_at":"2026-10-03T08:00:00+02:00"}}`

func signWebhook(body string, t time.Time, secret string) string {
	ts := strconv.FormatInt(t.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + body))
	return "t=" + ts + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func fixedClock(c *verifyConfig) { c.now = func() time.Time { return testNow } }

func TestWebhooksEndpoints(t *testing.T) {
	reg := map[string]interface{}{"url": "https://example.com/hook", "secret": testSecret, "enabled": true, "events": []string{EventMessageDelivered, EventMessageFailed}}
	c, rec := newTestClient(t,
		ok(reg),
		ok(reg),
		ok(map[string]interface{}{"url": nil, "secret": nil, "enabled": false, "events": []string{}}),
		ok(map[string]interface{}{"delivered": false, "status_code": 500, "error": "Endpoint responded with HTTP 500", "event_id": "evt_t"}),
	)
	ctx := context.Background()

	got, err := c.Webhooks.Get(ctx)
	if err != nil || got.URL != "https://example.com/hook" || got.Secret != testSecret || !got.Enabled {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := c.Webhooks.Set(ctx, &SetWebhookParams{URL: "https://example.com/hook", RotateSecret: true}); err != nil {
		t.Fatal(err)
	}
	deleted, err := c.Webhooks.Delete(ctx)
	if err != nil || deleted.URL != "" || deleted.Enabled {
		t.Fatalf("Delete = %+v, %v", deleted, err)
	}
	result, err := c.Webhooks.Test(ctx)
	if err != nil || result.Delivered || result.StatusCode != 500 || result.EventID != "evt_t" {
		t.Fatalf("Test = %+v, %v", result, err)
	}

	want := [][2]string{
		{"GET", "/api/v3/webhooks/delivery-reports"},
		{"PUT", "/api/v3/webhooks/delivery-reports"},
		{"DELETE", "/api/v3/webhooks/delivery-reports"},
		{"POST", "/api/v3/webhooks/delivery-reports/test"},
	}
	for i, w := range want {
		if rec.calls[i].Method != w[0] || rec.calls[i].Path != w[1] {
			t.Fatalf("call %d = %s %s, want %s %s", i, rec.calls[i].Method, rec.calls[i].Path, w[0], w[1])
		}
	}
	jsonEqual(t, rec.calls[1].Body, map[string]interface{}{"url": "https://example.com/hook", "rotate_secret": true})
}

func TestWebhooksSetOmitsRotateSecretAndValidates(t *testing.T) {
	c, rec := newTestClient(t, ok(map[string]interface{}{}))
	if _, err := c.Webhooks.Set(context.Background(), &SetWebhookParams{URL: "https://example.com/hook"}); err != nil {
		t.Fatal(err)
	}
	jsonEqual(t, rec.calls[0].Body, map[string]interface{}{"url": "https://example.com/hook"})

	if _, err := c.Webhooks.Set(context.Background(), &SetWebhookParams{}); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("err = %v, want ErrInvalidParams", err)
	}
}

func TestVerifyWebhook(t *testing.T) {
	event, err := VerifyWebhook([]byte(testEvent), signWebhook(testEvent, testNow, testSecret), testSecret, fixedClock)
	if err != nil {
		t.Fatal(err)
	}
	if event.ID != "evt_1" || event.Type != EventMessageFailed || event.Data.Status != StatusUndelivered ||
		event.Data.StatusDetail != "Undelivered (handset unreachable)" || event.Data.SMSCount != 1 || !event.Data.Status.IsFinal() {
		t.Fatalf("event = %+v", event)
	}

	header := "t=" + strconv.FormatInt(testNow.Unix(), 10) + ",v1=" + strings.Repeat("0", 64) + "," +
		strings.Split(signWebhook(testEvent, testNow, testSecret), ",")[1]
	if _, err := VerifyWebhook([]byte(testEvent), header, testSecret, fixedClock); err != nil {
		t.Fatalf("second v1 entry: %v", err)
	}

	old := signWebhook(testEvent, time.Unix(1, 0), testSecret)
	if _, err := VerifyWebhook([]byte(testEvent), old, testSecret, fixedClock, WithTolerance(0)); err != nil {
		t.Fatalf("tolerance 0: %v", err)
	}
}

func TestVerifyWebhookRejects(t *testing.T) {
	valid := signWebhook(testEvent, testNow, testSecret)
	cases := map[string]struct {
		body, header, secret, msg string
	}{
		"tampered body":  {strings.Replace(testEvent, "Undelivered", "Delivered", 1), valid, testSecret, "does not match"},
		"wrong secret":   {testEvent, valid, "whsec_other", "does not match"},
		"missing header": {testEvent, "", testSecret, "missing Mista-Signature"},
		"malformed":      {testEvent, "v1=abc", testSecret, "malformed"},
		"too old":        {testEvent, signWebhook(testEvent, testNow.Add(-301*time.Second), testSecret), testSecret, "tolerance"},
		"empty secret":   {testEvent, valid, "", "missing webhook secret"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := VerifyWebhook([]byte(tc.body), tc.header, tc.secret, fixedClock)
			if !errors.Is(err, ErrWebhookSignature) || !strings.Contains(err.Error(), tc.msg) {
				t.Fatalf("err = %v, want ErrWebhookSignature containing %q", err, tc.msg)
			}
		})
	}
}

func TestMessageStatusDecodes(t *testing.T) {
	c, _ := newTestClient(t, ok(map[string]interface{}{"uid": "m1", "status": "Sent", "status_detail": nil}))
	msg, err := c.Logs.Get(context.Background(), "m1")
	if err != nil || msg.Status != StatusSent || msg.Status.IsFinal() || msg.StatusDetail != "" {
		t.Fatalf("msg = %+v, %v", msg, err)
	}
}
