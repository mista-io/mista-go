package mista

import (
	"errors"
	"testing"
	"time"
)

type row = map[string]interface{}

func TestSMSSend(t *testing.T) {
	c, rec := newTestClient(t, ok(row{"uid": "m1", "status": "Queued", "cost": 1}))
	msg, err := c.SMS.Send(ctx, &SendSMSParams{To: "250780000001", SenderID: "YourBrand", Message: "Hello", Type: "plain"})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Cost != "1" {
		t.Fatalf("cost %q", msg.Cost)
	}
	got := rec.calls[0]
	if got.Method != "POST" || got.Path != "/api/v3/sms" || got.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("%+v", got)
	}
	jsonEqual(t, got.Body, row{"recipient": "250780000001", "sender_id": "YourBrand", "message": "Hello", "type": "plain"})
}

func TestSMSSendRejectsManyRecipients(t *testing.T) {
	c, rec := newTestClient(t)
	_, err := c.SMS.Send(ctx, &SendSMSParams{To: "250780000001,250780000002", SenderID: "S", Message: "x"})
	if !errors.Is(err, ErrInvalidParams) || len(rec.calls) != 0 {
		t.Fatalf("err=%v calls=%d", err, len(rec.calls))
	}
}

func TestBulkBroadcast(t *testing.T) {
	c, rec := newTestClient(t, ok(row{"uid": "c1", "mode": "broadcast", "recipient_count": 2}))
	res, err := c.Campaigns.Bulk(ctx, &BulkCampaignParams{
		SenderID:     "LOYALTY",
		Recipients:   []string{"250780000001", "250780000002"},
		Message:      "Double points!",
		ScheduleTime: FormatScheduleTime(time.Date(2026, 12, 24, 9, 5, 0, 0, time.UTC)),
	})
	if err != nil || res.RecipientCount != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if rec.calls[0].Path != "/api/v3/campaigns/bulk" {
		t.Fatal(rec.calls[0].Path)
	}
	jsonEqual(t, rec.calls[0].Body, row{
		"sender_id":     "LOYALTY",
		"recipients":    []string{"250780000001", "250780000002"},
		"message":       "Double points!",
		"schedule_time": "2026-12-24 09:05",
	})
}

func TestBulkPersonalized(t *testing.T) {
	c, rec := newTestClient(t, ok(row{"uid": "c1"}))
	_, err := c.Campaigns.Bulk(ctx, &BulkCampaignParams{
		SenderID:     "LOYALTY",
		Personalized: []PersonalizedRecipient{{To: "250780000001", Message: "Hi Alice"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonEqual(t, rec.calls[0].Body, row{"sender_id": "LOYALTY", "recipients": []row{{"to": "250780000001", "message": "Hi Alice"}}})
}

func TestBulkValidation(t *testing.T) {
	c, rec := newTestClient(t)
	bad := []*BulkCampaignParams{
		{SenderID: "S", Recipients: []string{"1"}, Personalized: []PersonalizedRecipient{{To: "2", Message: "x"}}, Message: "m"},
		{SenderID: "S", Recipients: []string{"250780000001"}},
		{SenderID: "S"},
		{SenderID: "S", Recipients: make([]string, MaxBulkRecipients+1), Message: "m"},
	}
	for i, p := range bad {
		if _, err := c.Campaigns.Bulk(ctx, p); !errors.Is(err, ErrInvalidParams) {
			t.Fatalf("case %d: %v", i, err)
		}
	}
	if len(rec.calls) != 0 {
		t.Fatal("validation should not call the API")
	}
}

func TestSendToGroups(t *testing.T) {
	c, rec := newTestClient(t, ok(row{"uid": "c2"}))
	if _, err := c.Campaigns.SendToGroups(ctx, &GroupCampaignParams{GroupUIDs: []string{"g1", "g2"}, SenderID: "S", Message: "Hi"}); err != nil {
		t.Fatal(err)
	}
	if rec.calls[0].Path != "/api/v3/sms/campaign" {
		t.Fatal(rec.calls[0].Path)
	}
	jsonEqual(t, rec.calls[0].Body, row{"contact_list_id": "g1,g2", "sender_id": "S", "message": "Hi"})
}

func TestCampaignGet(t *testing.T) {
	c, rec := newTestClient(t, ok(row{"id": "c1"}))
	if _, err := c.Campaigns.Get(ctx, "c1"); err != nil {
		t.Fatal(err)
	}
	if rec.calls[0].Method != "GET" || rec.calls[0].Path != "/api/v3/campaign/c1/view" {
		t.Fatalf("%+v", rec.calls[0])
	}
}

func TestLogsListAndAutoPaging(t *testing.T) {
	c, rec := newTestClient(t,
		ok(paginated([]row{{"uid": "a"}, {"uid": "b"}}, 1, 2)),
		ok(paginated([]row{{"uid": "a"}, {"uid": "b"}}, 1, 2)),
		ok(paginated([]row{{"uid": "c"}}, 2, 2)),
	)
	params := &ListMessagesParams{StartDate: "2026-10-01", Status: "Delivered", SenderID: "BRAND", PerPage: 2}
	page, err := c.Logs.List(ctx, params)
	if err != nil || len(page.Items) != 2 || !page.HasNextPage() {
		t.Fatalf("%+v %v", page, err)
	}
	jsonEqual(t, rec.calls[0].Query, map[string]string{"start_date": "2026-10-01", "status": "Delivered", "from": "BRAND", "per_page": "2"})

	var uids []string
	it := c.Logs.ListAutoPaging(ctx, params)
	for it.Next(ctx) {
		uids = append(uids, it.Current().UID)
	}
	if it.Err() != nil {
		t.Fatal(it.Err())
	}
	jsonEqual(t, uids, []string{"a", "b", "c"})
	if rec.calls[2].Query["page"] != "2" || rec.calls[2].Query["from"] != "BRAND" {
		t.Fatalf("%+v", rec.calls[2].Query)
	}
}

func TestLogsListEmptyOn404(t *testing.T) {
	c, _ := newTestClient(t, errReply(404, row{"status": "error", "message": "SMS Info not found"}))
	page, err := c.Logs.List(ctx, &ListMessagesParams{SenderID: "Nobody"})
	if err != nil || len(page.Items) != 0 || page.HasNextPage() {
		t.Fatalf("%+v %v", page, err)
	}
}

func TestIterStopsOnError(t *testing.T) {
	c, _ := newTestClient(t, errReply(401, row{"status": "error", "message": "Invalid credentials."}))
	it := c.ContactGroups.ListAutoPaging(ctx)
	if it.Next(ctx) || !errors.Is(it.Err(), ErrUnauthorized) {
		t.Fatalf("err=%v", it.Err())
	}
}

func TestLogsGetEscapesPath(t *testing.T) {
	c, rec := newTestClient(t, ok(row{"uid": "a/b"}))
	if _, err := c.Logs.Get(ctx, "a/b"); err != nil {
		t.Fatal(err)
	}
	if rec.calls[0].Raw != "/api/v3/log/a%2Fb" {
		t.Fatal(rec.calls[0].Raw)
	}
}

func TestContactGroups(t *testing.T) {
	c, rec := newTestClient(t,
		ok(paginated([]row{{"uid": "g1", "name": "Devs"}}, 1, 1)),
		ok(row{"uid": "g1", "name": "Devs"}),
		ok(row{"uid": "g1", "name": "Devs"}),
		ok(row{"uid": "g1", "name": "Devs KGL"}),
		ok(nil),
	)
	page, err := c.ContactGroups.List(ctx, 0)
	if err != nil || len(page.Items) != 1 || page.Items[0].Name != "Devs" {
		t.Fatalf("%+v %v", page, err)
	}
	mustOK(t, func() error { _, e := c.ContactGroups.Create(ctx, "Devs"); return e })
	mustOK(t, func() error { _, e := c.ContactGroups.Get(ctx, "g1"); return e })
	mustOK(t, func() error { _, e := c.ContactGroups.Update(ctx, "g1", "Devs KGL"); return e })
	mustOK(t, func() error { return c.ContactGroups.Delete(ctx, "g1") })

	var got []string
	for _, cl := range rec.calls {
		got = append(got, cl.Method+" "+cl.Path)
	}
	jsonEqual(t, got, []string{
		"GET /api/v3/contacts",
		"POST /api/v3/contacts",
		"POST /api/v3/contacts/g1/show",
		"PATCH /api/v3/contacts/g1",
		"DELETE /api/v3/contacts/g1",
	})
	jsonEqual(t, rec.calls[1].Body, row{"name": "Devs"})
	if len(rec.calls[4].RawBody) != 0 {
		t.Fatal("DELETE should have no body")
	}
}

func TestContactsUseFieldTags(t *testing.T) {
	c, rec := newTestClient(t, ok(row{"uid": "c1"}), ok(row{"uid": "c1"}))
	mustOK(t, func() error {
		_, e := c.Contacts.Create(ctx, "g1", &ContactFields{Phone: "250780000001", FirstName: "Alice", LastName: "Uwase", Fields: map[string]string{"CITY": "Kigali"}})
		return e
	})
	mustOK(t, func() error {
		_, e := c.Contacts.Update(ctx, "g1", "c1", &ContactFields{Phone: "250780000001", FirstName: "Alicia"})
		return e
	})
	if rec.calls[0].Path != "/api/v3/contacts/g1/store" || rec.calls[1].Method != "PATCH" || rec.calls[1].Path != "/api/v3/contacts/g1/update/c1" {
		t.Fatalf("%+v", rec.calls)
	}
	jsonEqual(t, rec.calls[0].Body, row{"CITY": "Kigali", "PHONE": "250780000001", "FIRST_NAME": "Alice", "LAST_NAME": "Uwase"})
	jsonEqual(t, rec.calls[1].Body, row{"PHONE": "250780000001", "FIRST_NAME": "Alicia"})
}

func TestContactsListGetDelete(t *testing.T) {
	c, rec := newTestClient(t, ok(paginated([]row{{"uid": "c1", "phone": 250780000001}}, 1, 1)), ok(row{"uid": "c1"}), ok(nil))
	page, err := c.Contacts.List(ctx, "g1", 3)
	if err != nil || page.Items[0].Phone != "250780000001" {
		t.Fatalf("%+v %v", page, err)
	}
	mustOK(t, func() error { _, e := c.Contacts.Get(ctx, "g1", "c1"); return e })
	mustOK(t, func() error { return c.Contacts.Delete(ctx, "g1", "c1") })
	var got []string
	for _, cl := range rec.calls {
		got = append(got, cl.Method+" "+cl.Raw)
	}
	jsonEqual(t, got, []string{
		"GET /api/v3/contacts/g1/all?page=3",
		"POST /api/v3/contacts/g1/search/c1",
		"DELETE /api/v3/contacts/g1/delete/c1",
	})
}

func TestVerifyStart(t *testing.T) {
	c, rec := newTestClient(t, ok(row{"sid": "v1", "status": "pending"}))
	v, err := c.Verify.Start(ctx, &StartVerificationParams{To: "+250780000001", Channel: "sms", SenderID: "MISTA"})
	if err != nil || v.SID != "v1" {
		t.Fatalf("%+v %v", v, err)
	}
	if rec.calls[0].Path != "/api/v3/verify" {
		t.Fatal(rec.calls[0].Path)
	}
	jsonEqual(t, rec.calls[0].Body, row{"to": "+250780000001", "channel": "sms", "sender_id": "MISTA"})
}

func TestVerifyCheck(t *testing.T) {
	c, rec := newTestClient(t,
		ok(row{"sid": "v1", "status": "approved", "verified_at": "t"}),
		errReply(422, row{"status": "error", "message": "Verification failed", "data": row{"sid": "v1", "status": "pending", "reason": "invalid_code"}}),
		errReply(422, row{"status": "error", "message": "Verification not found."}),
	)
	res, err := c.Verify.Check(ctx, "v1", "123456")
	if err != nil || !res.Verified || res.Status != "approved" {
		t.Fatalf("%+v %v", res, err)
	}
	jsonEqual(t, rec.calls[0].Body, row{"sid": "v1", "code": "123456"})

	res, err = c.Verify.Check(ctx, "v1", "000000")
	if err != nil || res.Verified || res.Reason != "invalid_code" {
		t.Fatalf("%+v %v", res, err)
	}

	if _, err = c.Verify.Check(ctx, "nope", "1"); !errors.Is(err, ErrValidation) {
		t.Fatalf("want validation error, got %v", err)
	}
}

func TestVerifyGet(t *testing.T) {
	c, rec := newTestClient(t, ok(row{"sid": "v1"}))
	mustOK(t, func() error { _, e := c.Verify.Get(ctx, "v1"); return e })
	if rec.calls[0].Path != "/api/v3/verify/v1" {
		t.Fatal(rec.calls[0].Path)
	}
}

func TestVoice(t *testing.T) {
	c, rec := newTestClient(t,
		ok(row{"token": "jwt"}),
		ok([]row{{"uid": "n1", "number": "+250780000001"}}),
		ok(row{"items": []row{{"uid": "call1", "duration": nil}}, "pagination": row{"current_page": 1, "per_page": 20, "total": 1, "last_page": 1}}),
		ok(row{"uid": "call1"}),
	)
	tok, err := c.Voice.AccessToken(ctx, "ios")
	if err != nil || tok.Token != "jwt" {
		t.Fatalf("%+v %v", tok, err)
	}
	nums, err := c.Voice.Numbers(ctx)
	if err != nil || len(nums) != 1 {
		t.Fatalf("%+v %v", nums, err)
	}
	page, err := c.Voice.Calls.List(ctx, &ListCallsParams{Filter: "missed", PerPage: 20})
	if err != nil || len(page.Items) != 1 || page.Meta != (PageMeta{CurrentPage: 1, LastPage: 1, PerPage: 20, Total: 1}) {
		t.Fatalf("%+v %v", page, err)
	}
	mustOK(t, func() error { _, e := c.Voice.Calls.Get(ctx, "call1"); return e })
	var got []string
	for _, cl := range rec.calls {
		got = append(got, cl.Method+" "+cl.Raw)
	}
	jsonEqual(t, got, []string{
		"GET /api/v3/voice/access-token?platform=ios",
		"GET /api/v3/voice/numbers",
		"GET /api/v3/voice/calls?filter=missed&per_page=20",
		"GET /api/v3/voice/calls/call1",
	})
}

func mustOK(t *testing.T, f func() error) {
	t.Helper()
	if err := f(); err != nil {
		t.Fatal(err)
	}
}
