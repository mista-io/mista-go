# Changelog

## v0.2.1

- Examples and README use masked US phone numbers (+1555***4567). No code changes.

## v0.2.0

- Delivery report webhooks: `Webhooks.Get`, `Webhooks.Set`, `Webhooks.Delete`, `Webhooks.Test`
- `VerifyWebhook` checks the `Mista-Signature` header and returns the decoded `*WebhookEvent`;
  failures wrap `ErrWebhookSignature`. Tolerance is configurable with `WithTolerance`.
- New `MessageStatus` type (`StatusQueued`, `StatusSent`, `StatusDelivered`, `StatusUndelivered`,
  `StatusExpired`, `StatusRejected`, `StatusFailed`) with `IsFinal()`.
- `SMSMessage.Status` is now a `MessageStatus` and `SMSMessage.StatusDetail` explains failures.
  The API no longer appends gateway references to the status (`Sent|ATXid_...` is now `Sent`).
- `ListMessagesParams.Status` is now a `MessageStatus`. String constants still compile
  (`Status: "Delivered"`); variables of type `string` need a conversion.

## v0.1.0

First release, covering the Mista API v3 as documented at https://docs.mista.io:

- SMS: `SMS.Send`
- Campaigns: `Campaigns.Bulk`, `Campaigns.SendToGroups`, `Campaigns.Get`
- Logs: `Logs.List`, `Logs.ListAutoPaging`, `Logs.Get`
- Account: `Account.Balance`, `Account.Me`
- Contact groups and contacts: list (plus auto-paging), create, get, update, delete
- Verify: `Verify.Start`, `Verify.Check`, `Verify.Get`
- Voice: `Voice.AccessToken`, `Voice.Numbers`, `Voice.Calls.List`, `Voice.Calls.ListAutoPaging`, `Voice.Calls.Get`
- `*APIError` with sentinel matching (`errors.Is(err, mista.ErrNotFound)`), retries for rate limits, context support
