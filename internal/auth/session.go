package auth

import (
	"context"
	"errors"
	authmodel "factorbacktest/internal/db/models/postgres/app_auth/model"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/util"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// loginUser is the shared post-identity-verified path. After Google or
// Twilio proves who the user is, the caller resolves them to a uuid and
// hands it here; we issue a fresh session and set the cookie.
func (s *Service) loginUser(ctx context.Context, c *gin.Context, userID uuid.UUID) error {
	id, err := newSessionID()
	if err != nil {
		return err
	}
	now := s.now().UTC()
	row := &authmodel.UserSession{
		ID:            id,
		UserAccountID: userID,
		CreatedAt:     now,
		ExpiresAt:     now.Add(s.cfg.SessionTTL),
		LastSeenAt:    now,
		IP:            stringPtrOrNil(c.ClientIP()),
		UserAgent:     stringPtrOrNil(truncate(c.GetHeader("User-Agent"), 512)),
	}
	if err := s.sessions.Create(ctx, row); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	s.setSessionCookie(c, s.signCookieValue(id), s.cfg.SessionTTL)
	return nil
}

func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// resolveSession is what the auth middleware calls on every request:
//  1. cookie present + HMAC valid?         -> false: anonymous
//  2. row exists + expires_at > now?       -> false: clear stale cookie
//  3. within absolute max age?             -> false: delete row, clear cookie
//  4. slide expires_at (best-effort Touch)
//
// Clearing the cookie on every "not authenticated" path means a stale or
// tampered cookie isn't repeatedly re-presented on subsequent requests.
func (s *Service) resolveSession(ctx context.Context, c *gin.Context) (uuid.UUID, bool) {
	raw, err := c.Request.Cookie(sessionCookieName)
	if err != nil || raw == nil {
		return uuid.Nil, false
	}
	id, ok := s.verifyCookieValue(raw.Value)
	if !ok {
		s.clearSessionCookie(c)
		return uuid.Nil, false
	}
	row, err := s.sessions.Get(ctx, id)
	if err != nil {
		if !errors.Is(err, repository.ErrSessionNotFound) {
			s.log.Errorw("session lookup failed", "err", err)
		}
		s.clearSessionCookie(c)
		return uuid.Nil, false
	}
	now := s.now().UTC()
	if now.Sub(row.CreatedAt) >= s.cfg.SessionAbsoluteMaxAge {
		// If Delete fails, the cookie holder could still authenticate
		// against the row until its sliding expires_at trips. Log so
		// it's observable; cookie is cleared regardless so the browser
		// re-auths.
		if err := s.sessions.Delete(ctx, id); err != nil {
			s.log.Errorw("absolute-max-age delete failed", "err", err)
		}
		s.clearSessionCookie(c)
		return uuid.Nil, false
	}
	newExpires := now.Add(s.cfg.SessionTTL)
	if newExpires.After(row.ExpiresAt) {
		if err := s.sessions.Touch(ctx, id, newExpires); err != nil {
			s.log.Warnw("session touch failed", "err", err)
		}
	}
	return row.UserAccountID, true
}

// upsertGoogleUser is the only path Google sign-in takes to materialize a
// user_account row. Identity is keyed on (LOCAL_GOOGLE, sub) — not email —
// so email changes / recycling don't cause account collisions.
func (s *Service) upsertGoogleUser(_ context.Context, sub, email, firstName, lastName string) (uuid.UUID, error) {
	in := &model.UserAccount{
		Provider:   model.UserAccountProviderType_LocalGoogle,
		ProviderID: util.StringPointer(sub),
	}
	if email != "" {
		in.Email = util.StringPointer(email)
	}
	if firstName != "" {
		in.FirstName = util.StringPointer(firstName)
	}
	if lastName != "" {
		in.LastName = util.StringPointer(lastName)
	}
	row, err := s.users.GetOrCreateByProviderIdentity(in)
	if err != nil {
		return uuid.Nil, err
	}
	return row.UserAccountID, nil
}

// upsertPhoneUser materializes a user_account row from a Twilio-verified
// phone number. Phone IS the identity for SMS auth: Twilio Verify proves
// control of the number, so any existing row with that phone (regardless
// of which provider created it) is the user. We use the existing
// repository.GetOrCreate (which keys lookup on phone_number when email
// isn't set) instead of GetOrCreateByProviderIdentity, because the
// unique-by-phone semantics matter more here than the unique-by-(provider,
// provider_id) tuple — and the latter would collide with the existing
// phone_number unique constraint.
func (s *Service) upsertPhoneUser(_ context.Context, phone string) (uuid.UUID, error) {
	in := &model.UserAccount{
		Provider:    model.UserAccountProviderType_LocalSms,
		ProviderID:  util.StringPointer(phone),
		PhoneNumber: util.StringPointer(phone),
	}
	row, err := s.users.GetOrCreate(in)
	if err != nil {
		return uuid.Nil, err
	}
	return row.UserAccountID, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
