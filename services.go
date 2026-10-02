package mista

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// MaxBulkRecipients is the most numbers one bulk campaign request accepts.
const MaxBulkRecipients = 10000

// call sends a request and decodes the envelope's data into a new T.
func call[T any](ctx context.Context, c *Client, method, path string, query url.Values, body interface{}) (*T, error) {
	out := new(T)
	if err := c.do(ctx, method, path, query, body, out); err != nil {
		return nil, err
	}
	return out, nil
}

func compact(m map[string]interface{}) map[string]interface{} {
	for k, v := range m {
		if s, ok := v.(string); ok && s == "" {
			delete(m, k)
		}
	}
	return m
}

// ------------------------------------------------------------------ SMS

// SMSService sends single messages.
type SMSService struct{ client *Client }

// Send sends one SMS to one recipient. It is queued immediately; read its
// delivery status later with Logs.Get. For several numbers or a scheduled send,
// use Campaigns.Bulk.
func (s *SMSService) Send(ctx context.Context, p *SendSMSParams) (*SMSMessage, error) {
	if p == nil || p.To == "" {
		return nil, fmt.Errorf("%w: To is required", ErrInvalidParams)
	}
	if strings.Contains(p.To, ",") {
		return nil, fmt.Errorf("%w: SMS.Send takes one recipient; use Campaigns.Bulk for several numbers", ErrInvalidParams)
	}
	return call[SMSMessage](ctx, s.client, http.MethodPost, "/api/v3/sms", nil, p)
}

// ------------------------------------------------------------ Campaigns

// CampaignsService sends and inspects campaigns.
type CampaignsService struct{ client *Client }

// Bulk sends a broadcast (Recipients + Message) or personalized (Personalized)
// campaign to up to 10,000 numbers.
func (s *CampaignsService) Bulk(ctx context.Context, p *BulkCampaignParams) (*BulkCampaignResult, error) {
	if p == nil {
		return nil, fmt.Errorf("%w: params are required", ErrInvalidParams)
	}
	var recipients interface{}
	message := ""
	switch {
	case len(p.Recipients) > 0 && len(p.Personalized) > 0:
		return nil, fmt.Errorf("%w: set either Recipients or Personalized, not both", ErrInvalidParams)
	case len(p.Recipients) > 0:
		if p.Message == "" {
			return nil, fmt.Errorf("%w: Message is required with Recipients", ErrInvalidParams)
		}
		recipients, message = p.Recipients, p.Message
	case len(p.Personalized) > 0:
		recipients = p.Personalized
	default:
		return nil, fmt.Errorf("%w: recipients must not be empty", ErrInvalidParams)
	}
	if len(p.Recipients)+len(p.Personalized) > MaxBulkRecipients {
		return nil, fmt.Errorf("%w: at most %d recipients per request", ErrInvalidParams, MaxBulkRecipients)
	}
	body := compact(map[string]interface{}{
		"sender_id":       p.SenderID,
		"recipients":      recipients,
		"message":         message,
		"type":            p.Type,
		"name":            p.Name,
		"schedule_time":   p.ScheduleTime,
		"dlt_template_id": p.DLTTemplateID,
	})
	return call[BulkCampaignResult](ctx, s.client, http.MethodPost, "/api/v3/campaigns/bulk", nil, body)
}

// SendToGroups sends one message to every subscribed contact in the groups.
func (s *CampaignsService) SendToGroups(ctx context.Context, p *GroupCampaignParams) (*GroupCampaignResult, error) {
	if p == nil || len(p.GroupUIDs) == 0 {
		return nil, fmt.Errorf("%w: GroupUIDs must not be empty", ErrInvalidParams)
	}
	body := compact(map[string]interface{}{
		"contact_list_id": strings.Join(p.GroupUIDs, ","),
		"sender_id":       p.SenderID,
		"message":         p.Message,
		"type":            p.Type,
		"schedule_time":   p.ScheduleTime,
		"dlt_template_id": p.DLTTemplateID,
	})
	return call[GroupCampaignResult](ctx, s.client, http.MethodPost, "/api/v3/sms/campaign", nil, body)
}

// Get returns a campaign with its delivery counters.
func (s *CampaignsService) Get(ctx context.Context, uid string) (*Campaign, error) {
	return call[Campaign](ctx, s.client, http.MethodGet, "/api/v3/campaign/"+seg(uid)+"/view", nil, nil)
}

// ----------------------------------------------------------------- Logs

// LogsService reads sent messages.
type LogsService struct{ client *Client }

// List returns one page of messages, newest first. When nothing matches the
// filters it returns an empty page (the API itself answers 404).
func (s *LogsService) List(ctx context.Context, p *ListMessagesParams) (*Page[SMSMessage], error) {
	if p == nil {
		p = &ListMessagesParams{}
	}
	q := url.Values{}
	setInt(q, "page", p.Page)
	setInt(q, "per_page", p.PerPage)
	setString(q, "start_date", p.StartDate)
	setString(q, "end_date", p.EndDate)
	setString(q, "from", p.SenderID)
	setString(q, "status", p.Status)
	setString(q, "sms_type", p.SMSType)
	out := new(Page[SMSMessage])
	err := s.client.do(ctx, http.MethodGet, "/api/v3/log/view", q, nil, out)
	if errors.Is(err, ErrNotFound) {
		return emptyPage[SMSMessage](), nil
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListAutoPaging iterates every matching message across all pages.
func (s *LogsService) ListAutoPaging(ctx context.Context, p *ListMessagesParams) *Iter[SMSMessage] {
	params := ListMessagesParams{}
	if p != nil {
		params = *p
	}
	return newIter(params.Page, func(ctx context.Context, page int) (*Page[SMSMessage], error) {
		params.Page = page
		return s.List(ctx, &params)
	})
}

// Get returns one message and its current delivery status.
func (s *LogsService) Get(ctx context.Context, uid string) (*SMSMessage, error) {
	return call[SMSMessage](ctx, s.client, http.MethodGet, "/api/v3/log/"+seg(uid), nil, nil)
}

// -------------------------------------------------------------- Account

// AccountService reads account details.
type AccountService struct{ client *Client }

// Balance returns the SMS units left and when the plan expires.
func (s *AccountService) Balance(ctx context.Context) (*Balance, error) {
	return call[Balance](ctx, s.client, http.MethodGet, "/api/v3/balance", nil, nil)
}

// Me returns the profile that owns the API token.
func (s *AccountService) Me(ctx context.Context) (*Account, error) {
	return call[Account](ctx, s.client, http.MethodGet, "/api/v3/account/me", nil, nil)
}

// ------------------------------------------------------- Contact groups

// ContactGroupsService manages contact groups (phonebooks).
type ContactGroupsService struct{ client *Client }

// List returns one page of contact groups (page 0 means the first page).
func (s *ContactGroupsService) List(ctx context.Context, page int) (*Page[ContactGroup], error) {
	q := url.Values{}
	setInt(q, "page", page)
	return call[Page[ContactGroup]](ctx, s.client, http.MethodGet, "/api/v3/contacts", q, nil)
}

// ListAutoPaging iterates every contact group.
func (s *ContactGroupsService) ListAutoPaging(ctx context.Context) *Iter[ContactGroup] {
	return newIter(1, func(ctx context.Context, page int) (*Page[ContactGroup], error) {
		return s.List(ctx, page)
	})
}

func (s *ContactGroupsService) Create(ctx context.Context, name string) (*ContactGroup, error) {
	return call[ContactGroup](ctx, s.client, http.MethodPost, "/api/v3/contacts", nil, map[string]string{"name": name})
}

func (s *ContactGroupsService) Get(ctx context.Context, groupUID string) (*ContactGroup, error) {
	return call[ContactGroup](ctx, s.client, http.MethodPost, "/api/v3/contacts/"+seg(groupUID)+"/show", nil, nil)
}

func (s *ContactGroupsService) Update(ctx context.Context, groupUID, name string) (*ContactGroup, error) {
	return call[ContactGroup](ctx, s.client, http.MethodPatch, "/api/v3/contacts/"+seg(groupUID), nil, map[string]string{"name": name})
}

// Delete removes the group and every contact in it.
func (s *ContactGroupsService) Delete(ctx context.Context, groupUID string) error {
	return s.client.do(ctx, http.MethodDelete, "/api/v3/contacts/"+seg(groupUID), nil, nil, nil)
}

// ------------------------------------------------------------- Contacts

// ContactsService manages contacts inside a group.
type ContactsService struct{ client *Client }

func contactBody(f *ContactFields) (map[string]string, error) {
	if f == nil || f.Phone == "" {
		return nil, fmt.Errorf("%w: Phone is required", ErrInvalidParams)
	}
	body := map[string]string{}
	for k, v := range f.Fields {
		body[k] = v
	}
	body["PHONE"] = f.Phone
	if f.FirstName != "" {
		body["FIRST_NAME"] = f.FirstName
	}
	if f.LastName != "" {
		body["LAST_NAME"] = f.LastName
	}
	return body, nil
}

// Create adds a contact. It returns an *APIError if the phone is already in the group.
func (s *ContactsService) Create(ctx context.Context, groupUID string, f *ContactFields) (*Contact, error) {
	body, err := contactBody(f)
	if err != nil {
		return nil, err
	}
	return call[Contact](ctx, s.client, http.MethodPost, "/api/v3/contacts/"+seg(groupUID)+"/store", nil, body)
}

// List returns one page of a group's contacts (page 0 means the first page).
func (s *ContactsService) List(ctx context.Context, groupUID string, page int) (*Page[ContactListItem], error) {
	q := url.Values{}
	setInt(q, "page", page)
	return call[Page[ContactListItem]](ctx, s.client, http.MethodGet, "/api/v3/contacts/"+seg(groupUID)+"/all", q, nil)
}

// ListAutoPaging iterates every contact in the group.
func (s *ContactsService) ListAutoPaging(ctx context.Context, groupUID string) *Iter[ContactListItem] {
	return newIter(1, func(ctx context.Context, page int) (*Page[ContactListItem], error) {
		return s.List(ctx, groupUID, page)
	})
}

func (s *ContactsService) Get(ctx context.Context, groupUID, uid string) (*Contact, error) {
	return call[Contact](ctx, s.client, http.MethodPost, "/api/v3/contacts/"+seg(groupUID)+"/search/"+seg(uid), nil, nil)
}

// Update changes a contact. Phone is always required; empty fields keep their value.
func (s *ContactsService) Update(ctx context.Context, groupUID, uid string, f *ContactFields) (*Contact, error) {
	body, err := contactBody(f)
	if err != nil {
		return nil, err
	}
	return call[Contact](ctx, s.client, http.MethodPatch, "/api/v3/contacts/"+seg(groupUID)+"/update/"+seg(uid), nil, body)
}

func (s *ContactsService) Delete(ctx context.Context, groupUID, uid string) error {
	return s.client.do(ctx, http.MethodDelete, "/api/v3/contacts/"+seg(groupUID)+"/delete/"+seg(uid), nil, nil, nil)
}

// --------------------------------------------------------------- Verify

// VerifyService sends and checks one-time codes.
type VerifyService struct{ client *Client }

// Start sends a code. Keep the returned SID for Check.
func (s *VerifyService) Start(ctx context.Context, p *StartVerificationParams) (*Verification, error) {
	if p == nil || p.To == "" {
		return nil, fmt.Errorf("%w: To is required", ErrInvalidParams)
	}
	return call[Verification](ctx, s.client, http.MethodPost, "/api/v3/verify", nil, p)
}

// Check checks a code. A wrong or expired code returns Verified=false with a
// Reason and a nil error.
func (s *VerifyService) Check(ctx context.Context, sid, code string) (*VerificationCheck, error) {
	out := new(VerificationCheck)
	err := s.client.do(ctx, http.MethodPost, "/api/v3/verify/check", nil, map[string]string{"sid": sid, "code": code}, out)
	if err == nil {
		out.Verified = true
		return out, nil
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnprocessableEntity {
		var env struct {
			Data *VerificationCheck `json:"data"`
		}
		if json.Unmarshal(apiErr.Body, &env) == nil && env.Data != nil && env.Data.Reason != "" {
			return env.Data, nil
		}
	}
	return nil, err
}

func (s *VerifyService) Get(ctx context.Context, sid string) (*Verification, error) {
	return call[Verification](ctx, s.client, http.MethodGet, "/api/v3/verify/"+seg(sid), nil, nil)
}

// ---------------------------------------------------------------- Voice

// VoiceService covers the softphone token, numbers and call history.
type VoiceService struct {
	client *Client
	Calls  *CallsService
}

// AccessToken returns a short-lived Twilio Voice token. Platform may be empty.
func (s *VoiceService) AccessToken(ctx context.Context, platform string) (*VoiceAccessToken, error) {
	q := url.Values{}
	setString(q, "platform", platform)
	return call[VoiceAccessToken](ctx, s.client, http.MethodGet, "/api/v3/voice/access-token", q, nil)
}

// Numbers lists voice-capable numbers on the account.
func (s *VoiceService) Numbers(ctx context.Context) ([]VoiceNumber, error) {
	var out []VoiceNumber
	if err := s.client.do(ctx, http.MethodGet, "/api/v3/voice/numbers", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CallsService reads call history.
type CallsService struct{ client *Client }

func (s *CallsService) List(ctx context.Context, p *ListCallsParams) (*Page[VoiceCall], error) {
	if p == nil {
		p = &ListCallsParams{}
	}
	q := url.Values{}
	setString(q, "filter", p.Filter)
	setInt(q, "page", p.Page)
	setInt(q, "per_page", p.PerPage)
	return call[Page[VoiceCall]](ctx, s.client, http.MethodGet, "/api/v3/voice/calls", q, nil)
}

// ListAutoPaging iterates every matching call.
func (s *CallsService) ListAutoPaging(ctx context.Context, p *ListCallsParams) *Iter[VoiceCall] {
	params := ListCallsParams{}
	if p != nil {
		params = *p
	}
	return newIter(params.Page, func(ctx context.Context, page int) (*Page[VoiceCall], error) {
		params.Page = page
		return s.List(ctx, &params)
	})
}

func (s *CallsService) Get(ctx context.Context, uid string) (*VoiceCall, error) {
	return call[VoiceCall](ctx, s.client, http.MethodGet, "/api/v3/voice/calls/"+seg(uid), nil, nil)
}
