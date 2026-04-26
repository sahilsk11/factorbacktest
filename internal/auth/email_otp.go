package auth

import (
	"crypto/rand"
	"errors"
	"factorbacktest/internal/repository"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// We deliberately keep the email regex loose: the IETF grammar admits
// shapes most servers reject anyway, and a tighter regex risks rejecting
// real addresses. The verify path requires the user to also receive a
// code at this address before it grants identity, so syntactic
// over-acceptance here is harmless.
var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// emailSuffix returns "***@example.com" — enough to triage abuse from
// logs without leaking the local-part.
func emailSuffix(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at <= 0 {
		return "***"
	}
	return "***" + email[at:]
}

const (
	emailOTPTTL         = 10 * time.Minute
	emailOTPMaxAttempts = 5
	bcryptCost          = 10
)

// generateEmailOTP returns 6 random decimal digits. crypto/rand + a
// single rand.Int(reader, 1_000_000) gives a uniform distribution; we
// then zero-pad so codes like 000123 still display correctly. Do NOT
// substitute math/rand here — the code IS the auth secret in transit.
func generateEmailOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("rand.Int: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// emailOTPBody builds the HTML the user receives. Keep it minimal: a
// single big number is what users actually copy, and complex HTML
// raises spam-classifier risk.
func emailOTPBody(code string) string {
	return fmt.Sprintf(`<!doctype html>
<html><body style="font-family:-apple-system,Segoe UI,sans-serif">
<p>Your Factor sign-in code:</p>
<p style="font-size:32px;font-weight:600;letter-spacing:4px">%s</p>
<p>This code expires in 10 minutes. If you didn't request it, ignore this email.</p>
</body></html>`, code)
}

// handleEmailSend always returns 204. Same enumeration-shield rationale
// as handleSmsSend: if a request shape leaks "this email is registered"
// we've handed an attacker a free user-discovery oracle. Real failures
// (sender outage) are observable through 503 ONLY when the configuration
// itself is missing — actual transport errors are logged + swallowed.
func (s *Service) handleEmailSend(c *gin.Context) {
	if s.emailSender == nil || s.emailOTPs == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	var body struct {
		Email string `json:"email"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.Status(http.StatusNoContent)
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	ip := c.ClientIP()

	if !emailRegex.MatchString(email) || len(email) > 320 {
		s.log.Infow("email send: bad email format", "ip", ip)
		c.Status(http.StatusNoContent)
		return
	}
	if !s.emailLimit.allowEmail(email) || !s.emailLimit.allowIP(ip) {
		s.log.Infow("email send: rate limited", "email_suffix", emailSuffix(email), "ip", ip)
		c.Status(http.StatusNoContent)
		return
	}

	code, err := generateEmailOTP()
	if err != nil {
		s.log.Errorw("email send: generate code", "err", err)
		c.Status(http.StatusNoContent)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcryptCost)
	if err != nil {
		s.log.Errorw("email send: bcrypt", "err", err)
		c.Status(http.StatusNoContent)
		return
	}

	now := s.now().UTC()
	ipPtr := stringPtrOrNil(ip)
	if _, err := s.emailOTPs.Create(c.Request.Context(), &repository.EmailOTP{
		Email:         email,
		CodeHash:      string(hash),
		ExpiresAt:     now.Add(emailOTPTTL),
		AttemptsLeft:  emailOTPMaxAttempts,
		IPCreatedFrom: ipPtr,
	}); err != nil {
		s.log.Errorw("email send: insert otp", "err", err)
		c.Status(http.StatusNoContent)
		return
	}

	// Subject contains the code so iOS/Android can surface it on the
	// lock screen — same UX bet Stripe/Slack make. The email channel
	// is the same threat surface either way; putting it in the subject
	// just speeds up the legit user.
	subject := fmt.Sprintf("Your Factor sign-in code: %s", code)
	if err := s.emailSender.SendEmail(email, subject, emailOTPBody(code)); err != nil {
		s.log.Errorw("email send: transport", "err", err, "email_suffix", emailSuffix(email))
	}
	c.Status(http.StatusNoContent)
}

// handleEmailVerify checks the submitted code against the latest pending
// OTP for this email, then on success issues a session. 401 covers all
// "you got it wrong" paths (no pending OTP, expired, mismatch, exhausted
// attempts) without distinguishing — same shape as handleSmsVerify so
// we don't hand an attacker an oracle. 503 only for our own infra
// failures; 500 if the post-verify user-creation/login path breaks.
func (s *Service) handleEmailVerify(c *gin.Context) {
	if s.emailSender == nil || s.emailOTPs == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	var body struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	code := strings.TrimSpace(body.Code)
	if !emailRegex.MatchString(email) || len(email) > 320 || code == "" || len(code) > 16 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	ctx := c.Request.Context()
	now := s.now().UTC()
	otp, err := s.emailOTPs.LatestUnconsumedByEmail(ctx, email, now)
	if err != nil {
		if errors.Is(err, repository.ErrEmailOTPNotFound) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		s.log.Errorw("email verify: lookup", "err", err)
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	// AttemptsLeft check is BEFORE bcrypt.Compare so an exhausted
	// row stops costing us hash work. We don't decrement here — the
	// row is already at the floor.
	if otp.AttemptsLeft <= 0 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(otp.CodeHash), []byte(code)); err != nil {
		// Wrong code: best-effort decrement, but a DB hiccup here
		// shouldn't change the response shape — the user still got
		// the code wrong. Log and 401.
		if derr := s.emailOTPs.DecrementAttempts(ctx, otp.EmailOTPID); derr != nil {
			s.log.Errorw("email verify: decrement attempts", "err", derr)
		}
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if err := s.emailOTPs.MarkConsumed(ctx, otp.EmailOTPID, now); err != nil {
		s.log.Errorw("email verify: mark consumed", "err", err)
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	userID, err := s.upsertEmailOtpUser(ctx, email)
	if err != nil {
		s.log.Errorw("email verify: get/create user", "err", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := s.loginUser(ctx, c, userID); err != nil {
		s.log.Errorw("email verify: login", "err", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}
