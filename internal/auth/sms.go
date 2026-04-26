package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// E.164 regex: leading +, 1-15 digits, no spaces or punctuation. We're
// intentionally strict here: Twilio expects E.164, and any normalization
// the FE chooses to do should be visible to the server (no silent
// reformatting on our side).
var e164Regex = regexp.MustCompile(`^\+[1-9][0-9]{1,14}$`)

// twilioClient is the minimal Twilio Verify wrapper we need. We use direct
// REST calls instead of twilio-go because (a) we use only two endpoints,
// (b) we want strict timeouts and explicit retry behavior that matches our
// threat model, (c) it's one fewer dependency to track for security
// advisories.
type twilioClient struct {
	cfg  TwilioConfig
	http *http.Client
}

func newTwilioClient(cfg TwilioConfig) *twilioClient {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{
			Timeout: 10 * time.Second,
		}
	}
	return &twilioClient{cfg: cfg, http: hc}
}

// twilioVerifyEndpoint is overridable in tests so we can point at httptest.
// In production it's always Twilio's real API.
var twilioVerifyEndpoint = "https://verify.twilio.com/v2"

// sendVerification triggers Twilio Verify to deliver an OTP via SMS to the
// given E.164 phone number. We do NOT see the OTP value; Twilio generates,
// stores, and validates it.
//
// Returns nil for any "we did our part" outcome (Twilio accepted the
// request, even if delivery later fails for a bad number). Returns an
// error only for our-side problems (network, auth misconfig). This is
// deliberate: we don't want to leak "phone number is registered with
// Twilio" vs "phone number isn't registered" to callers via different
// error shapes.
func (t *twilioClient) sendVerification(ctx context.Context, phone string) error {
	form := url.Values{}
	form.Set("To", phone)
	form.Set("Channel", "sms")

	endpoint := fmt.Sprintf("%s/Services/%s/Verifications", twilioVerifyEndpoint, t.cfg.VerifyServiceSID)
	resp, err := t.do(ctx, endpoint, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusTooManyRequests:
		// Twilio's per-phone rate limit triggered. Surface as nil so we
		// don't change client-visible behavior — the per-IP/per-phone
		// limiter on our side has already accepted this request. An
		// attacker can't tell the difference between "Twilio sent" and
		// "Twilio rate-limited," which is what we want.
		return nil
	default:
		return fmt.Errorf("twilio verifications: unexpected status %d", resp.StatusCode)
	}
}

// verifyCheck asks Twilio whether (phone, code) is a valid pair. Returns
// (approved, err). Approved is true ONLY when Twilio explicitly returns
// status="approved"; any other status (pending, expired, max_attempts) is
// treated as not approved.
func (t *twilioClient) verifyCheck(ctx context.Context, phone, code string) (bool, error) {
	form := url.Values{}
	form.Set("To", phone)
	form.Set("Code", code)

	endpoint := fmt.Sprintf("%s/Services/%s/VerificationCheck", twilioVerifyEndpoint, t.cfg.VerifyServiceSID)
	resp, err := t.do(ctx, endpoint, form)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// 404 = no pending verification for this phone. Treat as bad code.
		return false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("twilio verificationcheck: unexpected status %d", resp.StatusCode)
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, fmt.Errorf("decode verificationcheck response: %w", err)
	}
	return body.Status == "approved", nil
}

// do issues an authenticated POST to a Twilio Verify endpoint.
// Authentication uses HTTP Basic with (account_sid, auth_token); we set
// these via http.Request.SetBasicAuth so the credentials never appear in
// any error string we construct.
func (t *twilioClient) do(ctx context.Context, endpoint string, form url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build twilio request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(t.cfg.AccountSID, t.cfg.AuthToken)
	resp, err := t.http.Do(req)
	if err != nil {
		// Don't include the request body or headers in the error: if for
		// some reason we ever log it, we don't want creds or phone numbers
		// in the log.
		return nil, fmt.Errorf("twilio request: %w", err)
	}
	return resp, nil
}

// handleSmsSend triggers an OTP delivery. Always responds 204, regardless
// of: phone number format, rate-limit status, or Twilio's response. The
// uniform response is what prevents user enumeration ("is this phone
// registered?"). Tests in TestSmsSend_NoEnumerationLeak assert this.
//
// Rate limits (in-memory, per-process — see README's "multi-instance gap"):
//   - 3 requests per phone number per 10 minutes
//   - 10 requests per source IP per 10 minutes
//
// Both checks must pass; we don't reveal which one tripped.
func (s *Service) handleSmsSend(c *gin.Context) {
	var body struct {
		PhoneNumber string `json:"phoneNumber"`
	}
	if err := c.BindJSON(&body); err != nil {
		// Even malformed bodies get 204. We never want a 4xx to leak the
		// shape of valid input vs. invalid.
		c.Status(http.StatusNoContent)
		return
	}
	phone := strings.TrimSpace(body.PhoneNumber)
	ip := clientIP(c)

	// Validation, rate-limiting, and Twilio failures all collapse to 204.
	// We log internally so abuse is observable in logs but not over the wire.
	if !e164Regex.MatchString(phone) {
		logf("sms send: bad phone format ip=%s", ip)
		c.Status(http.StatusNoContent)
		return
	}
	if !s.smsLimit.allowPhone(phone) || !s.smsLimit.allowIP(ip) {
		logf("sms send: rate limited phone=%s ip=%s", phone, ip)
		c.Status(http.StatusNoContent)
		return
	}

	if err := s.twilio.sendVerification(c.Request.Context(), phone); err != nil {
		logf("sms send: twilio error: %v", err)
		// Still 204. The user shouldn't see Twilio's internal state.
	}
	c.Status(http.StatusNoContent)
}

// handleSmsVerify checks the (phone, code) pair against Twilio Verify and,
// on approval, finds-or-creates the user and creates a session.
//
// Failure responses use 401 Unauthorized so the FE can distinguish "wrong
// code" from "we have no idea what's going on" (5xx). 401 doesn't leak
// whether the phone was previously sent an OTP — Twilio's "no pending
// verification" returns the same 401.
func (s *Service) handleSmsVerify(c *gin.Context) {
	var body struct {
		PhoneNumber string `json:"phoneNumber"`
		Code        string `json:"code"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	phone := strings.TrimSpace(body.PhoneNumber)
	code := strings.TrimSpace(body.Code)
	if !e164Regex.MatchString(phone) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if code == "" || len(code) > 16 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	ctx := c.Request.Context()
	approved, err := s.twilio.verifyCheck(ctx, phone, code)
	if err != nil {
		logf("sms verify: twilio error: %v", err)
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	if !approved {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	userID, err := s.users.GetOrCreateByPhone(ctx, phone)
	if err != nil {
		logf("sms verify: get/create user: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if _, err := s.loginUser(ctx, c, userID); err != nil {
		logf("sms verify: login: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}
