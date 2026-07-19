package acceptance

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sbs-bsp-cctv/internal/audit"
	"sbs-bsp-cctv/internal/hardware"
	"sbs-bsp-cctv/internal/mailer"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrLinkInvalid     = errors.New("signing link is invalid, expired, or already used")
	ErrAlreadyAccepted = errors.New("client acceptance already recorded")
)

const ClientLinkTTL = 7 * 24 * time.Hour

type Service struct {
	pool     *pgxpool.Pool
	audit    *audit.Service
	hardware *hardware.Service
	mailCfg  mailer.Config
}

func NewService(pool *pgxpool.Pool, auditSvc *audit.Service, hardwareSvc *hardware.Service, mailCfg mailer.Config) *Service {
	return &Service{pool: pool, audit: auditSvc, hardware: hardwareSvc, mailCfg: mailCfg}
}

func toJSONB(v map[string]any) ([]byte, error) {
	if v == nil {
		v = map[string]any{}
	}
	return json.Marshal(v)
}

func fromJSONB(b []byte) (map[string]any, error) {
	m := map[string]any{}
	if len(b) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

const selectColumns = `id, hardware_unit_id, method, bsp_acceptance_by, bsp_signed_by_name, bsp_signature, bsp_acceptance_date,
	client_signature_link_expires_at, client_email, client_link_emailed_at, client_name, client_signed_at, client_signature_data,
	uploaded_document_path,
	head_office_signature_link_expires_at, head_office_email, head_office_link_emailed_at, head_office_name, head_office_signed_at, head_office_signature_data,
	comments, signed_off_at, meta_data`

func scanAcceptance(row pgx.Row) (*ClientAcceptance, error) {
	var a ClientAcceptance
	var metaRaw []byte
	err := row.Scan(&a.ID, &a.HardwareUnitID, &a.Method, &a.BSPAcceptanceBy, &a.BSPSignedByName, &a.BSPSignature, &a.BSPAcceptanceDate,
		&a.ClientSignatureLinkExpiresAt, &a.ClientEmail, &a.ClientLinkEmailedAt, &a.ClientName, &a.ClientSignedAt, &a.ClientSignatureData,
		&a.UploadedDocumentPath,
		&a.HeadOfficeSignatureLinkExpiresAt, &a.HeadOfficeEmail, &a.HeadOfficeLinkEmailedAt, &a.HeadOfficeName, &a.HeadOfficeSignedAt, &a.HeadOfficeSignatureData,
		&a.Comments, &a.SignedOffAt, &metaRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.MetaData, err = fromJSONB(metaRaw)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// GetAcceptance also opportunistically catches signed_off_at up via
// finalizeIfComplete (a no-op unless the condition is newly true and hasn't
// been recorded yet) — e.g. for records that satisfied the sign-off
// condition before that logic last changed, so they don't stay stuck
// without ever passing through one of the sign-off actions again.
func (s *Service) GetAcceptance(ctx context.Context, unitID uuid.UUID) (*ClientAcceptance, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+selectColumns+` FROM client_acceptances WHERE hardware_unit_id = $1`, unitID)
	a, err := scanAcceptance(row)
	if err != nil {
		return nil, err
	}
	return s.finalizeIfComplete(ctx, unitID, nil, a)
}

// ---- BSP internal acceptance ----

type BSPSignOffInput struct {
	// SignedByName is the free-text name typed into "Signed by" — the actual
	// physical signer (a Branch Manager or BSP representative on site),
	// which may differ from actor (the logged-in account submitting the
	// form, possibly on the signer's behalf).
	SignedByName string
	Signature    string
	Comments     *string
	MetaData     map[string]any
}

func (s *Service) RecordBSPAcceptance(ctx context.Context, unitID, actor uuid.UUID, in BSPSignOffInput) (*ClientAcceptance, error) {
	metaRaw, err := toJSONB(in.MetaData)
	if err != nil {
		return nil, err
	}

	var signedByName *string
	if in.SignedByName != "" {
		signedByName = &in.SignedByName
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO client_acceptances (hardware_unit_id, bsp_acceptance_by, bsp_signed_by_name, bsp_signature, bsp_acceptance_date, comments, meta_data)
		VALUES ($1, $2, $3, $4, now(), $5, $6)
		ON CONFLICT (hardware_unit_id) DO UPDATE SET
			bsp_acceptance_by = EXCLUDED.bsp_acceptance_by,
			bsp_signed_by_name = EXCLUDED.bsp_signed_by_name,
			bsp_signature = EXCLUDED.bsp_signature,
			bsp_acceptance_date = now(),
			comments = EXCLUDED.comments,
			meta_data = EXCLUDED.meta_data,
			updated_at = now()
		RETURNING `+selectColumns,
		unitID, actor, signedByName, in.Signature, in.Comments, metaRaw,
	)

	a, err := scanAcceptance(row)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "acceptance_bsp_signoff",
		PerformedBy: &actor, NewValue: a,
	})

	return s.finalizeIfComplete(ctx, unitID, &actor, a)
}

// ---- Client e-signature link ----

// GenerateClientSigningLink creates a fresh, time-limited token for the
// client to sign via the public (unauthenticated-by-login) signing page.
// Only the hash is stored; the raw token is returned once for BSP staff to
// share with the client through whatever channel is appropriate.
func (s *Service) GenerateClientSigningLink(ctx context.Context, unitID, actor uuid.UUID) (rawToken string, err error) {
	rawToken, err = s.createSigningLinkToken(ctx, unitID)
	if err != nil {
		return "", err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "acceptance_client_link_generated",
		PerformedBy: &actor,
	})

	return rawToken, nil
}

// createSigningLinkToken generates a new token, storing only its hash, and
// resets any prior (unfinished) branch-manager-signing state for the unit.
// Only the Branch Manager's own prior signature blocks this — the acceptance
// stage may already be closed via Head Office, but Branch Manager signing is
// the primary path and stays available regardless (it just won't reopen a
// closed stage; see finalizeIfComplete).
func (s *Service) createSigningLinkToken(ctx context.Context, unitID uuid.UUID) (rawToken string, err error) {
	if signed, err := s.branchManagerSigned(ctx, unitID); err != nil {
		return "", err
	} else if signed {
		return "", ErrAlreadyAccepted
	}

	rawToken = generateToken()
	hash := hashToken(rawToken)
	expiresAt := time.Now().Add(ClientLinkTTL)

	_, err = s.pool.Exec(ctx, `
		INSERT INTO client_acceptances (hardware_unit_id, method, client_signature_link_token, client_signature_link_expires_at)
		VALUES ($1, 'e_signature', $2, $3)
		ON CONFLICT (hardware_unit_id) DO UPDATE SET
			method = 'e_signature',
			client_signature_link_token = EXCLUDED.client_signature_link_token,
			client_signature_link_expires_at = EXCLUDED.client_signature_link_expires_at,
			client_signed_at = NULL,
			client_signature_data = NULL,
			client_name = NULL,
			updated_at = now()`,
		unitID, hash, expiresAt,
	)
	if err != nil {
		return "", err
	}

	return rawToken, nil
}

// EmailClientSigningLink generates a fresh signing link (same as
// GenerateClientSigningLink) and emails it to the client in an HTML
// template. baseURL is the server's public origin (scheme + host), used to
// build the full link the client clicks. If SMTP isn't configured, the
// intended recipient is still recorded so the workflow isn't blocked on
// infrastructure that may not exist yet in a given deployment.
func (s *Service) EmailClientSigningLink(ctx context.Context, unitID, actor uuid.UUID, clientEmail, baseURL string) (acc *ClientAcceptance, sent bool, err error) {
	rawToken, err := s.createSigningLinkToken(ctx, unitID)
	if err != nil {
		return nil, false, err
	}
	linkURL := baseURL + "/sign/" + rawToken

	unit, err := s.hardware.GetUnit(ctx, unitID)
	if err != nil {
		return nil, false, err
	}

	sendErr := mailer.SendHTML(s.mailCfg, clientEmail,
		fmt.Sprintf("Branch Manager Acceptance Sign-Off Required — %s", unit.Barcode),
		signingLinkEmailHTML(unit.Barcode, "Branch Manager", linkURL),
		nil,
	)
	sent = sendErr == nil
	if sendErr != nil && !errors.Is(sendErr, mailer.ErrNotConfigured) {
		return nil, false, sendErr
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE client_acceptances SET
			client_email = $2,
			client_link_emailed_at = now(),
			updated_at = now()
		WHERE hardware_unit_id = $1
		RETURNING `+selectColumns,
		unitID, clientEmail,
	)
	acc, err = scanAcceptance(row)
	if err != nil {
		return nil, false, err
	}

	notes := "sent via SMTP"
	if !sent {
		notes = "SMTP not configured; recorded without sending"
	}
	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "acceptance_client_link_emailed",
		PerformedBy: &actor, NewValue: acc, Notes: notes,
	})

	return acc, sent, nil
}

// ---- Head Office / Security Manager e-signature link (alternate signer) ----

// GenerateHeadOfficeSigningLink creates a fresh, time-limited token for the
// Head Office / Security Manager signer. Either this signer or the Branch
// Manager signing is sufficient to close the acceptance stage — whichever
// happens first.
func (s *Service) GenerateHeadOfficeSigningLink(ctx context.Context, unitID, actor uuid.UUID) (rawToken string, err error) {
	rawToken, err = s.createHeadOfficeSigningLinkToken(ctx, unitID)
	if err != nil {
		return "", err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "acceptance_head_office_link_generated",
		PerformedBy: &actor,
	})

	return rawToken, nil
}

// createHeadOfficeSigningLinkToken is gated only on Head Office's own prior
// signature — this signer is a purely optional/additional record and stays
// available even after the Branch Manager has already closed the stage.
func (s *Service) createHeadOfficeSigningLinkToken(ctx context.Context, unitID uuid.UUID) (rawToken string, err error) {
	if signed, err := s.headOfficeSigned(ctx, unitID); err != nil {
		return "", err
	} else if signed {
		return "", ErrAlreadyAccepted
	}

	rawToken = generateToken()
	hash := hashToken(rawToken)
	expiresAt := time.Now().Add(ClientLinkTTL)

	_, err = s.pool.Exec(ctx, `
		UPDATE client_acceptances SET
			head_office_signature_link_token = $2,
			head_office_signature_link_expires_at = $3,
			head_office_signed_at = NULL,
			head_office_signature_data = NULL,
			head_office_name = NULL,
			updated_at = now()
		WHERE hardware_unit_id = $1`,
		unitID, hash, expiresAt,
	)
	if err != nil {
		return "", err
	}

	return rawToken, nil
}

// EmailHeadOfficeSigningLink generates a fresh Head Office signing link and
// emails it. Same SMTP-not-configured fallback as EmailClientSigningLink.
func (s *Service) EmailHeadOfficeSigningLink(ctx context.Context, unitID, actor uuid.UUID, email, baseURL string) (acc *ClientAcceptance, sent bool, err error) {
	rawToken, err := s.createHeadOfficeSigningLinkToken(ctx, unitID)
	if err != nil {
		return nil, false, err
	}
	linkURL := baseURL + "/sign/" + rawToken

	unit, err := s.hardware.GetUnit(ctx, unitID)
	if err != nil {
		return nil, false, err
	}

	sendErr := mailer.SendHTML(s.mailCfg, email,
		fmt.Sprintf("Head Office / Security Manager Sign-Off Required — %s", unit.Barcode),
		signingLinkEmailHTML(unit.Barcode, "Head Office / Security Manager", linkURL),
		nil,
	)
	sent = sendErr == nil
	if sendErr != nil && !errors.Is(sendErr, mailer.ErrNotConfigured) {
		return nil, false, sendErr
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE client_acceptances SET
			head_office_email = $2,
			head_office_link_emailed_at = now(),
			updated_at = now()
		WHERE hardware_unit_id = $1
		RETURNING `+selectColumns,
		unitID, email,
	)
	acc, err = scanAcceptance(row)
	if err != nil {
		return nil, false, err
	}

	notes := "sent via SMTP"
	if !sent {
		notes = "SMTP not configured; recorded without sending"
	}
	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "acceptance_head_office_link_emailed",
		PerformedBy: &actor, NewValue: acc, Notes: notes,
	})

	return acc, sent, nil
}

func signingLinkEmailHTML(barcode, roleLabel, linkURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="margin:0;padding:0;background-color:#f4f5f7;font-family:Arial,Helvetica,sans-serif;">
	<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f4f5f7;padding:32px 0;">
		<tr>
			<td align="center">
				<table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:8px;overflow:hidden;">
					<tr>
						<td style="background-color:#1f2937;padding:20px 32px;">
							<span style="color:#ffffff;font-size:18px;font-weight:bold;">SBS BSP CCTV</span>
						</td>
					</tr>
					<tr>
						<td style="padding:32px;color:#1f2937;">
							<h1 style="font-size:18px;margin:0 0 16px;">%s Sign-Off Required</h1>
							<p style="font-size:14px;line-height:1.6;margin:0 0 16px;">
								Hardware unit <strong>%s</strong> is ready for your acceptance sign-off as %s.
								Please review and sign using the link below.
							</p>
							<p style="text-align:center;margin:24px 0;">
								<a href="%s" style="background-color:#2563eb;color:#ffffff;text-decoration:none;padding:12px 24px;border-radius:6px;font-size:14px;font-weight:bold;display:inline-block;">
									Review &amp; Sign
								</a>
							</p>
							<p style="font-size:12px;line-height:1.6;color:#6b7280;margin:0 0 8px;">
								If the button above doesn't work, copy and paste this link into your browser:
							</p>
							<p style="font-size:12px;word-break:break-all;color:#2563eb;margin:0 0 16px;">%s</p>
							<p style="font-size:12px;color:#6b7280;margin:0;">This link expires in 7 days.</p>
						</td>
					</tr>
				</table>
			</td>
		</tr>
	</table>
</body>
</html>`, roleLabel, barcode, roleLabel, linkURL, linkURL)
}

// branchManagerSigned reports whether the unit already has a completed
// Branch Manager signature (via e-signature or wet-ink upload), so callers
// can refuse to re-open/overwrite it. A missing row is not an error — it
// just means no client action has happened yet.
func (s *Service) branchManagerSigned(ctx context.Context, unitID uuid.UUID) (bool, error) {
	var signedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT client_signed_at FROM client_acceptances WHERE hardware_unit_id = $1`, unitID,
	).Scan(&signedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return signedAt != nil, nil
}

// headOfficeSigned reports whether the unit already has a completed Head
// Office / Security Manager signature — this is a purely optional/additional
// record, independent of whether the Branch Manager has already signed.
func (s *Service) headOfficeSigned(ctx context.Context, unitID uuid.UUID) (bool, error) {
	var signedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT head_office_signed_at FROM client_acceptances WHERE hardware_unit_id = $1`, unitID,
	).Scan(&signedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return signedAt != nil, nil
}

// GetPublicSigningInfo resolves a raw token (from the public link) to
// display context for the signing page, without exposing internal IDs. A
// token can belong to either sequential stage, so both are checked.
func (s *Service) GetPublicSigningInfo(ctx context.Context, rawToken string) (*PublicSigningInfo, error) {
	hash := hashToken(rawToken)

	var barcode, stage string
	var expiresAt time.Time
	var signedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT hu.barcode, 'branch_manager', ca.client_signature_link_expires_at, ca.client_signed_at
		FROM client_acceptances ca
		JOIN hardware_units hu ON hu.id = ca.hardware_unit_id
		WHERE ca.client_signature_link_token = $1
		UNION ALL
		SELECT hu.barcode, 'head_office', ca.head_office_signature_link_expires_at, ca.head_office_signed_at
		FROM client_acceptances ca
		JOIN hardware_units hu ON hu.id = ca.hardware_unit_id
		WHERE ca.head_office_signature_link_token = $1
		LIMIT 1`, hash,
	).Scan(&barcode, &stage, &expiresAt, &signedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLinkInvalid
	}
	if err != nil {
		return nil, err
	}

	return &PublicSigningInfo{
		HardwareBarcode: barcode,
		Stage:           stage,
		AlreadySigned:   signedAt != nil,
		Expired:         time.Now().After(expiresAt),
	}, nil
}

type ClientSignInput struct {
	ClientName    string
	SignatureData string
}

// SubmitClientSignature is called from the public signing page. It rejects
// expired or already-used links outright. The token resolves to whichever
// sequential stage (Branch Manager or Head Office) it was issued for.
func (s *Service) SubmitClientSignature(ctx context.Context, rawToken string, in ClientSignInput) (*ClientAcceptance, error) {
	hash := hashToken(rawToken)

	var unitID uuid.UUID
	var stage string
	var expiresAt time.Time
	var signedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT hardware_unit_id, 'branch_manager', client_signature_link_expires_at, client_signed_at
		FROM client_acceptances WHERE client_signature_link_token = $1
		UNION ALL
		SELECT hardware_unit_id, 'head_office', head_office_signature_link_expires_at, head_office_signed_at
		FROM client_acceptances WHERE head_office_signature_link_token = $1
		LIMIT 1`, hash,
	).Scan(&unitID, &stage, &expiresAt, &signedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLinkInvalid
	}
	if err != nil {
		return nil, err
	}
	if signedAt != nil {
		return nil, ErrAlreadyAccepted
	}
	if time.Now().After(expiresAt) {
		return nil, ErrLinkInvalid
	}

	var row pgx.Row
	if stage == "head_office" {
		row = s.pool.QueryRow(ctx, `
			UPDATE client_acceptances SET
				head_office_name = $2,
				head_office_signature_data = $3,
				head_office_signed_at = now(),
				updated_at = now()
			WHERE hardware_unit_id = $1
			RETURNING `+selectColumns,
			unitID, in.ClientName, in.SignatureData,
		)
	} else {
		row = s.pool.QueryRow(ctx, `
			UPDATE client_acceptances SET
				client_name = $2,
				client_signature_data = $3,
				client_signed_at = now(),
				updated_at = now()
			WHERE hardware_unit_id = $1
			RETURNING `+selectColumns,
			unitID, in.ClientName, in.SignatureData,
		)
	}

	a, err := scanAcceptance(row)
	if err != nil {
		return nil, err
	}

	action := "acceptance_client_signed"
	if stage == "head_office" {
		action = "acceptance_head_office_signed"
	}
	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: action,
		NewValue: a,
	})

	return s.finalizeIfComplete(ctx, unitID, nil, a)
}

// ---- Manual upload path ----

type ManualUploadInput struct {
	ClientName   *string
	DocumentPath string
}

func (s *Service) RecordManualUpload(ctx context.Context, unitID, actor uuid.UUID, in ManualUploadInput) (*ClientAcceptance, error) {
	if signed, err := s.branchManagerSigned(ctx, unitID); err != nil {
		return nil, err
	} else if signed {
		return nil, ErrAlreadyAccepted
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO client_acceptances (hardware_unit_id, method, client_name, uploaded_document_path, client_signed_at)
		VALUES ($1, 'manual_upload', $2, $3, now())
		ON CONFLICT (hardware_unit_id) DO UPDATE SET
			method = 'manual_upload',
			client_name = EXCLUDED.client_name,
			uploaded_document_path = EXCLUDED.uploaded_document_path,
			client_signed_at = now(),
			updated_at = now()
		RETURNING `+selectColumns,
		unitID, in.ClientName, in.DocumentPath,
	)

	a, err := scanAcceptance(row)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "acceptance_manual_upload",
		PerformedBy: &actor, NewValue: a,
	})

	return s.finalizeIfComplete(ctx, unitID, &actor, a)
}

// finalizeIfComplete marks the acceptance record signed off and moves the
// unit's Kanban card from Commissioning to Completed once ANY ONE of BSP
// Acceptance, Branch Manager, or Head Office / Security Manager sign-off is
// present — whichever happens first closes the stage; the other two are
// optional/additional records, not requirements. actor is nil for the
// public (unauthenticated) client-signing path.
func (s *Service) finalizeIfComplete(ctx context.Context, unitID uuid.UUID, actor *uuid.UUID, a *ClientAcceptance) (*ClientAcceptance, error) {
	if (a.BSPAcceptanceDate != nil || a.ClientSignedAt != nil || a.HeadOfficeSignedAt != nil) && a.SignedOffAt == nil {
		now := time.Now()
		if _, err := s.pool.Exec(ctx, `UPDATE client_acceptances SET signed_off_at = $2 WHERE hardware_unit_id = $1`, unitID, now); err != nil {
			return nil, err
		}
		a.SignedOffAt = &now

		if err := s.hardware.AdvanceBoardColumn(ctx, unitID, "completed", "completed"); err != nil {
			return nil, err
		}
		_ = s.audit.Record(ctx, audit.Entry{
			EntityType: "hardware_unit", EntityID: unitID.String(), Action: "acceptance_signed_off",
			PerformedBy: actor, NewValue: a,
		})
	}
	return a, nil
}
