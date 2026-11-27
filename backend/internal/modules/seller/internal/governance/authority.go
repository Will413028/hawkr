package governance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)


// Accounts 是登記來函時驗證被指名的帳號存在需要的 identity 公開 API（含已刪除的墓碑：帳號存在過，來函照樣要登記與回覆）。
type Accounts interface {
	Account(context.Context, int64) (identity.Account, error)
}

// ConflictTargetAuthorityOrder 是完成、延長保存與記下告知帶的來函 etag 過時（版本衝突的 conflictTarget）。
const ConflictTargetAuthorityOrder = "authority_order"

func errAuthorityOrderVersion() error {
	return apperr.VersionConflict("%s", text.AuthorityOrderVersionConflict().Text).WithMetadata(ConflictTargetKey, ConflictTargetAuthorityOrder)
}

func errAuthorityOrderNotFound() error {
	m, _ := uitext.For(uitext.ZhTW).CommonError("NOT_FOUND")
	return apperr.NotFound("%s", m.Text)
}

func authorityBlocked(blocks []model.AuthorityOrderBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	return apperr.FailedPrecondition("%s", blocks[0].Text().Text)
}

func targetField(i int, name string) string {
	return fmt.Sprintf("Targets[%d].%s", i, name)
}

func ptrTime(t time.Time) *time.Time { return &t }

func toAuthorityOrder(row store.SellerAuthorityOrder) model.AuthorityOrder {
	o := model.AuthorityOrder{
		ID: row.ID, Agency: row.Agency, DocumentNo: row.DocumentNo, ReceivedAt: row.ReceivedAt, Kind: model.AuthorityOrderKind(row.Kind),
		Summary: row.Summary, DueAt: row.DueAt, HoldUntil: row.HoldUntil, RegisteredAt: row.RegisteredAt, RegisteredByAdminID: row.RegisteredByAdminID,
		Version: row.Version,
	}
	if row.CompletedAt != nil && row.CompletedByAdminID != nil && row.ResolutionSummary != nil {
		o.Completed = &model.AuthorityCompletion{At: *row.CompletedAt, AdminID: *row.CompletedByAdminID, Summary: *row.ResolutionSummary}
	}
	if row.RevokedAt != nil && row.RevokedByAdminID != nil && row.RevokeReason != nil {
		o.Revoked = &model.AuthorityRevocation{At: *row.RevokedAt, AdminID: *row.RevokedByAdminID, Reason: *row.RevokeReason}
	}
	return o
}

func toAuthorityTarget(row store.SellerAuthorityOrderTarget) model.AuthorityOrderTarget {
	t := model.AuthorityOrderTarget{
		Position: int(row.Position), AccountID: row.AccountID, StoreID: row.StoreID, ProductIDs: slices.Clone(row.ProductIds),
	}
	if t.ProductIDs == nil {
		t.ProductIDs = []string{}
	}
	if row.Notice != nil && row.NoticeRecordedAt != nil && row.NoticeRecordedByAdminID != nil {
		t.Notice = &model.AuthorityNotice{
			Kind: model.AuthorityNoticeKind(*row.Notice), NotifiedAt: row.NotifiedAt, Note: deref(row.NoticeNote), RecheckOn: row.RecheckOn,
			RecordedAt: *row.NoticeRecordedAt, RecordedByAdminID: *row.NoticeRecordedByAdminID,
		}
	}
	return t
}

func readAuthorityOrders(ctx context.Context, q *store.Queries, rows []store.SellerAuthorityOrder, withExtensions bool, now time.Time) ([]model.AdminAuthorityOrder, error) {
	out := make([]model.AdminAuthorityOrder, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(rows))
	index := make(map[uuid.UUID]int, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
		index[r.ID] = i
		out[i].AuthorityOrder = toAuthorityOrder(r)
	}
	targets, err := q.AuthorityOrderTargets(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, t := range targets {
		o := &out[index[t.OrderID]]
		o.Targets = append(o.Targets, toAuthorityTarget(t))
	}
	stores, err := q.AuthorityTargetStores(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, st := range stores {
		o := &out[index[st.OrderID]]
		t := &o.Targets[st.Position]
		ts := model.AuthorityTargetStore{
			StoreID: st.StoreID, StoreName: st.StoreName, PurgedAt: st.PiiPurgedAt, Coverage: o.StoreCoverageOf(st.PiiPurgedAt, st.PurgedAtRegistration, now),
			Target: model.TerminationTarget{Closed: st.Closed, LatestOfAccount: st.LatestOfAccount},
		}
		ts.CanCreateMeasure, ts.CanCreateTermination = o.CanCreateMeasureOn(ts), o.CanCreateTerminationOn(ts)
		t.Stores = append(t.Stores, ts)
	}
	if withExtensions {
		extensions, err := q.AuthorityHoldExtensions(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, e := range extensions {
			o := &out[index[e.OrderID]]
			o.HoldExtensions = append(o.HoldExtensions, model.AuthorityHoldExtension{
				From: e.FromUntil, To: e.ToUntil, Cause: model.HoldExtensionCause(e.Cause), Reason: deref(e.Reason), AdminID: e.AdminID, CreatedAt: e.CreatedAt,
			})
		}
	}
	deadlines, err := q.ReviewAuthorityOrderByIDs(ctx, store.ReviewAuthorityOrderByIDsParams{At: now, Ids: ids})
	if err != nil {
		return nil, err
	}
	for _, d := range deadlines {
		out[index[d.AuthorityOrderID]].Deadline = &model.Deadline{Start: d.StartAt, Due: d.DueAt}
	}
	for i := range out {
		o := &out[i]
		for j := range o.Targets {
			o.Targets[j].NoSellerData = len(o.Targets[j].Stores) == 0
		}
		o.Availability = o.AuthorityOrder.Availability()
		o.HoldActive = o.AuthorityOrder.HoldActive(now)
	}
	return out, nil
}

func readAuthorityOrder(ctx context.Context, q *store.Queries, id uuid.UUID, now time.Time) (model.AdminAuthorityOrder, error) {
	row, err := q.AuthorityOrderByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AdminAuthorityOrder{}, errAuthorityOrderNotFound()
	}
	if err != nil {
		return model.AdminAuthorityOrder{}, err
	}
	views, err := readAuthorityOrders(ctx, q, []store.SellerAuthorityOrder{row}, true, now)
	if err != nil {
		return model.AdminAuthorityOrder{}, err
	}
	return views[0], nil
}

func authorityRequestHash(in model.CreateAuthorityOrderInput, agency, documentNo, summary string, targets []model.AuthorityTargetInput, due *time.Time) []byte {
	h := sha256.New()
	put := func(parts ...string) {
		for _, p := range parts {
			_, _ = fmt.Fprintf(h, "%d:%s|", len(p), p)
		}
	}
	at := "-"
	if due != nil {
		at = due.UTC().Format(time.RFC3339Nano)
	}
	put(agency, documentNo, in.ReceivedAt.UTC().Format(time.RFC3339Nano), string(in.Kind), summary, at, strconv.Itoa(len(targets)))
	for _, t := range targets {
		storeID := "-"
		if t.StoreID != nil {
			storeID = strconv.FormatInt(*t.StoreID, 10)
		}
		put(strconv.FormatInt(t.AccountID, 10), storeID, strconv.Itoa(len(t.ProductIDs)))
		put(t.ProductIDs...)
	}
	return h.Sum(nil)
}

func validateTargets(in []model.AuthorityTargetInput) ([]model.AuthorityTargetInput, error) {
	if n := len(in); n < 1 || n > model.MaxAuthorityTargets {
		return nil, invalidOn("Targets", text.AuthorityOrderTargetsCount())
	}
	out := make([]model.AuthorityTargetInput, len(in))
	type key struct {
		account int64
		store   int64
	}
	seen := make(map[key]bool, len(in))
	whole := make(map[int64]bool, len(in))
	withStore := make(map[int64]bool, len(in))
	for i, t := range in {
		if t.AccountID <= 0 {
			return nil, invalidOn(targetField(i, "AccountID"), text.AuthorityOrderAccountNotFound())
		}
		k := key{account: t.AccountID}
		if t.StoreID != nil {
			if *t.StoreID <= 0 {
				return nil, invalidOn(targetField(i, "StoreID"), text.AuthorityOrderTargetStoreMismatch())
			}
			k.store = *t.StoreID
			withStore[t.AccountID] = true
		} else {
			k.store = -1
			whole[t.AccountID] = true
		}
		if seen[k] {
			return nil, invalidOn(targetField(i, "AccountID"), text.AuthorityOrderTargetDuplicate())
		}
		seen[k] = true
		products := make([]string, 0, len(t.ProductIDs))
		for _, p := range t.ProductIDs {
			if p = strings.TrimSpace(p); p != "" && !slices.Contains(products, p) {
				products = append(products, p)
			}
		}
		if len(products) > model.MaxAuthorityTargetProducts {
			return nil, invalidOn(targetField(i, "ProductIDs"), text.AuthorityOrderProductsCount())
		}
		out[i] = model.AuthorityTargetInput{AccountID: t.AccountID, StoreID: t.StoreID, ProductIDs: products}
	}
	for i, t := range out {
		if t.StoreID != nil && whole[t.AccountID] {
			return nil, invalidOn(targetField(i, "StoreID"), text.AuthorityOrderTargetOverlap())
		}
	}
	return out, nil
}

func distinctAccounts(targets []model.AuthorityTargetInput) []int64 {
	ids := make([]int64, 0, len(targets))
	for _, t := range targets {
		ids = append(ids, t.AccountID)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// CreateAuthorityOrder 登記主管機關的一份來函。冪等重送（帶 request_id）先於有狀態的驗證：同一位管理員、同一個 request_id、同樣的內容回傳原本那一份（AIP-155
// 的 stale success 例外），內容不同回 InvalidInput；鎖住被指名的帳號之後再查一次。帳號存在與否在 transaction 外問 identity；transaction 內先依 account_id 升序鎖住
// 被指名帳號的店家列（沒有店家的帳號沒有列可鎖，也沒有資料要保存），再取現在（登記時間因此晚於任何先 commit 的清理）、檢查收文時間與店家屬於帳號。保存期限定為收文後 AuthorityHold。
// 不發事件：沒有消費者。
func (s *Service) CreateAuthorityOrder(ctx context.Context, in model.CreateAuthorityOrderInput) (model.CreateAuthorityOrderResult, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.CreateAuthorityOrderResult{}, err
	}
	agency := strings.TrimSpace(in.Agency)
	if n := utf8.RuneCountInString(agency); n == 0 || n > model.AuthorityAgencyMaxRunes {
		return model.CreateAuthorityOrderResult{}, invalidOn("Agency", text.AuthorityOrderAgencyLength())
	}
	documentNo := strings.TrimSpace(in.DocumentNo)
	if n := utf8.RuneCountInString(documentNo); n == 0 || n > model.AuthorityDocumentNoMaxRunes {
		return model.CreateAuthorityOrderResult{}, invalidOn("DocumentNo", text.AuthorityOrderDocumentNoLength())
	}
	summary := strings.TrimSpace(in.Summary)
	if n := utf8.RuneCountInString(summary); n == 0 || n > model.AuthoritySummaryMaxRunes {
		return model.CreateAuthorityOrderResult{}, invalidOn("Summary", text.AuthorityOrderSummaryLength())
	}
	if !slices.Contains(model.AuthorityOrderKindValues(), in.Kind) {
		return model.CreateAuthorityOrderResult{}, invalidOn("Kind", text.AuthorityOrderKindRequired())
	}
	if in.ReceivedAt.IsZero() {
		return model.CreateAuthorityOrderResult{}, invalidOn("ReceivedAt", text.AuthorityOrderReceiveTimeRange())
	}
	receivedAt := in.ReceivedAt.UTC().Truncate(time.Microsecond)
	in.ReceivedAt = receivedAt
	dueAt := receivedAt.Add(model.DefaultAuthorityDue(in.Kind))
	var due *time.Time
	if in.DueAt != nil {
		t := in.DueAt.UTC().Truncate(time.Microsecond)
		due, dueAt = &t, t
		if d := dueAt.Sub(receivedAt); d < model.AuthorityDueMin || d > model.AuthorityDueMax {
			return model.CreateAuthorityOrderResult{}, invalidOn("DueAt", text.AuthorityOrderDueRange())
		}
	}
	targets, err := validateTargets(in.Targets)
	if err != nil {
		return model.CreateAuthorityOrderResult{}, err
	}
	var hash []byte
	if in.RequestID != nil {
		hash = authorityRequestHash(in, agency, documentNo, summary, targets, due)
	}
	for i, t := range targets {
		if i > 0 && slices.ContainsFunc(targets[:i], func(p model.AuthorityTargetInput) bool { return p.AccountID == t.AccountID }) {
			continue
		}
		if _, err := s.d.Identity.Account(ctx, t.AccountID); err != nil {
			if apperr.KindOf(err) == apperr.KindNotFound {
				return model.CreateAuthorityOrderResult{}, invalidOn(targetField(i, "AccountID"), text.AuthorityOrderAccountNotFound())
			}
			return model.CreateAuthorityOrderResult{}, err
		}
	}
	var result model.CreateAuthorityOrderResult
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if replayed, ok, err := s.replayAuthorityOrder(ctx, q, in.AdminID, in.RequestID, hash); err != nil || ok {
			result = replayed
			return err
		}
		if err := stint.LockAccounts(ctx, q, distinctAccounts(targets)); err != nil {
			return err
		}
		if replayed, ok, err := s.replayAuthorityOrder(ctx, q, in.AdminID, in.RequestID, hash); err != nil || ok {
			result = replayed
			return err
		}
		now := s.now()
		if receivedAt.After(now) || receivedAt.Before(now.Add(-model.AuthorityReceiveLookback)) {
			return invalidOn("ReceivedAt", text.AuthorityOrderReceiveTimeRange())
		}
		for i, t := range targets {
			if t.StoreID == nil {
				continue
			}
			row, err := q.StoreByID(ctx, *t.StoreID)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.AccountID != t.AccountID) {
				return invalidOn(targetField(i, "StoreID"), text.AuthorityOrderTargetStoreMismatch())
			}
			if err != nil {
				return err
			}
		}
		row, err := q.InsertAuthorityOrder(ctx, store.InsertAuthorityOrderParams{
			ID: uuid.NewV7(), Agency: agency, DocumentNo: documentNo, ReceivedAt: receivedAt, Kind: string(in.Kind), Summary: summary, DueAt: dueAt,
			HoldUntil: model.DefaultHoldUntil(receivedAt), Now: now, AdminID: in.AdminID, RequestID: in.RequestID, RequestHash: hash,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			replayed, ok, rerr := s.replayAuthorityOrder(ctx, q, in.AdminID, in.RequestID, hash)
			if rerr != nil {
				return rerr
			}
			if !ok {
				return fmt.Errorf("seller：來函的 request_id %s 衝突，卻查不到原本那一份", in.RequestID)
			}
			result = replayed
			return nil
		}
		if err != nil {
			return err
		}
		for i, t := range targets {
			if err := q.InsertAuthorityOrderTarget(ctx, store.InsertAuthorityOrderTargetParams{
				OrderID: row.ID, Position: int16(i), AccountID: t.AccountID, StoreID: t.StoreID, ProductIds: t.ProductIDs,
			}); err != nil {
				return err
			}
		}
		if err := q.InsertAuthorityOrderPrepurgedStores(ctx, row.ID); err != nil {
			return err
		}
		view, err := readAuthorityOrder(ctx, q, row.ID, now)
		result = model.CreateAuthorityOrderResult{Order: view}
		return err
	})
	return result, err
}

func (s *Service) replayAuthorityOrder(ctx context.Context, q *store.Queries, adminID int64, requestID *uuid.UUID, hash []byte) (model.CreateAuthorityOrderResult, bool, error) {
	if requestID == nil {
		return model.CreateAuthorityOrderResult{}, false, nil
	}
	row, err := q.AuthorityOrderByRequest(ctx, store.AuthorityOrderByRequestParams{AdminID: adminID, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.CreateAuthorityOrderResult{}, false, nil
	}
	if err != nil {
		return model.CreateAuthorityOrderResult{}, false, err
	}
	if !bytes.Equal(row.RequestHash, hash) {
		return model.CreateAuthorityOrderResult{}, false, invalidOn("RequestID", text.InquiryRequestIdReused())
	}
	view, err := readAuthorityOrder(ctx, q, row.ID, s.now())
	return model.CreateAuthorityOrderResult{Order: view, Replayed: true}, true, err
}

func (s *Service) lockedAuthorityOrder(ctx context.Context, q *store.Queries, id uuid.UUID, expected int64, lockAccounts bool) (model.AuthorityOrder, error) {
	if _, err := q.AuthorityOrderByID(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return model.AuthorityOrder{}, errAuthorityOrderNotFound()
	} else if err != nil {
		return model.AuthorityOrder{}, err
	}
	targets, err := q.AuthorityOrderTargets(ctx, []uuid.UUID{id})
	if err != nil {
		return model.AuthorityOrder{}, err
	}
	if lockAccounts {
		accounts := make([]int64, len(targets))
		for i, t := range targets {
			accounts[i] = t.AccountID
		}
		if err := stint.LockAccounts(ctx, q, accounts); err != nil {
			return model.AuthorityOrder{}, err
		}
	}
	row, err := q.LockAuthorityOrder(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AuthorityOrder{}, errAuthorityOrderNotFound()
	}
	if err != nil {
		return model.AuthorityOrder{}, err
	}
	if row.Version != expected {
		return model.AuthorityOrder{}, errAuthorityOrderVersion()
	}
	order := toAuthorityOrder(row)
	for _, t := range targets {
		order.Targets = append(order.Targets, toAuthorityTarget(t))
	}
	return order, nil
}

type noticeValue struct {
	kind       model.AuthorityNoticeKind
	notifiedAt *time.Time
	note       *string
	recheckOn  *time.Time
}

func validateNotice(order model.AuthorityOrder, target model.AuthorityOrderTarget, in model.AuthorityNoticeInput, allowed []model.AuthorityNoticeKind, now time.Time, derived []store.AuthorityOrderMeasureTargetsRow, field string) (noticeValue, error) {
	if !slices.Contains(allowed, in.Kind) {
		return noticeValue{}, invalidOn(field+".Kind", text.AuthorityOrderTargetNoticeRequired())
	}
	v := noticeValue{kind: in.Kind}
	if note := strings.TrimSpace(in.Note); note != "" {
		if utf8.RuneCountInString(note) > model.AuthorityNoteMaxRunes {
			return noticeValue{}, invalidOn(field+".Note", text.AuthorityOrderNoticeNoteLength())
		}
		v.note = &note
	}
	switch in.Kind {
	case model.AuthorityNoticeNotified:
		if in.NotifiedAt == nil || in.NotifiedAt.IsZero() {
			return noticeValue{}, invalidOn(field+".NotifiedAt", text.AuthorityOrderNotifyTimeRange())
		}
		at := in.NotifiedAt.UTC().Truncate(time.Microsecond)
		if at.Before(order.ReceivedAt) || at.After(now) {
			return noticeValue{}, invalidOn(field+".NotifiedAt", text.AuthorityOrderNotifyTimeRange())
		}
		v.notifiedAt = &at
	case model.AuthorityNoticeWithheld:
		if in.RecheckOn == nil || in.RecheckOn.IsZero() {
			return noticeValue{}, invalidOn(field+".RecheckOn", text.AuthorityOrderRecheckDateRequired())
		}
		day := civilDay(*in.RecheckOn)
		v.recheckOn = &day
	case model.AuthorityNoticeMeasure:
		named := slices.ContainsFunc(derived, func(d store.AuthorityOrderMeasureTargetsRow) bool { return target.Names(d.AccountID, d.StoreID) })
		if !order.Kind.MeasureCanNotify() || !named {
			return noticeValue{}, invalidOn(field+".Kind", text.AuthorityOrderMeasureIsNoticeInvalid())
		}
	}
	return v, nil
}

// CompleteAuthorityOrder 標記來函完成：處理摘要、每個對象的告知（剛好一筆，依 targets 的位置），調取資料的來函可以帶實際提供資料的時間（不設是現在，必須在 [收文, 現在]），
// 保存期限延到「提供時間＋AuthorityHold」（比目前晚才延，記一列 DATA_PROVIDED；取收文與提供兩者較晚的，設計 Q5 (b)）。完成不解除保存。etag 必須相符。
// 鎖順序：被指名帳號的店家列 → 來函列。已完成回 failed_precondition。
func (s *Service) CompleteAuthorityOrder(ctx context.Context, in model.CompleteAuthorityOrderInput) (model.AdminAuthorityOrder, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.AdminAuthorityOrder{}, err
	}
	summary := strings.TrimSpace(in.ResolutionSummary)
	if n := utf8.RuneCountInString(summary); n == 0 || n > model.AuthoritySummaryMaxRunes {
		return model.AdminAuthorityOrder{}, invalidOn("ResolutionSummary", text.AuthorityOrderResolutionLength())
	}
	var result model.AdminAuthorityOrder
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		order, err := s.lockedAuthorityOrder(ctx, q, in.ID, in.ExpectedVersion, true)
		if err != nil {
			return err
		}
		if err := authorityBlocked(order.Availability().Complete); err != nil {
			return err
		}
		now := s.now()
		hold := order.HoldUntil
		var extended bool
		switch {
		case order.Kind != model.AuthorityDataRequest && in.ProvideTime != nil:
			return invalidOn("ProvideTime", text.AuthorityOrderProvideTimeNotDataRequest())
		case order.Kind == model.AuthorityDataRequest:
			provide := now
			if in.ProvideTime != nil {
				provide = in.ProvideTime.UTC().Truncate(time.Microsecond)
			}
			if provide.Before(order.ReceivedAt) || provide.After(now) {
				return invalidOn("ProvideTime", text.AuthorityOrderProvideTimeRange())
			}
			hold, extended = model.HoldAfterProvision(order.HoldUntil, provide)
		}
		notices, err := s.targetNotices(ctx, q, order, in.TargetNotices, now)
		if err != nil {
			return err
		}
		if _, err := q.CompleteAuthorityOrder(ctx, store.CompleteAuthorityOrderParams{
			ID: in.ID, ExpectedVersion: order.Version, Now: now, AdminID: in.AdminID, ResolutionSummary: summary, HoldUntil: hold,
		}); errors.Is(err, pgx.ErrNoRows) {
			return errAuthorityOrderVersion()
		} else if err != nil {
			return err
		}
		for position, v := range notices {
			n, err := q.SetAuthorityTargetNotice(ctx, store.SetAuthorityTargetNoticeParams{
				OrderID: in.ID, Position: int16(position), Notice: string(v.kind), NotifiedAt: v.notifiedAt, NoticeNote: v.note, RecheckOn: v.recheckOn,
				Now: ptrTime(now), AdminID: &in.AdminID,
			})
			if err != nil {
				return err
			}
			if n != 1 {
				return fmt.Errorf("seller：來函 %s 的對象 %d 已有告知，完成卻沒有擋下", in.ID, position)
			}
		}
		if extended {
			if err := q.InsertAuthorityHoldExtension(ctx, store.InsertAuthorityHoldExtensionParams{
				OrderID: in.ID, FromUntil: order.HoldUntil, ToUntil: hold, Cause: string(model.HoldExtensionDataProvided), AdminID: in.AdminID, Now: now,
			}); err != nil {
				return err
			}
		}
		result, err = readAuthorityOrder(ctx, q, in.ID, now)
		return err
	})
	return result, err
}

func (s *Service) targetNotices(ctx context.Context, q *store.Queries, order model.AuthorityOrder, in []model.TargetNoticeInput, now time.Time) ([]noticeValue, error) {
	if len(in) != len(order.Targets) {
		return nil, invalidOn("TargetNotices", text.AuthorityOrderTargetNoticeRequired())
	}
	byPosition := make([]*model.AuthorityNoticeInput, len(order.Targets))
	for i, n := range in {
		if n.Position < 0 || n.Position >= len(order.Targets) || byPosition[n.Position] != nil {
			return nil, invalidOn(apperr.IndexedField("TargetNotices", i), text.AuthorityOrderTargetNoticeRequired())
		}
		byPosition[n.Position] = &in[i].Notice
	}
	var derived []store.AuthorityOrderMeasureTargetsRow
	if slices.ContainsFunc(in, func(n model.TargetNoticeInput) bool { return n.Notice.Kind == model.AuthorityNoticeMeasure }) {
		var err error
		if derived, err = q.AuthorityOrderMeasureTargets(ctx, &order.ID); err != nil {
			return nil, err
		}
	}
	out := make([]noticeValue, len(order.Targets))
	for position, n := range byPosition {
		i := slices.IndexFunc(in, func(x model.TargetNoticeInput) bool { return x.Position == position })
		v, err := validateNotice(order, order.Targets[position], *n, model.AuthorityNoticeKindValues(), now, derived, apperr.IndexedField("TargetNotices", i)+".Notice")
		if err != nil {
			return nil, err
		}
		out[position] = v
	}
	return out, nil
}

// ExtendAuthorityOrderHold 延長保存期限（只能延長：新值必須晚於目前，否則 failed_precondition），在同一個 transaction 追加一列 ADMIN 的延長歷史（前值、新值、原因、作者）。
// 已撤銷登記的來函不能延長（Availability().ExtendHold）。已完成或已到期的來函也能延長：資料可能還沒被清（店家還開著，或剛到期而清理還沒跑），訴訟也可能在完成之後才發生；逐家的涵蓋狀態照實顯示已清理的店家。
// 鎖順序：被指名帳號的店家列 → 來函列。
func (s *Service) ExtendAuthorityOrderHold(ctx context.Context, in model.ExtendAuthorityOrderHoldInput) (model.AdminAuthorityOrder, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.AdminAuthorityOrder{}, err
	}
	reason := trimtext.Trim(in.Reason)
	if n := utf8.RuneCountInString(reason); n == 0 || n > model.AuthorityNoteMaxRunes {
		return model.AdminAuthorityOrder{}, invalidOn("Reason", text.AuthorityOrderHoldReasonLength())
	}
	to := in.HoldUntil.UTC().Truncate(time.Microsecond)
	var result model.AdminAuthorityOrder
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		order, err := s.lockedAuthorityOrder(ctx, q, in.ID, in.ExpectedVersion, true)
		if err != nil {
			return err
		}
		if err := authorityBlocked(order.Availability().ExtendHold); err != nil {
			return err
		}
		if !to.After(order.HoldUntil) {
			return apperr.FailedPrecondition("%s", text.AuthorityOrderHoldNotLater().Text)
		}
		now := s.now()
		if _, err := q.ExtendAuthorityOrderHold(ctx, store.ExtendAuthorityOrderHoldParams{ID: in.ID, ExpectedVersion: order.Version, HoldUntil: to}); errors.Is(err, pgx.ErrNoRows) {
			return errAuthorityOrderVersion()
		} else if err != nil {
			return err
		}
		if err := q.InsertAuthorityHoldExtension(ctx, store.InsertAuthorityHoldExtensionParams{
			OrderID: in.ID, FromUntil: order.HoldUntil, ToUntil: to, Cause: string(model.HoldExtensionAdmin), Reason: &reason, AdminID: in.AdminID, Now: now,
		}); err != nil {
			return err
		}
		result, err = readAuthorityOrder(ctx, q, in.ID, now)
		return err
	})
	return result, err
}

// RecordAuthorityTargetNotice 記下暫不告知的對象後來的告知：改成已告知，或寫新的確認日期。只有目前是暫不告知的對象能改（Availability().RecordNotice，
// API 的 can_record_notice 是同一份）；被取代的值在同一個 transaction 寫進歷史
// （誰、何時取代的）。讓來函的版本前進。不鎖店家列：不改保存。
func (s *Service) RecordAuthorityTargetNotice(ctx context.Context, in model.RecordAuthorityTargetNoticeInput) (model.AdminAuthorityOrder, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.AdminAuthorityOrder{}, err
	}
	var result model.AdminAuthorityOrder
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		order, err := s.lockedAuthorityOrder(ctx, q, in.ID, in.ExpectedVersion, false)
		if err != nil {
			return err
		}
		if in.Position < 0 || in.Position >= len(order.Targets) {
			return invalidOn("TargetPosition", text.AuthorityOrderTargetPositionInvalid())
		}
		target := order.Targets[in.Position]
		if err := authorityBlocked(order.Availability().RecordNotice[in.Position]); err != nil {
			return err
		}
		now := s.now()
		v, err := validateNotice(order, target, in.Notice, []model.AuthorityNoticeKind{model.AuthorityNoticeNotified, model.AuthorityNoticeWithheld}, now, nil, "Notice")
		if err != nil {
			return err
		}
		if err := q.InsertAuthorityTargetNoticeHistory(ctx, store.InsertAuthorityTargetNoticeHistoryParams{
			OrderID: in.ID, Position: int16(in.Position), Now: now, AdminID: in.AdminID, //nolint:gosec // 位置已檢查在 0 到 len(Targets)（最多 20）之內
		}); err != nil {
			return err
		}
		n, err := q.ReplaceWithheldAuthorityTargetNotice(ctx, store.ReplaceWithheldAuthorityTargetNoticeParams{
			OrderID: in.ID, Position: int16(in.Position), Notice: string(v.kind), //nolint:gosec // 同上
			NotifiedAt: v.notifiedAt, NoticeNote: v.note, RecheckOn: v.recheckOn,
			Now: ptrTime(now), AdminID: &in.AdminID,
		})
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("seller：來函 %s 的對象 %d 不是暫不告知，卻沒有擋下", in.ID, in.Position)
		}
		if _, err := q.BumpAuthorityOrder(ctx, store.BumpAuthorityOrderParams{ID: in.ID, ExpectedVersion: order.Version}); errors.Is(err, pgx.ErrNoRows) {
			return errAuthorityOrderVersion()
		} else if err != nil {
			return err
		}
		result, err = readAuthorityOrder(ctx, q, in.ID, now)
		return err
	})
	return result, err
}

// RevokeAuthorityOrder 撤銷登記錯誤的來函（Availability().Revoke：還沒完成、還沒撤銷），必須附原因；撤銷之後保存解除（StoreHolds 不看已撤銷的來函）、
// 不在處理中、不能再當作措施的來源，紀錄（時間、管理員、原因）保留。etag 必須相符。冪等以管理員為範圍：這份來函已由同一位管理員撤銷時回傳目前的紀錄與
// AlreadyRevoked（重送，不比較 etag 與原因）；已由別的管理員撤銷或已完成回 failed_precondition（先問能不能撤銷、再比 etag，與其他指令同序，所以拿撤銷之前的 etag 的第二位管理員得到 failed_precondition）。不鎖店家列：解除保存只會讓清理更早可以進行，清理在鎖住候選之後才問保存，
// 看到的是撤銷之前或之後的其中一個，兩者都正確。鎖順序：來函列（建立措施以 FOR SHARE 讀同一列，所以撤銷與建立措施串行）。
func (s *Service) RevokeAuthorityOrder(ctx context.Context, in model.RevokeAuthorityOrderInput) (model.RevokeAuthorityOrderResult, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.RevokeAuthorityOrderResult{}, err
	}
	reason := trimtext.Trim(in.Reason)
	if n := utf8.RuneCountInString(reason); n == 0 || n > model.AuthorityNoteMaxRunes {
		return model.RevokeAuthorityOrderResult{}, invalidOn("Reason", text.AuthorityOrderRevokeReasonLength())
	}
	var result model.RevokeAuthorityOrderResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		row, err := q.LockAuthorityOrder(ctx, in.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errAuthorityOrderNotFound()
		}
		if err != nil {
			return err
		}
		now := s.now()
		if row.RevokedByAdminID != nil && *row.RevokedByAdminID == in.AdminID {
			view, err := readAuthorityOrder(ctx, q, in.ID, now)
			result = model.RevokeAuthorityOrderResult{Order: view, AlreadyRevoked: true}
			return err
		}
		if err := authorityBlocked(toAuthorityOrder(row).Availability().Revoke); err != nil {
			return err
		}
		if row.Version != in.ExpectedVersion {
			return errAuthorityOrderVersion()
		}
		if _, err := q.RevokeAuthorityOrder(ctx, store.RevokeAuthorityOrderParams{
			ID: in.ID, ExpectedVersion: row.Version, Now: now, AdminID: in.AdminID, Reason: reason,
		}); errors.Is(err, pgx.ErrNoRows) {
			return errAuthorityOrderVersion()
		} else if err != nil {
			return err
		}
		view, err := readAuthorityOrder(ctx, q, in.ID, now)
		result = model.RevokeAuthorityOrderResult{Order: view}
		return err
	})
	return result, err
}

// AuthorityOrder 是一份來函的後台檢視（含對象逐家的涵蓋狀態、告知與延長歷史）；不存在回 NotFound。
func (s *Service) AuthorityOrder(ctx context.Context, id uuid.UUID) (model.AdminAuthorityOrder, error) {
	var out model.AdminAuthorityOrder
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		var err error
		out, err = readAuthorityOrder(ctx, store.New(s.d.DB.Conn(ctx)), id, s.now())
		return err
	})
	return out, err
}

// AuthorityOrders 是來函清單：OPEN 依期限舊到新，其餘依收文時間新到舊（keyset）；多讀一筆判斷有沒有下一頁。清單的來函帶對象與涵蓋狀態，不帶延長歷史。
func (s *Service) AuthorityOrders(ctx context.Context, in model.AuthorityOrderListInput) (model.AdminAuthorityOrderPage, error) {
	size, err := pageSize(in.PageSize)
	if err != nil {
		return model.AdminAuthorityOrderPage{}, err
	}
	params := store.AdminAuthorityOrdersParams{PageSize: size + 1}
	if in.State != nil {
		if !slices.Contains(model.AuthorityOrderStateValues(), *in.State) {
			return model.AdminAuthorityOrderPage{}, invalidOn("State", text.ListStateInvalid())
		}
		state := string(*in.State)
		params.State = &state
	}
	if in.AccountID != 0 {
		if err := model.Positive(in.AccountID); err != nil {
			return model.AdminAuthorityOrderPage{}, err
		}
		params.AccountID = &in.AccountID
	}
	if in.After != nil {
		params.AfterAt, params.AfterID = &in.After.At, &in.After.ID
	}
	open := in.State != nil && *in.State == model.AuthorityStateOpen
	var page model.AdminAuthorityOrderPage
	err = s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		rows, err := q.AdminAuthorityOrders(ctx, params)
		if err != nil {
			return err
		}
		more := len(rows) > int(size)
		if more {
			rows = rows[:size]
		}
		if page.Items, err = readAuthorityOrders(ctx, q, rows, false, s.now()); err != nil {
			return err
		}
		if more {
			last := rows[size-1]
			cursor := model.AuthorityCursor{At: last.ReceivedAt, ID: last.ID}
			if open {
				cursor.At = last.DueAt
			}
			page.Next = &cursor
		}
		return nil
	})
	return page, err
}

func civilDay(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
