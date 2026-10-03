# Mista Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/mista-io/mista-go.svg)](https://pkg.go.dev/github.com/mista-io/mista-go)

Official Go client for the [Mista](https://mista.io) Messaging, Verify and Voice APIs.
Full API reference: https://docs.mista.io

```bash
go get github.com/mista-io/mista-go
```

Requires Go 1.22+. Standard library only.

## Quickstart

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mista-io/mista-go"
)

func main() {
	ctx := context.Background()
	client := mista.NewClient("YOUR_API_TOKEN") // "" reads MISTA_API_TOKEN

	msg, err := client.SMS.Send(ctx, &mista.SendSMSParams{
		To:       "+1555***4567",
		SenderID: "YourBrand",
		Message:  "Your order has shipped",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(msg.UID, msg.Status)
}
```

Get your API token in the dashboard under **Settings → API**. Create one `Client` and reuse it;
it is safe for concurrent use. Every method takes a `context.Context` first.

## SMS

`SMS.Send` sends one message to **one** recipient. To reach several numbers, or to schedule a
send, use a campaign.

```go
msg, err := client.SMS.Send(ctx, &mista.SendSMSParams{
	To:       "+1555***4567",
	SenderID: "YourBrand",
	Message:  "Hello",
	Type:     "plain", // plain | unicode | voice | mms | whatsapp | viber | otp
})
latest, err := client.Logs.Get(ctx, msg.UID) // delivery status
```

## Campaigns

```go
// Broadcast: one message, up to 10,000 numbers
client.Campaigns.Bulk(ctx, &mista.BulkCampaignParams{
	SenderID:     "LOYALTY",
	Recipients:   []string{"+1555***4567", "+1555***7890"},
	Message:      "Double points this weekend!",
	ScheduleTime: "2026-12-24 09:00", // or mista.FormatScheduleTime(t); account timezone
})

// Personalized: one message per number
client.Campaigns.Bulk(ctx, &mista.BulkCampaignParams{
	SenderID: "LOYALTY",
	Personalized: []mista.PersonalizedRecipient{
		{To: "+1555***4567", Message: "Hi Alice, you have 120 points."},
		{To: "+1555***7890", Message: "Hi Bob, you have 45 points."},
	},
})

// Everyone in one or more contact groups
client.Campaigns.SendToGroups(ctx, &mista.GroupCampaignParams{
	GroupUIDs: []string{"grp_uid"}, SenderID: "YourBrand", Message: "Hi!",
})

campaign, err := client.Campaigns.Get(ctx, "campaign_uid")
```

## Message logs

```go
page, err := client.Logs.List(ctx, &mista.ListMessagesParams{StartDate: "2026-10-01", Status: mista.StatusDelivered, PerPage: 50})
page.Items        // this page
page.Meta.Total   // total matches

it := client.Logs.ListAutoPaging(ctx, &mista.ListMessagesParams{Status: mista.StatusFailed})
for it.Next(ctx) {
	msg := it.Current()
	fmt.Println(msg.UID, msg.Status, msg.StatusDetail)
}
if err := it.Err(); err != nil {
	log.Fatal(err)
}
```

Filters: `Page`, `PerPage`, `StartDate`, `EndDate` (`2006-01-02`), `SenderID`, `Status`, `SMSType`.
When nothing matches, you get an empty page.

`Status` is a `mista.MessageStatus`:

| Status | Meaning |
| --- | --- |
| `StatusQueued` | Not yet accepted by the carrier |
| `StatusSent` | Accepted by the carrier, waiting for the handset delivery report |
| `StatusDelivered` | Delivered to the handset (final) |
| `StatusUndelivered` | The carrier could not deliver it (final) |
| `StatusExpired` | Not delivered before the carrier gave up (final) |
| `StatusRejected` | Rejected by the carrier or blocked (final) |
| `StatusFailed` | Could not be sent (final) |

`status.IsFinal()` tells you whether it can still change. For the last four, `StatusDetail` says why.

## Delivery report webhooks

Instead of polling `Logs.Get`, register a URL and Mista POSTs to it when a message is delivered
(`message.delivered`) or fails (`message.failed`: Undelivered, Expired, Rejected or Failed).

```go
hook, err := client.Webhooks.Set(ctx, &mista.SetWebhookParams{URL: "https://example.com/mista/dlr"})
secret := hook.Secret // whsec_..., store it to verify requests

client.Webhooks.Test(ctx)   // sends a signed webhook.test event now
client.Webhooks.Get(ctx)
client.Webhooks.Set(ctx, &mista.SetWebhookParams{URL: hook.URL, RotateSecret: true})
client.Webhooks.Delete(ctx)
```

You can also register it in the dashboard under **Developers → Settings**.

Verify every request with the raw body before trusting it:

```go
http.HandleFunc("/mista/dlr", func(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	event, err := mista.VerifyWebhook(body, r.Header.Get(mista.WebhookSignatureHeader), secret)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}
	switch event.Type {
	case mista.EventMessageDelivered:
		markDelivered(event.Data.UID)
	case mista.EventMessageFailed:
		markFailed(event.Data.UID, event.Data.Status, event.Data.StatusDetail)
	}
	w.WriteHeader(http.StatusNoContent)
})
```

Answer with any 2xx within 10 seconds. Otherwise Mista retries 5 more times over about 3 hours
(30s, 2m, 10m, 30m, 2h). `event.ID` is the same on every retry, so use it to skip duplicates.
`VerifyWebhook` rejects signatures older than 5 minutes; change that with `mista.WithTolerance`.
See [`examples/webhook-server`](examples/webhook-server/main.go).

## Account

```go
balance, err := client.Account.Balance(ctx) // RemainingUnit is "Unlimited" on unlimited plans
me, err := client.Account.Me(ctx)
```

## Contact groups and contacts

```go
group, err := client.ContactGroups.Create(ctx, "Developers")
client.ContactGroups.List(ctx, 1)
client.ContactGroups.Get(ctx, group.UID)
client.ContactGroups.Update(ctx, group.UID, "Developers KGL")

contact, err := client.Contacts.Create(ctx, group.UID, &mista.ContactFields{
	Phone:     "+1555***4567",
	FirstName: "Alice",
	LastName:  "Uwase",
	Fields:    map[string]string{"CITY": "Kigali"}, // custom fields, keyed by the group's field tag
})
client.Contacts.List(ctx, group.UID, 1) // or ListAutoPaging
client.Contacts.Get(ctx, group.UID, contact.UID)
client.Contacts.Update(ctx, group.UID, contact.UID, &mista.ContactFields{Phone: "+1555***4567", FirstName: "Alicia"})
client.Contacts.Delete(ctx, group.UID, contact.UID)

client.ContactGroups.Delete(ctx, group.UID) // also deletes its contacts
```

## Verify (OTP)

```go
v, err := client.Verify.Start(ctx, &mista.StartVerificationParams{To: "+1555***4567", Channel: "sms"})

result, err := client.Verify.Check(ctx, v.SID, "123456")
if err != nil {
	log.Fatal(err)
}
if result.Verified {
	// signed in
} else {
	fmt.Println(result.Reason) // e.g. "invalid_code"; a wrong code is not an error
}

client.Verify.Get(ctx, v.SID)
```

## Voice

```go
token, err := client.Voice.AccessToken(ctx, "ios")
numbers, err := client.Voice.Numbers(ctx)
calls, err := client.Voice.Calls.List(ctx, &mista.ListCallsParams{Filter: "missed", PerPage: 20})
call, err := client.Voice.Calls.Get(ctx, "call_uid")
```

## Errors

API failures are returned as `*mista.APIError` (status code, message, field errors, raw body and
headers). Match the kind with `errors.Is`:

| Sentinel | When |
| --- | --- |
| `ErrBadRequest` | 400, e.g. an invalid phone number |
| `ErrUnauthorized` | 401, missing or wrong token |
| `ErrForbidden` | 403 |
| `ErrNotFound` | 404 |
| `ErrValidation` | 422; field problems are in `apiErr.Errors` |
| `ErrRateLimited` | 429 after retries; see `apiErr.RetryAfter()` |
| `ErrServer` | 5xx |

A 200 whose body says `"status": "error"` (for example a contact already in the group) is also
returned as an `*APIError`. Network failures and timeouts are `*mista.ConnectionError`;
client-side validation failures wrap `mista.ErrInvalidParams` and send no request.

```go
_, err := client.SMS.Send(ctx, params)
var apiErr *mista.APIError
if errors.As(err, &apiErr) && errors.Is(err, mista.ErrValidation) {
	for _, fe := range apiErr.Errors {
		fmt.Println(fe.Field, fe.Message)
	}
}
```

## Retries, timeouts and HTTP client

```go
client := mista.NewClient(token,
	mista.WithHTTPClient(myHTTPClient), // proxies, custom transports
	mista.WithTimeout(10*time.Second),  // default 30s
	mista.WithMaxRetries(2),            // default 2; 0 disables
)
```

- `429 Too Many Requests` is retried for every request, waiting for `Retry-After`.
- Network errors and 5xx responses are retried for `GET` only, so a send is never duplicated.
- Cancelling the context stops the request and any pending retry.

Numbers the API may send as either a string or a number (`cost`, `phone`, `remaining_unit`)
are decoded into `mista.FlexString`.

## Not covered

Delivery-report webhooks (configure those in the dashboard) and the retired Push API.

## Development

```bash
go vet ./... && go test -race ./...
MISTA_API_TOKEN=... go run ./scripts/smoke   # read-only: balance + account
```

## License

MIT
