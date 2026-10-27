//go:build integration

package commerce

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

const (
	evidenceBuyerID    = identity.BuyerID(5321)
	evidenceStoreID    = int64(8800)
	evidenceProspectID = int64(8900)
	evidenceSnapHash   = "evidence-snapshot-hash"
)

type evidenceFixture struct {
	pool           *pgxpool.Pool
	repo           *repository.Repo
	uc             *SellerResponseUseCase
	publicationID  int64
	externalInvite int64
	externalPrinc  SellerPrincipal
	platformPrinc  SellerPrincipal
}

func seedEvidenceFixture(ctx context.Context, t *testing.T, pool *pgxpool.Pool) evidenceFixture {
	t.Helper()
	tm := database.NewTxManager(pool)
	repo := repository.New(pool, tm, acceptRaceIDGen())

	req, rev, err := repo.CreateRequest(ctx, evidenceBuyerID)
	require.NoError(t, err)
	_, err = repo.InsertConfirmedRfqSnapshot(ctx, repository.InsertConfirmedRfqSnapshotInput{
		BuyerID: evidenceBuyerID, RequestID: req.ID, RequestRevisionID: rev.ID,
		PricePreviewID: 8001, PricePreviewVersion: 1,
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN", PricingMode: "PRICEBOOK_GUIDED",
		CanonicalPayloadJSON: json.RawMessage(`{"item":"芭樂","quantity":100,"unit":"KG"}`),
		PayloadHash:          evidenceSnapHash, ClientRequestID: "evidence-snapshot",
		ExpiresAt: sellerResponseTestNow.Add(time.Hour),
	})
	require.NoError(t, err)
	var snapID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM procurement_confirmed_rfq_snapshots WHERE payload_hash = $1`, evidenceSnapHash).Scan(&snapID))

	pub, err := repo.InsertPublication(ctx, repository.InsertPublicationInput{
		BuyerID: evidenceBuyerID, RequestID: req.ID, RequestRevisionID: rev.ID,
		ApprovalID: 8401, ApprovalVersion: 1, CommandID: 8501, ConfirmedRfqSnapshotID: snapID,
		SellerFacingPayloadHash: "evidence-seller-facing-hash",
		AudienceMode:            "DIRECT_RECIPIENTS", AudienceHash: "evidence-audience-hash",
		State: "PUBLISHED", SchemaVersion: "hawkr.agent.publication.v1", PublishedAt: sellerResponseTestNow,
	})
	require.NoError(t, err)

	invites, err := repo.InsertPublicationInvites(ctx, repository.InsertPublicationInvitesInput{
		PublicationID: pub.ID, BuyerID: evidenceBuyerID, Now: sellerResponseTestNow,
		Recipients: []domain.PublicationRecipient{
			domain.PlatformRecipient(evidenceStoreID), domain.ExternalRecipient(evidenceProspectID),
		},
	})
	require.NoError(t, err)
	var externalInvite int64
	for _, inv := range invites {
		if inv.Lane == domain.QuoteLaneExternal {
			externalInvite = inv.ID
		}
	}
	require.NotZero(t, externalInvite)

	return evidenceFixture{
		pool: pool, repo: repo, uc: NewSellerResponseUseCase(repo, tm, sellerResponseNow),
		publicationID: pub.ID, externalInvite: externalInvite,
		externalPrinc: SellerPrincipal{InviteID: externalInvite},
		platformPrinc: SellerPrincipal{StoreID: evidenceStoreID},
	}
}

func (f evidenceFixture) sellerResponseID(ctx context.Context, t *testing.T, p SellerPrincipal) int64 {
	t.Helper()
	var id int64
	query := `SELECT id FROM procurement_seller_responses WHERE publication_id = $1 AND prospect_id = $2`
	arg := evidenceProspectID
	if !p.IsExternal() {
		query = `SELECT id FROM procurement_seller_responses WHERE publication_id = $1 AND store_id = $2`
		arg = evidenceStoreID
	}
	require.NoError(t, f.pool.QueryRow(ctx, query, f.publicationID, arg).Scan(&id))
	return id
}

func (f evidenceFixture) counts(ctx context.Context, t *testing.T) (versions, commands int) {
	t.Helper()
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM procurement_seller_response_versions`).Scan(&versions))
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM procurement_seller_response_commands`).Scan(&commands))
	return versions, commands
}

func (f evidenceFixture) submit(ctx context.Context, t *testing.T, p SellerPrincipal, key, expected string, v *repository.SellerResponseVerification) SellerResponseVersionResult {
	t.Helper()
	res, err := f.uc.Submit(ctx, SubmitSellerResponseInput{
		Principal: p, PublicationID: f.pubFor(p), ClientRequestID: key,
		ExpectedSellerResponseVersion: expected, Quote: validQuote(), Verification: v,
	})
	require.NoError(t, err)
	return res
}

func (f evidenceFixture) pubFor(p SellerPrincipal) int64 {
	if p.IsExternal() {
		return 0
	}
	return f.publicationID
}

func TestEvidence_MissingEvidenceIsRejectedAndWritesNothing(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	f := seedEvidenceFixture(ctx, t, pool)
	f.submit(ctx, t, f.externalPrinc, "seed", "", completeVerification())
	versions, commands := f.counts(ctx, t)

	for name, v := range incompleteVerifications() {
		_, err := f.uc.Submit(ctx, SubmitSellerResponseInput{
			Principal: f.externalPrinc, ClientRequestID: "s-" + name, ExpectedSellerResponseVersion: "1",
			Quote: validQuote(), Verification: v,
		})
		require.ErrorIs(t, err, ErrVerificationRequired, name)
		_, err = f.uc.Withdraw(ctx, WithdrawSellerResponseInput{
			Principal: f.externalPrinc, ClientRequestID: "w-" + name, ExpectedSellerResponseVersion: "1", Verification: v,
		})
		require.ErrorIs(t, err, ErrVerificationRequired, name)
		_, err = f.uc.Decline(ctx, DeclineRfqPublicationInput{
			Principal: f.externalPrinc, ClientRequestID: "d-" + name, Verification: v,
		})
		require.ErrorIs(t, err, ErrVerificationRequired, name)
	}
	gotVersions, gotCommands := f.counts(ctx, t)
	require.Equal(t, versions, gotVersions)
	require.Equal(t, commands, gotCommands)
}

func TestEvidence_SubmitPersistsEvidenceOnTheVersion(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	f := seedEvidenceFixture(ctx, t, pool)
	v := completeVerification()

	res := f.submit(ctx, t, f.externalPrinc, "k1", "", v)

	stored, err := f.repo.FindSellerResponseVersion(ctx, res.Identity.ID, 1)
	require.NoError(t, err)
	require.NotNil(t, stored.Verification)
	require.Equal(t, v.Method, stored.Verification.Method)
	require.Equal(t, v.Email, stored.Verification.Email)
	require.Equal(t, v.SessionID, stored.Verification.SessionID)
	require.True(t, v.VerifiedAt.Equal(stored.Verification.VerifiedAt), "verified_at 必須是同一個時間點")
}

func TestEvidence_WithdrawAndDeclineWriteOneRecordEach(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	f := seedEvidenceFixture(ctx, t, pool)
	v := completeVerification()
	f.submit(ctx, t, f.externalPrinc, "k1", "", v)

	out, err := f.uc.Withdraw(ctx, WithdrawSellerResponseInput{
		Principal: f.externalPrinc, ClientRequestID: "w1", ExpectedSellerResponseVersion: "1",
		ReasonCode: "PRICE_CHANGED", Verification: v,
	})
	require.NoError(t, err)
	require.NotNil(t, out.WithdrawnVersion)
	require.EqualValues(t, 1, *out.WithdrawnVersion)
	require.NotNil(t, out.WithdrawnAt)

	cmds, err := f.repo.ListSellerResponseCommands(ctx, out.ID)
	require.NoError(t, err)
	require.Len(t, cmds, 1)
	require.Equal(t, repository.SellerResponseCommandWithdraw, cmds[0].Kind)
	require.EqualValues(t, 1, *cmds[0].WithdrawnVersion)
	require.Equal(t, "w1", cmds[0].ClientRequestID)
	require.Equal(t, "PRICE_CHANGED", cmds[0].ReasonCode)
	require.NotNil(t, cmds[0].Verification)
	require.Equal(t, v.SessionID, cmds[0].Verification.SessionID)

	declined, err := f.uc.Decline(ctx, DeclineRfqPublicationInput{
		Principal: f.externalPrinc, ClientRequestID: "d1", ReasonCode: "NO_STOCK", Verification: v,
	})
	require.NoError(t, err)
	require.True(t, declined.WriteClosed())

	cmds, err = f.repo.ListSellerResponseCommands(ctx, out.ID)
	require.NoError(t, err)
	require.Len(t, cmds, 2)
	require.Equal(t, repository.SellerResponseCommandDecline, cmds[1].Kind)
	require.Nil(t, cmds[1].WithdrawnVersion)
	require.Equal(t, "NO_STOCK", cmds[1].ReasonCode)
	require.NotNil(t, cmds[1].Verification)
}

func TestEvidence_ReplayBySameKeyReturnsSameResultAndWritesNothing(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	f := seedEvidenceFixture(ctx, t, pool)
	v := completeVerification()
	f.submit(ctx, t, f.externalPrinc, "k1", "", v)
	in := WithdrawSellerResponseInput{
		Principal: f.externalPrinc, ClientRequestID: "w1", ExpectedSellerResponseVersion: "1", ReasonCode: "X", Verification: v,
	}
	first, err := f.uc.Withdraw(ctx, in)
	require.NoError(t, err)
	versions, commands := f.counts(ctx, t)

	replayed, err := f.uc.Withdraw(ctx, in)
	require.NoError(t, err)
	require.Equal(t, first.ID, replayed.ID)
	require.Equal(t, first.Version, replayed.Version, "回放不得再推進身分列的版本")
	require.Equal(t, first.WithdrawnVersion, replayed.WithdrawnVersion)
	require.True(t, first.WithdrawnAt.Equal(*replayed.WithdrawnAt))

	changed := in
	changed.ReasonCode = "Y"
	_, err = f.uc.Withdraw(ctx, changed)
	require.ErrorIs(t, err, repository.ErrSellerResponseIdempotencyConflict)

	gotVersions, gotCommands := f.counts(ctx, t)
	require.Equal(t, versions, gotVersions)
	require.Equal(t, commands, gotCommands)

	dIn := DeclineRfqPublicationInput{Principal: f.externalPrinc, ClientRequestID: "d1", Verification: v}
	// 撤回後的身分列還沒關閉寫入，婉拒第一次會寫一筆並關閉。
	firstDecline, err := f.uc.Decline(ctx, dIn)
	require.NoError(t, err)
	_, commands = f.counts(ctx, t)
	replayedDecline, err := f.uc.Decline(ctx, dIn)
	require.NoError(t, err)
	require.Equal(t, firstDecline.Version, replayedDecline.Version)
	_, gotCommands = f.counts(ctx, t)
	require.Equal(t, commands, gotCommands)
}

func TestEvidence_WithdrawSubmitWithdrawKeepsBothRecords(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	f := seedEvidenceFixture(ctx, t, pool)
	v := completeVerification()

	f.submit(ctx, t, f.externalPrinc, "k1", "", v)
	_, err := f.uc.Withdraw(ctx, WithdrawSellerResponseInput{
		Principal: f.externalPrinc, ClientRequestID: "w1", ExpectedSellerResponseVersion: "1", Verification: v,
	})
	require.NoError(t, err)
	f.submit(ctx, t, f.externalPrinc, "k2", "1", v)
	out, err := f.uc.Withdraw(ctx, WithdrawSellerResponseInput{
		Principal: f.externalPrinc, ClientRequestID: "w2", ExpectedSellerResponseVersion: "2", Verification: v,
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, *out.WithdrawnVersion, "身分列的撤回欄位由最新一筆紀錄推出")

	cmds, err := f.repo.ListSellerResponseCommands(ctx, out.ID)
	require.NoError(t, err)
	require.Len(t, cmds, 2)
	require.EqualValues(t, 1, *cmds[0].WithdrawnVersion)
	require.EqualValues(t, 2, *cmds[1].WithdrawnVersion)
}

func TestEvidence_PlatformPrincipalWritesRecordsWithNullEvidence(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	f := seedEvidenceFixture(ctx, t, pool)

	res := f.submit(ctx, t, f.platformPrinc, "k1", "", nil)
	require.Nil(t, res.Version.Verification)
	out, err := f.uc.Withdraw(ctx, WithdrawSellerResponseInput{
		Principal: f.platformPrinc, PublicationID: f.publicationID, ClientRequestID: "w1", ExpectedSellerResponseVersion: "1",
	})
	require.NoError(t, err)
	_, err = f.uc.Decline(ctx, DeclineRfqPublicationInput{
		Principal: f.platformPrinc, PublicationID: f.publicationID, ClientRequestID: "d1",
	})
	require.NoError(t, err)

	cmds, err := f.repo.ListSellerResponseCommands(ctx, out.ID)
	require.NoError(t, err)
	require.Len(t, cmds, 2)
	for _, c := range cmds {
		require.Nil(t, c.Verification)
	}
	stored, err := f.repo.FindSellerResponseVersion(ctx, out.ID, 1)
	require.NoError(t, err)
	require.Nil(t, stored.Verification)
}

func TestEvidence_DatabaseRejectsPartialEvidence(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	f := seedEvidenceFixture(ctx, t, pool)
	res := f.submit(ctx, t, f.externalPrinc, "k1", "", completeVerification())

	for name, evidence := range map[string]string{
		"method only":   `'GOOGLE', NULL, NULL, NULL`,
		"empty email":   `'GOOGLE', '', now(), 'sess'`,
		"empty session": `'GOOGLE', 'a@b.c', now(), ''`,
		"unknown":       `'EMAIL', 'a@b.c', now(), 'sess'`,
	} {
		_, err := pool.Exec(ctx, `INSERT INTO procurement_seller_response_commands
			(id, seller_response_id, kind, client_request_id, verification_method, verification_email, verified_at, verification_session_id)
			VALUES (99001, $1, 'DECLINE', 'raw-`+name+`', `+evidence+`)`, res.Identity.ID)
		require.Error(t, err, name)
	}
}

func TestEvidence_DatabaseRejectsPartialEvidenceOnVersions(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	f := seedEvidenceFixture(ctx, t, pool)
	res := f.submit(ctx, t, f.externalPrinc, "k1", "", completeVerification())

	for name, v := range map[string]*repository.SellerResponseVerification{
		"method only": {Method: repository.VerificationMethodGoogle},
		"no session":  {Method: repository.VerificationMethodGoogle, Email: "a@b.c", VerifiedAt: sellerResponseTestNow},
	} {
		in := repository.InsertSellerResponseVersionInput{
			SellerResponseID: res.Identity.ID, Version: 2, ResponseType: "MANUAL_QUOTE",
			OfferedQualityGrade: "A", OfferedQualityInspectionNotes: "n",
			UnitBasisQuantity: "1", UnitBasisUnit: "KG", OfferedQuantity: "1", OfferedQuantityUnit: "KG",
			ConversionBasis: "DIRECT", Currency: "TWD", GoodsAmount: 1, TaxInclusionBasis: "TAX_EXCLUDED",
			TotalAmount: 1, ValidUntil: sellerResponseTestNow.Add(time.Hour), PayloadHash: "h-" + name,
			PublicationVersion: 1, SellerFacingPayloadHash: "s", ClientRequestID: "raw-" + name,
			SubmittedAt: sellerResponseTestNow, Verification: v,
		}
		_, err := f.repo.InsertSellerResponseVersion(ctx, in)
		require.Error(t, err, name)
	}
}

func TestEvidence_ReplayAfterResubmitReturnsTheOriginalWithdrawal(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	f := seedEvidenceFixture(ctx, t, pool)
	v := completeVerification()
	f.submit(ctx, t, f.externalPrinc, "k1", "", v)
	in := WithdrawSellerResponseInput{
		Principal: f.externalPrinc, ClientRequestID: "w1", ExpectedSellerResponseVersion: "1", Verification: v,
	}
	first, err := f.uc.Withdraw(ctx, in)
	require.NoError(t, err)
	f.submit(ctx, t, f.externalPrinc, "k2", "1", v)

	replayed, err := f.uc.Withdraw(ctx, in)
	require.NoError(t, err)
	require.NotNil(t, replayed.WithdrawnVersion)
	require.EqualValues(t, 1, *replayed.WithdrawnVersion)
	require.EqualValues(t, 1, replayed.LatestVersion)
	require.True(t, first.WithdrawnAt.Equal(*replayed.WithdrawnAt))
}

func TestEvidence_ConcurrentWithdrawsWithTheSameKeyBothSucceedWithOneRecord(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	f := seedEvidenceFixture(ctx, t, pool)
	v := completeVerification()
	f.submit(ctx, t, f.externalPrinc, "k1", "", v)
	in := WithdrawSellerResponseInput{
		Principal: f.externalPrinc, ClientRequestID: "w1", ExpectedSellerResponseVersion: "1", Verification: v,
	}

	const n = 4
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, n)
	outs := make([]repository.SellerResponseIdentity, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			outs[i], errs[i] = f.uc.Withdraw(ctx, in)
		}(i)
	}
	close(start)
	wg.Wait()

	for i := 0; i < n; i++ {
		require.NoError(t, errs[i])
		require.NotNil(t, outs[i].WithdrawnVersion)
		require.EqualValues(t, 1, *outs[i].WithdrawnVersion)
	}
	_, commands := f.counts(ctx, t)
	require.Equal(t, 1, commands)
}
