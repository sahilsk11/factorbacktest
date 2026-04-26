package auth

import (
	"bytes"
	"crypto/rand"
	"errors"
	"factorbacktest/internal/db/models/postgres/app_auth/model"
	"factorbacktest/internal/repository"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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
	emailOTPTemplate    = "email_otp"
	maxEmailLen         = 320 // RFC 5321 cap; defensive bound on bcrypt input size
	maxOTPCodeLen       = 16  // bound the input we'd hand to bcrypt.Compare
)

// emailOTPTmpl is loaded once at first use. The strategy summary email
// in internal/service/email.service.go does the same kind of lookup
// per-call; we cache to avoid repeating disk I/O on every OTP send.
var (
	emailOTPTmplOnce sync.Once
	emailOTPTmpl     *template.Template
	emailOTPTmplErr  error
)

func loadEmailOTPTemplate() (*template.Template, error) {
	emailOTPTmplOnce.Do(func() {
		// Mirror internal/service/email.service.go's findTemplatePath
		// search list so the template resolves identically regardless
		// of which directory the binary was invoked from.
		wd, _ := os.Getwd()
		candidates := []string{
			filepath.Join("templates", emailOTPTemplate+".html"),
			filepath.Join("..", "templates", emailOTPTemplate+".html"),
			filepath.Join("../..", "templates", emailOTPTemplate+".html"),
			filepath.Join(wd, "templates", emailOTPTemplate+".html"),
		}
		var path string
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				path = c
				break
			}
		}
		if path == "" {
			emailOTPTmplErr = fmt.Errorf("email otp template %s not found in any of: %v", emailOTPTemplate, candidates)
			return
		}
		t, err := template.ParseFiles(path)
		if err != nil {
			emailOTPTmplErr = fmt.Errorf("parse email otp template: %w", err)
			return
		}
		emailOTPTmpl = t
	})
	return emailOTPTmpl, emailOTPTmplErr
}

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

// renderEmailOTPBody produces the HTML body the user receives from the
// templates/email_otp.html file. Keeping the markup in a real template
// (and not inlined in Go) matches the strategy-summary email path and
// makes copy edits possible without a redeploy of generated strings.
func renderEmailOTPBody(code string) (string, error) {
	tmpl, err := loadEmailOTPTemplate()
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	data := struct {
		Code             string
		ExpiresInMinutes int
	}{
		Code:             code,
		ExpiresInMinutes: int(emailOTPTTL / time.Minute),
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute email otp template: %w", err)
	}
	return buf.String(), nil
}

// handleEmailSend always returns 204. Same enumeration-shield rationale
// as handleSmsSend: if a request shape leaks "this email is registered"
// we've handed an attacker a free user-discovery oracle. Real failures
// (sender outage) are observable through 503 ONLY when the configuration
// itself is missing — actual transport errors are logged + swallowed.
//
// Brute-force surface (see internal/auth/README.md for the full
// threat-model row):
//   - per-email send limit (3/10min) caps how many fresh codes can be
//     generated for one address.
//   - per-IP send limit (10/10min) caps how many fresh codes can be
//     generated from one source.
//   - codes are bcrypt-hashed before persistence, so a DB read does
//     NOT yield the plaintext code (defense against compromised
//     read-only DB credentials).
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

	if !emailRegex.MatchString(email) || len(email) > maxEmailLen {
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
	row := &model.EmailOtp{
		Email:         email,
		CodeHash:      string(hash),
		ExpiresAt:     now.Add(emailOTPTTL),
		AttemptsLeft:  emailOTPMaxAttempts,
		IPCreatedFrom: ipPtr,
	}
	if _, err := s.emailOTPs.Create(c.Request.Context(), row); err != nil {
		s.log.Errorw("email send: insert otp", "err", err)
		c.Status(http.StatusNoContent)
		return
	}

	htmlBody, err := renderEmailOTPBody(code)
	if err != nil {
		s.log.Errorw("email send: render template", "err", err)
		c.Status(http.StatusNoContent)
		return
	}

	// Subject contains the code so iOS/Android can surface it on the
	// lock screen — a UX bet several auth providers make. The email
	// channel is the same threat surface either way; putting it in
	// the subject just speeds up the legit user.
	subject := fmt.Sprintf("Your Factor sign-in code: %s", code)
	if err := s.emailSender.SendEmail(email, subject, htmlBody); err != nil {
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
//
// Brute-force surface on this endpoint:
//   - attempts_left starts at 5 per OTP row and is GREATEST'd at 0 in
//     the DB, so 5 wrong submissions exhaust the row regardless of
//     handler concurrency.
//   - per-email and per-IP rate limits also gate verify (in addition
//     to the same limiters on /send), so an attacker can't stream
//     submissions even at 5-per-OTP.
//   - combined with the 3/email/10min /send cap, the per-window upper
//     bound on a single-email brute force is 3 OTPs × 5 attempts = 15
//     guesses against a 6-digit space (1.5e-5 success rate).
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
	if !emailRegex.MatchString(email) || len(email) > maxEmailLen || code == "" || len(code) > maxOTPCodeLen {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Mirror /send's rate-limiting on /verify so an attacker who knows
	// a valid OTP exists can't fire 5 guesses at it within milliseconds.
	// The IP bucket also stops cross-account brute-force from a single
	// source. Limit-trip collapses to the same 401 as a wrong code so
	// no oracle is exposed.
	ip := c.ClientIP()
	if !s.emailLimit.allowEmail(email) || !s.emailLimit.allowIP(ip) {
		s.log.Infow("email verify: rate limited", "email_suffix", emailSuffix(email), "ip", ip)
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
		if derr := s.emailOTPs.DecrementAttempts(ctx, otp.EmailOtpID); derr != nil {
			s.log.Errorw("email verify: decrement attempts", "err", derr)
		}
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if err := s.emailOTPs.MarkConsumed(ctx, otp.EmailOtpID, now); err != nil {
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
