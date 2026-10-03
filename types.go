package mista

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// FlexString decodes a JSON string, number or null into a string. Used for
// fields the API may send either way (cost, phone, remaining_unit).
type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if string(b) == "null" {
		*f = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = FlexString(s)
		return nil
	}
	*f = FlexString(b)
	return nil
}

func (f FlexString) String() string { return string(f) }

// FlexInt decodes a JSON number, numeric string or null into an int.
type FlexInt int

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(bytes.TrimSpace(b)), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = FlexInt(n)
	return nil
}

// FormatScheduleTime formats t as "2006-01-02 15:04", the schedule_time format
// the API expects. The wall-clock time is used as-is and read in your account
// timezone, so build t in that timezone.
func FormatScheduleTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

// ------------------------------------------------------------------ SMS

// SendSMSParams sends one message to one recipient.
type SendSMSParams struct {
	// One phone number in international format, e.g. "+1555***4567".
	To       string `json:"recipient"`
	SenderID string `json:"sender_id"`
	Message  string `json:"message"`
	// plain (default), unicode, voice, mms, whatsapp, viber, otp.
	Type string `json:"type,omitempty"`
	// Required for mms; optional for whatsapp and viber.
	MediaURL string `json:"media_url,omitempty"`
	// Required for voice.
	Language string `json:"language,omitempty"`
	// Required for voice.
	Gender        string `json:"gender,omitempty"`
	DLTTemplateID string `json:"dlt_template_id,omitempty"`
}

// MessageStatus is the delivery status of a message.
type MessageStatus string

const (
	// StatusQueued: not yet accepted by the carrier.
	StatusQueued MessageStatus = "Queued"
	// StatusSent: accepted by the carrier, waiting for the handset delivery report.
	StatusSent        MessageStatus = "Sent"
	StatusDelivered   MessageStatus = "Delivered"
	StatusUndelivered MessageStatus = "Undelivered"
	StatusExpired     MessageStatus = "Expired"
	StatusRejected    MessageStatus = "Rejected"
	StatusFailed      MessageStatus = "Failed"
)

// IsFinal reports whether the status will not change again.
func (s MessageStatus) IsFinal() bool {
	switch s {
	case StatusDelivered, StatusUndelivered, StatusExpired, StatusRejected, StatusFailed:
		return true
	}
	return false
}

// SMSMessage is a sent message and its delivery status.
type SMSMessage struct {
	UID     string        `json:"uid"`
	To      string        `json:"to"`
	From    string        `json:"from"`
	Message string        `json:"message"`
	Status  MessageStatus `json:"status"`
	// Why the message failed, was undelivered, expired or was rejected; empty otherwise.
	StatusDetail string     `json:"status_detail,omitempty"`
	Cost         FlexString `json:"cost"`
	SMSType      string     `json:"sms_type,omitempty"`
	Direction    string     `json:"direction,omitempty"`
	CreatedAt    string     `json:"created_at,omitempty"`
	UpdatedAt    string     `json:"updated_at,omitempty"`
}

// ------------------------------------------------------------ Campaigns

// PersonalizedRecipient is one number with its own message.
type PersonalizedRecipient struct {
	To      string `json:"to"`
	Message string `json:"message"`
}

// BulkCampaignParams sends to up to 10,000 numbers. Set either Recipients plus
// Message (broadcast) or Personalized (one message per number).
type BulkCampaignParams struct {
	SenderID     string
	Recipients   []string
	Message      string
	Personalized []PersonalizedRecipient
	Type         string
	Name         string
	// "2006-01-02 15:04" in your account timezone; see FormatScheduleTime.
	ScheduleTime  string
	DLTTemplateID string
}

// BulkCampaignResult is the queued campaign.
type BulkCampaignResult struct {
	UID            string `json:"uid"`
	CampaignName   string `json:"campaign_name"`
	Status         string `json:"status"`
	RecipientCount int    `json:"recipient_count"`
	Mode           string `json:"mode"`
}

// GroupCampaignParams sends one message to every subscribed contact in the groups.
type GroupCampaignParams struct {
	GroupUIDs     []string
	SenderID      string
	Message       string
	Type          string
	ScheduleTime  string
	DLTTemplateID string
}

// GroupCampaignResult is the queued campaign.
type GroupCampaignResult struct {
	UID          string `json:"uid"`
	CampaignName string `json:"campaign_name"`
	Status       string `json:"status"`
}

// Campaign is a campaign with its delivery counters.
type Campaign struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Message    string             `json:"message"`
	Status     string             `json:"status"`
	Type       string             `json:"type"`
	CreatedAt  string             `json:"created_at"`
	StartAt    string             `json:"start_at"`
	DeliveryAt string             `json:"delivery_at"`
	Stats      map[string]FlexInt `json:"stats"`
}

// ----------------------------------------------------------------- Logs

// ListMessagesParams filters the message log. Zero values are omitted.
type ListMessagesParams struct {
	Page    int
	PerPage int
	// "2006-01-02"
	StartDate string
	// "2006-01-02"
	EndDate  string
	SenderID string
	Status   MessageStatus
	SMSType  string
}

// -------------------------------------------------------------- Account

// Balance is the account's remaining units ("Unlimited" on unlimited plans).
type Balance struct {
	RemainingUnit   FlexString `json:"remaining_unit"`
	ExpiredOn       string     `json:"expired_on"`
	AirtimeBalance  FlexString `json:"airtime_balance"`
	AirtimeCurrency string     `json:"airtime_currency"`
}

// Account is the profile that owns the API token.
type Account struct {
	UID          string `json:"uid"`
	APIToken     string `json:"api_token"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Email        string `json:"email"`
	Locale       string `json:"locale"`
	Timezone     string `json:"timezone"`
	LastAccessAt string `json:"last_access_at"`
}

// ------------------------------------------------------------- Contacts

// ContactGroup is a phonebook.
type ContactGroup struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
}

// ContactFields are sent by their group field tag. Phone is always required.
type ContactFields struct {
	Phone     string
	FirstName string
	LastName  string
	// Custom fields keyed by the tag configured on the group, e.g. {"CITY": "Kigali"}.
	Fields map[string]string
}

// Contact is a contact with its field values.
type Contact struct {
	UID          string                `json:"uid"`
	Phone        FlexString            `json:"phone"`
	Status       string                `json:"status"`
	CustomFields map[string]FlexString `json:"custom_fields"`
}

// ContactListItem is a row in a group's contact list.
type ContactListItem struct {
	UID       string     `json:"uid"`
	Phone     FlexString `json:"phone"`
	FirstName string     `json:"first_name"`
	LastName  string     `json:"last_name"`
}

// --------------------------------------------------------------- Verify

// StartVerificationParams sends a one-time code.
type StartVerificationParams struct {
	To string `json:"to"`
	// auto (dashboard settings), sms, whatsapp, whatsapp_sms, sms_whatsapp, whatsapp_only.
	Channel  string `json:"channel,omitempty"`
	SenderID string `json:"sender_id,omitempty"`
}

// Verification is a verification and its state.
type Verification struct {
	SID           string `json:"sid"`
	To            string `json:"to"`
	Channel       string `json:"channel"`
	Status        string `json:"status"`
	ExpiresAt     string `json:"expires_at"`
	CheckAttempts int    `json:"check_attempts"`
	VerifiedAt    string `json:"verified_at"`
	CreatedAt     string `json:"created_at"`
}

// VerificationCheck is the result of checking a code. A wrong or expired code
// is reported with Verified=false and a Reason, not as an error.
type VerificationCheck struct {
	Verified   bool   `json:"-"`
	SID        string `json:"sid"`
	Status     string `json:"status"`
	VerifiedAt string `json:"verified_at"`
	// Why the check failed, e.g. "invalid_code" or "expired".
	Reason string `json:"reason"`
}

// ------------------------------------------------------------- Webhooks

// Delivery report webhook event types.
const (
	EventMessageDelivered = "message.delivered"
	EventMessageFailed    = "message.failed"
	EventWebhookTest      = "webhook.test"
)

// DeliveryReportWebhook is the account's delivery report webhook registration.
type DeliveryReportWebhook struct {
	// Empty when no webhook is registered.
	URL string `json:"url"`
	// Signing secret (whsec_...) used to verify the Mista-Signature header.
	Secret  string   `json:"secret"`
	Enabled bool     `json:"enabled"`
	Events  []string `json:"events"`
}

// SetWebhookParams registers or changes the webhook URL.
type SetWebhookParams struct {
	// Public http(s) URL that accepts POST requests.
	URL string `json:"url"`
	// Issue a new signing secret. The old one stops working immediately.
	RotateSecret bool `json:"rotate_secret,omitempty"`
}

// WebhookTestResult reports how your endpoint answered a webhook.test event.
type WebhookTestResult struct {
	Delivered bool `json:"delivered"`
	// HTTP status your endpoint answered with; 0 if it could not be reached.
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
	EventID    string `json:"event_id"`
}

// DeliveryReport is the message in its final state, sent as a webhook event's data.
type DeliveryReport struct {
	UID          string        `json:"uid"`
	To           string        `json:"to"`
	From         string        `json:"from"`
	Status       MessageStatus `json:"status"`
	StatusDetail string        `json:"status_detail"`
	Cost         FlexString    `json:"cost"`
	SMSCount     FlexInt       `json:"sms_count"`
	// Set when the message was part of a campaign.
	CampaignUID string `json:"campaign_uid"`
	SentAt      string `json:"sent_at"`
	UpdatedAt   string `json:"updated_at"`
}

// WebhookEvent is the body of a delivery report webhook request.
type WebhookEvent struct {
	// Unique per event and identical across retries; use it to ignore duplicates.
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	CreatedAt string         `json:"created_at"`
	Data      DeliveryReport `json:"data"`
}

// ---------------------------------------------------------------- Voice

// VoiceAccessToken is a short-lived Twilio Voice token for the softphone SDKs.
type VoiceAccessToken struct {
	Token       string `json:"token"`
	Identity    string `json:"identity"`
	CallerID    string `json:"caller_id"`
	AccountSID  string `json:"account_sid"`
	TwimlAppSID string `json:"twiml_app_sid"`
}

// VoiceNumber is a voice-capable number on the account.
type VoiceNumber struct {
	UID          string   `json:"uid"`
	Number       string   `json:"number"`
	Capabilities []string `json:"capabilities"`
	Provider     string   `json:"provider"`
	Voice        bool     `json:"voice"`
	Status       string   `json:"status"`
}

// ListCallsParams filters the call history. Zero values are omitted.
type ListCallsParams struct {
	// all, inbound, outbound, missed.
	Filter string
	Page   int
	// 1-100, default 20.
	PerPage int
}

// VoiceCall is one call.
type VoiceCall struct {
	UID               string  `json:"uid"`
	Direction         string  `json:"direction"`
	From              string  `json:"from"`
	To                string  `json:"to"`
	Status            string  `json:"status"`
	IsActive          bool    `json:"is_active"`
	Duration          *int    `json:"duration"`
	RecordingURL      *string `json:"recording_url"`
	RecordingDuration *int    `json:"recording_duration"`
	StartedAt         *string `json:"started_at"`
	EndedAt           *string `json:"ended_at"`
	CreatedAt         *string `json:"created_at"`
}
