// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

//go:build integration

package identity

// Require-MFA confinement: while the installation requires a second factor, a
// member who holds none is flagged must-enrol at admission — the same mechanism
// a forced password change uses — and the flag clears the moment they confirm a
// factor. An installation that requires nothing pays nothing for the check.

import (
	"context"
	"errors"
	"testing"

	"github.com/margince/margince/backend/internal/shared/apperrors"
)

func (e *revocationEnv) requireMFAOn() {
	e.svc.requireMFA = func(context.Context) (bool, error) { return true, nil }
}

func TestRequireMFAFlagsAMemberWithNoFactorAndClearsOnEnrolment(t *testing.T) {
	e := setupRevocationEnv(t, "require-mfa")
	e.withVault()
	e.requireMFAOn()

	// The member has no factor, so the password alone still opens a session —
	// but admission flags it must-enrol.
	_, login, err := e.svc.Login(e.wsOnlyCtx(), e.member.Email, memberPassword, noDevice)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	id, err := e.svc.Authenticate(e.wsOnlyCtx(), login.Token)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if !id.MustEnrolMFA {
		t.Fatal("a member with no factor under require-MFA is not flagged must-enrol")
	}

	// Enrol and confirm a factor; the same still-valid session now clears — it
	// is the one the confirm names as its own, so the session cutover spares it.
	ctx := e.asMember()
	secret, err := e.svc.StartTOTPEnrolment(ctx)
	if err != nil {
		t.Fatalf("start enrolment: %v", err)
	}
	code, err := totpCodeAt(secret, e.svc.now(), totpDigits)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.ConfirmTOTP(ctx, code, hashToken(login.Token)); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	cleared, err := e.svc.Authenticate(e.wsOnlyCtx(), login.Token)
	if err != nil {
		t.Fatalf("re-authenticate: %v", err)
	}
	if cleared.MustEnrolMFA {
		t.Error("must-enrol is still set after the member confirmed a factor")
	}
}

func TestAVaultlessServiceNeverChallengesConfinesOrEnrols(t *testing.T) {
	// The posture the composition root leaves when a vault exists but the
	// challenge-signing key is short or absent: the identity service gets NO
	// vault (compose couples the two), and everything MFA must then degrade to
	// "not composed" rather than strand members. The member here confirmed a
	// factor under a working configuration before the key broke.
	e := setupRevocationEnv(t, "mfa-vaultless")
	e.withVault()
	e.enrolMemberMFA(t)

	bare := NewServiceFor(e.svc.db)
	bare.requireMFA = func(context.Context) (bool, error) { return true, nil }

	// Password login never answers the 202 challenge state: a challenge this
	// service could not verify would be a lockout, not a factor.
	_, login, err := bare.Login(e.wsOnlyCtx(), e.member.Email, memberPassword, noDevice)
	if err != nil {
		t.Fatalf("login on a vaultless service: %v", err)
	}
	if login.Token == "" {
		t.Fatal("no session from the password although no challenge can complete")
	}

	// The require-MFA policy cannot confine while enrolment is impossible.
	id, err := bare.Authenticate(e.wsOnlyCtx(), login.Token)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if id.MustEnrolMFA {
		t.Error("a vaultless service confined the member to enrolment routes that can only refuse")
	}

	// And enrolment itself stays the not-composed refusal.
	if _, err := bare.StartTOTPEnrolment(e.asMember()); err == nil {
		t.Error("a vaultless service accepted a TOTP enrolment; the secret had nowhere safe to live")
	}
}

func TestRequireMFAOffLeavesTheMemberUnconfined(t *testing.T) {
	e := setupRevocationEnv(t, "require-mfa-off")
	e.withVault()
	// requireMFA left nil: no enforcement, and no per-request enrolment check.
	_, login, err := e.svc.Login(e.wsOnlyCtx(), e.member.Email, memberPassword, noDevice)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	id, err := e.svc.Authenticate(e.wsOnlyCtx(), login.Token)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if id.MustEnrolMFA {
		t.Error("must-enrol is set with require-MFA off")
	}
}

// Agent <= human holds for enrolment too. A human confined to enrolling a
// factor reaches nothing else through their passports. That holds on the bearer
// path, the trusted-process path and a tool admission. The policy is stored,
// never wired onto the service: most services in production are built bare.
func TestAPassportStopsWorkingWhileItsHumanOwesARequiredFactor(t *testing.T) {
	e := setupRevocationEnv(t, "require-mfa-passport")
	e.withVault()
	ctx := e.wsOnlyCtx()
	issued, err := e.svc.IssuePassport(ctx, e.member, IssuePassportInput{Scopes: []string{"read"}})
	if err != nil {
		t.Fatalf("issue passport: %v", err)
	}
	admit := func() error {
		_, _, err := e.svc.AdmittedAuthority(ctx, e.ws.UUID, e.member.UserID.UUID, issued.ID.UUID)
		return err
	}
	if _, err := e.svc.AuthenticateAgent(ctx, issued.Token); err != nil {
		t.Fatalf("the passport must authenticate before a factor is required: %v", err)
	}

	e.storeRequireMFA(t)
	if _, err := e.svc.AuthenticateAgent(ctx, issued.Token); !errors.Is(err, apperrors.ErrNotFound) {
		t.Fatalf("the passport authenticates while its human owes a required factor: err = %v, want not-found", err)
	}
	if _, err := e.svc.AuthenticateAgentByID(ctx, issued.ID); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("the passport resolves by id while its human owes a required factor: err = %v, want not-found", err)
	}
	if err := admit(); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("a tool call is admitted while the human owes a required factor: err = %v, want not-found", err)
	}

	// Enrolment ends only sessions, so the same passport answers again.
	e.enrolMemberMFA(t)
	if _, err := e.svc.AuthenticateAgent(ctx, issued.Token); err != nil {
		t.Errorf("the passport stays refused after its human confirmed a factor: %v", err)
	}
	if _, err := e.svc.AuthenticateAgentByID(ctx, issued.ID); err != nil {
		t.Errorf("the passport stays unresolvable by id after its human confirmed a factor: %v", err)
	}
	if err := admit(); err != nil {
		t.Errorf("a tool call stays refused after its human confirmed a factor: %v", err)
	}
}

// storeRequireMFA turns the policy on the way an admin does, in the setting
// row, and takes it off again when the test ends.
func (e *revocationEnv) storeRequireMFA(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	unset := func() error {
		_, err := e.owner.Exec(ctx, `DELETE FROM setting WHERE key = $1`, RequireMFA.Key())
		return err
	}
	if err := unset(); err != nil {
		t.Fatalf("clear the require-MFA setting: %v", err)
	}
	if _, err := e.owner.Exec(ctx, `INSERT INTO setting (key, value) VALUES ($1, 'true'::jsonb)`, RequireMFA.Key()); err != nil {
		t.Fatalf("store the require-MFA setting: %v", err)
	}
	t.Cleanup(func() {
		if err := unset(); err != nil {
			t.Errorf("clear the require-MFA setting: %v", err)
		}
	})
}
