# Changelog

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
