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
		To:       "250780000001",
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
	To:       "250780000001",
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
	Recipients:   []string{"250780000001", "250780000002"},
	Message:      "Double points this weekend!",
	ScheduleTime: "2026-12-24 09:00", // or mista.FormatScheduleTime(t); account timezone
})

// Personalized: one message per number
client.Campaigns.Bulk(ctx, &mista.BulkCampaignParams{
	SenderID: "LOYALTY",
	Personalized: []mista.PersonalizedRecipient{
		{To: "250780000001", Message: "Hi Alice, you have 120 points."},
		{To: "250780000002", Message: "Hi Bob, you have 45 points."},
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
page, err := client.Logs.List(ctx, &mista.ListMessagesParams{StartDate: "2026-10-01", Status: "Delivered", PerPage: 50})
page.Items        // this page
page.Meta.Total   // total matches

it := client.Logs.ListAutoPaging(ctx, &mista.ListMessagesParams{Status: "Delivered"})
for it.Next(ctx) {
	msg := it.Current()
	fmt.Println(msg.UID, msg.Status)
}
if err := it.Err(); err != nil {
	log.Fatal(err)
}
```

Filters: `Page`, `PerPage`, `StartDate`, `EndDate` (`2006-01-02`), `SenderID`, `Status`, `SMSType`.
When nothing matches, you get an empty page.

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
	Phone:     "250780000001",
	FirstName: "Alice",
	LastName:  "Uwase",
	Fields:    map[string]string{"CITY": "Kigali"}, // custom fields, keyed by the group's field tag
})
client.Contacts.List(ctx, group.UID, 1) // or ListAutoPaging
client.Contacts.Get(ctx, group.UID, contact.UID)
client.Contacts.Update(ctx, group.UID, contact.UID, &mista.ContactFields{Phone: "250780000001", FirstName: "Alicia"})
client.Contacts.Delete(ctx, group.UID, contact.UID)

client.ContactGroups.Delete(ctx, group.UID) // also deletes its contacts
```

## Verify (OTP)

```go
v, err := client.Verify.Start(ctx, &mista.StartVerificationParams{To: "+250780000001", Channel: "sms"})

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
