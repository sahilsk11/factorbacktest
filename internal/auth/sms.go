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

// E.164: leading +, 1-15 digits, no spaces or punctuation. Strict on
// purpose — Twilio expects E.164 and we want any FE-side normalization
// to be visible (no silent reformatting).
var e164Regex = regexp.MustCompile(`^\+[1-9][0-9]{1,14}$`)

// phoneSuffix returns the last 4 digits of an E.164 phone for logs,
// minimizing PII while keeping enough signal to triage abuse patterns.
func phoneSuffix(phone string) string {
	if len(phone) <= 4 {
		return "****"
	}
	return "****" + phone[len(phone)-4:]
}

// twilioVerifyEndpoint is var (not const) so tests can point it at httptest.
var twilioVerifyEndpoint = "https://verify.twilio.com/v2"

type twilioClient struct {
	cfg  TwilioConfig
	http *http.Client
}

func newTwilioClient(cfg TwilioConfig) *twilioClient {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &twilioClient{cfg: cfg, http: hc}
}

// sendVerification triggers Twilio Verify to deliver an OTP. We do NOT
// see the OTP value; Twilio generates, stores, and validates it. Returns
// nil for any "we did our part" outcome (incl. Twilio rate-limit) so the
// per-call response shape doesn't leak phone-registration state.
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
		// Twilio's per-phone limit triggered. Treat as a "we did our part"
		// outcome — caller still returns 204 to the user, so an attacker
		// can't distinguish "Twilio sent" from "Twilio rate-limited".
		return nil
	default:
		return fmt.Errorf("twilio verifications: unexpected status %d", resp.StatusCode)
	}
}

// verifyCheck asks Twilio whether (phone, code) is a valid pair. Returns
// approved=true ONLY when Twilio explicitly says status="approved";
// pending / expired / max_attempts all collapse to false.
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
		return false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("twilio verificationcheck: unexpected status %d", resp.StatusCode)
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, fmt.Errorf("decode verificationcheck: %w", err)
	}
	return body.Status == "approved", nil
}

func (t *twilioClient) do(ctx context.Context, endpoint string, form url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build twilio request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// SetBasicAuth keeps creds out of any error string we construct.
	req.SetBasicAuth(t.cfg.AccountSID, t.cfg.AuthToken)
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twilio request: %w", err)
	}
	return resp, nil
}

// handleSmsSend always returns 204 — uniform response prevents user
// enumeration ("is this phone registered?"). Validation, rate-limiting,
// and Twilio failures all collapse to the same 204.
func (s *Service) handleSmsSend(c *gin.Context) {
	var body struct {
		PhoneNumber string `json:"phoneNumber"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.Status(http.StatusNoContent)
		return
	}
	phone := strings.TrimSpace(body.PhoneNumber)
	ip := c.ClientIP()

	if !e164Regex.MatchString(phone) {
		s.log.Infow("sms send: bad phone format", "ip", ip)
		c.Status(http.StatusNoContent)
		return
	}
	if !s.smsLimit.allowPhone(phone) || !s.smsLimit.allowIP(ip) {
		// Log only the last 4 digits — enough to triage abuse
		// patterns from logs without storing full PII.
		s.log.Infow("sms send: rate limited", "phone_suffix", phoneSuffix(phone), "ip", ip)
		c.Status(http.StatusNoContent)
		return
	}
	if err := s.twilio.sendVerification(c.Request.Context(), phone); err != nil {
		s.log.Errorw("sms send: twilio error", "err", err)
	}
	c.Status(http.StatusNoContent)
}

// handleSmsVerify approves (phone, code) via Twilio Verify, then issues
// a session. 401 covers "wrong code" / "no pending verification" / "code
// expired" without distinguishing — Twilio's "no pending" and "wrong
// code" return the same response shape to us.
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
	if !e164Regex.MatchString(phone) || code == "" || len(code) > 16 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	ctx := c.Request.Context()
	approved, err := s.twilio.verifyCheck(ctx, phone, code)
	if err != nil {
		s.log.Errorw("sms verify: twilio error", "err", err)
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	if !approved {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	userID, err := s.upsertPhoneUser(ctx, phone)
	if err != nil {
		s.log.Errorw("sms verify: get/create user", "err", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := s.loginUser(ctx, c, userID); err != nil {
		s.log.Errorw("sms verify: login", "err", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}
