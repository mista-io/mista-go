package mista

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

var ctx = context.Background()

func TestHeadersAndEnvelope(t *testing.T) {
	c, rec := newTestClient(t, ok(map[string]interface{}{"remaining_unit": "1,250", "expired_on": "x"}))
	b, err := c.Account.Balance(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if b.RemainingUnit != "1,250" || b.ExpiredOn != "x" {
		t.Fatalf("unexpected balance %+v", b)
	}
	h := rec.calls[0].Header
	if h.Get("Authorization") != "Bearer test-token" || h.Get("Accept") != "application/json" || h.Get("User-Agent") != "mista-go/"+Version {
		t.Fatalf("headers %v", h)
	}
	if h.Get("Content-Type") != "" {
		t.Fatal("GET should not send Content-Type")
	}
	if rec.calls[0].Path != "/api/v3/balance" {
		t.Fatal(rec.calls[0].Path)
	}
}

func TestDefaultBaseURL(t *testing.T) {
	if NewClient("x").baseURL != "https://api.mista.io" {
		t.Fatal("wrong default base URL")
	}
}

func TestTokenFromEnv(t *testing.T) {
	t.Setenv("MISTA_API_TOKEN", "")
	if _, err := NewClient("").Account.Me(ctx); !errors.Is(err, ErrMissingToken) {
		t.Fatalf("want ErrMissingToken, got %v", err)
	}
	t.Setenv("MISTA_API_TOKEN", "env-token")
	if NewClient("").token != "env-token" {
		t.Fatal("token not read from env")
	}
}

func TestStatusMapping(t *testing.T) {
	cases := []struct {
		status   int
		sentinel error
	}{
		{400, ErrBadRequest},
		{401, ErrUnauthorized},
		{403, ErrForbidden},
		{404, ErrNotFound},
		{422, ErrValidation},
		{500, ErrServer},
	}
	for _, tc := range cases {
		c, _ := newTestClient(t, errReply(tc.status, map[string]string{"status": "error", "message": "nope"}))
		c.maxRetries = 0
		_, err := c.Account.Me(ctx)
		if !errors.Is(err, tc.sentinel) {
			t.Fatalf("%d: want %v, got %v", tc.status, tc.sentinel, err)
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != tc.status || apiErr.Message != "nope" {
			t.Fatalf("%d: bad APIError %+v", tc.status, apiErr)
		}
	}
}

func TestOKWithErrorStatus(t *testing.T) {
	c, _ := newTestClient(t, reply{body: map[string]string{"status": "error", "message": "You have already subscribed to Developers"}})
	_, err := c.Contacts.Create(ctx, "g1", &ContactFields{Phone: "250780000001"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Message != "You have already subscribed to Developers" || apiErr.StatusCode != 200 {
		t.Fatalf("got %v", err)
	}
}

func TestFastAPIFieldErrors(t *testing.T) {
	body := map[string]interface{}{"detail": []map[string]interface{}{
		{"type": "missing", "loc": []string{"body", "sender_id"}, "msg": "Field required", "input": nil},
	}}
	c, _ := newTestClient(t, errReply(422, body))
	_, err := c.SMS.Send(ctx, &SendSMSParams{To: "250780000001", Message: "hi"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal(err)
	}
	if apiErr.Message != "sender_id: Field required" {
		t.Fatal(apiErr.Message)
	}
	if len(apiErr.Errors) != 1 || apiErr.Errors[0] != (FieldError{Field: "sender_id", Message: "Field required", Type: "missing"}) {
		t.Fatalf("%+v", apiErr.Errors)
	}
}

func TestLaravelFieldErrors(t *testing.T) {
	body := map[string]interface{}{"message": "The name field is required.", "errors": map[string][]string{"name": {"The name field is required."}}}
	c, _ := newTestClient(t, errReply(422, body))
	_, err := c.ContactGroups.Create(ctx, "")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || len(apiErr.Errors) != 1 || apiErr.Errors[0].Field != "name" {
		t.Fatalf("%v", err)
	}
}

func TestRetries429ForPost(t *testing.T) {
	limited := reply{status: 429, body: map[string]string{"message": "Too Many Attempts."}, headers: map[string]string{"Retry-After": "1"}}
	c, rec := newTestClient(t, limited, ok(map[string]string{"uid": "m1"}))
	msg, err := c.SMS.Send(ctx, &SendSMSParams{To: "250780000001", SenderID: "S", Message: "hi"})
	if err != nil || msg.UID != "m1" || len(rec.calls) != 2 {
		t.Fatalf("msg=%+v err=%v calls=%d", msg, err, len(rec.calls))
	}
}

func TestRateLimitAfterRetries(t *testing.T) {
	limited := reply{status: 429, body: map[string]string{"message": "Too Many Attempts."}, headers: map[string]string{"Retry-After": "7"}}
	c, rec := newTestClient(t, limited, limited, limited)
	_, err := c.Account.Me(ctx)
	var apiErr *APIError
	if !errors.Is(err, ErrRateLimited) || !errors.As(err, &apiErr) || apiErr.RetryAfter() != 7 || len(rec.calls) != 3 {
		t.Fatalf("err=%v calls=%d", err, len(rec.calls))
	}
}

func Test5xxRetriedForGetOnly(t *testing.T) {
	boom := errReply(503, map[string]string{"status": "error", "message": "down"})
	c, rec := newTestClient(t, boom, ok(map[string]string{}))
	if _, err := c.Account.Me(ctx); err != nil || len(rec.calls) != 2 {
		t.Fatalf("GET: err=%v calls=%d", err, len(rec.calls))
	}
	c, rec = newTestClient(t, boom, ok(map[string]string{}))
	if _, err := c.SMS.Send(ctx, &SendSMSParams{To: "250780000001", SenderID: "S", Message: "hi"}); !errors.Is(err, ErrServer) || len(rec.calls) != 1 {
		t.Fatalf("POST: err=%v calls=%d", err, len(rec.calls))
	}
}

func TestConnectionErrorsRetriedForGetOnly(t *testing.T) {
	c, rec := newTestClient(t, reply{hangup: true}, ok(map[string]string{}))
	if _, err := c.Account.Me(ctx); err != nil || rec.count() != 2 {
		t.Fatalf("GET: err=%v calls=%d", err, rec.count())
	}
	c, rec = newTestClient(t, reply{hangup: true})
	_, err := c.ContactGroups.Create(ctx, "x")
	var connErr *ConnectionError
	if !errors.As(err, &connErr) || rec.count() != 1 {
		t.Fatalf("POST: err=%v calls=%d", err, rec.count())
	}
}

func TestCustomHTTPClient(t *testing.T) {
	c, rec := newTestClient(t, ok(map[string]string{}))
	c2 := NewClient("t", WithBaseURL(c.baseURL), WithHTTPClient(&http.Client{}), WithMaxRetries(0))
	if _, err := c2.Account.Me(ctx); err != nil || len(rec.calls) != 1 {
		t.Fatalf("err=%v", err)
	}
}

func TestFlexTypes(t *testing.T) {
	c, _ := newTestClient(t,
		ok(map[string]interface{}{"uid": "c1", "phone": 250780000001, "status": "subscribe", "custom_fields": map[string]interface{}{"FIRST_NAME": "Alice", "AGE": 30, "NOTE": nil}}),
		ok(map[string]interface{}{"id": "x", "stats": map[string]interface{}{"delivered_count": "3", "failed_count": 0}}),
	)
	contact, err := c.Contacts.Get(ctx, "g1", "c1")
	if err != nil {
		t.Fatal(err)
	}
	if contact.Phone != "250780000001" || contact.CustomFields["AGE"] != "30" || contact.CustomFields["NOTE"] != "" {
		t.Fatalf("%+v", contact)
	}
	campaign, err := c.Campaigns.Get(ctx, "x")
	if err != nil || campaign.Stats["delivered_count"] != 3 {
		t.Fatalf("%+v %v", campaign, err)
	}
}
