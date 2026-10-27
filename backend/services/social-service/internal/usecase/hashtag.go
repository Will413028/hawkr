package usecase

import (
	"context"
	"sort"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/domain/hashtag"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

// HashtagUseCase orchestrates normalize → upsert → diff-apply → counter delta.
// See spec 34 §4 (Counter maintenance) and §5.4 (Apply flow).
type HashtagUseCase struct {
	repo domain.HashtagRepository
	tsid func() int64
}

func NewHashtagUseCase(repo domain.HashtagRepository, tsid func() int64) *HashtagUseCase {
	return &HashtagUseCase{repo: repo, tsid: tsid}
}

// ApplyResult holds the diff between previous and target bindings.
type ApplyResult struct {
	Added   []int64              // hashtag IDs newly bound (ASC)
	Removed []int64              // hashtag IDs unbound (ASC)
	Final   []hashtag.Normalized // final canonical+display list in target order
}

// Apply normalizes raw tags, upserts into cms_hashtag, diffs against current
// post bindings, and writes adds/removes/kept-updates. When publicVisible is
// true, applies counter delta (+1 added, -1 removed) in hashtag.id ASC order
// to avoid deadlock with concurrent Apply calls (spec 34 §4.2).
//
// q may be nil (uses pool) or a pgx.Tx for transactional callers. ListByPostID
// is tx-aware (accepts q) so a caller running Apply inside its own transaction
// — e.g. CreatePost (#2773) — reads current bindings through that same
// connection instead of taking a second one from the pool: doing otherwise
// deadlocks the pool once concurrent callers >= MaxConns, each holding its tx
// connection while blocked waiting for a second.
func (uc *HashtagUseCase) Apply(ctx context.Context, q gen.DBTX, postID int64, rawTags []string, publicVisible bool) (ApplyResult, error) {
	normalized, err := hashtag.RequireSet(rawTags)
	if err != nil {
		return ApplyResult{}, err
	}

	// Read current bindings BEFORE any writes (position-ordered).
	current, err := uc.repo.ListByPostID(ctx, q, postID)
	if err != nil {
		return ApplyResult{}, err
	}
	currentByID := make(map[int64]gen.CmsPostHashtag, len(current))
	for _, row := range current {
		currentByID[row.HashtagID] = row
	}

	// Upsert each canonical → id (preserving input order via parallel slice).
	type upserted struct {
		id        int64
		canonical string
		display   string
	}
	targets := make([]upserted, 0, len(normalized))
	for _, n := range normalized {
		id, err := uc.repo.UpsertByName(ctx, q, uc.tsid(), n.Canonical)
		if err != nil {
			return ApplyResult{}, err
		}
		targets = append(targets, upserted{id, n.Canonical, n.Display})
	}

	// Build target set + diff.
	targetByID := make(map[int64]upserted, len(targets))
	for _, t := range targets {
		// dedup at this layer is defensive — RequireSet already dedupes.
		if _, ok := targetByID[t.id]; !ok {
			targetByID[t.id] = t
		}
	}

	addedIDs := make([]int64, 0)
	removedIDs := make([]int64, 0)
	for id := range targetByID {
		if _, ok := currentByID[id]; !ok {
			addedIDs = append(addedIDs, id)
		}
	}
	for id := range currentByID {
		if _, ok := targetByID[id]; !ok {
			removedIDs = append(removedIDs, id)
		}
	}
	sort.Slice(addedIDs, func(i, j int) bool { return addedIDs[i] < addedIDs[j] })
	sort.Slice(removedIDs, func(i, j int) bool { return removedIDs[i] < removedIDs[j] })

	// 1. Delete removed bindings.
	if len(removedIDs) > 0 {
		if err := uc.repo.DeleteBindings(ctx, q, postID, removedIDs); err != nil {
			return ApplyResult{}, err
		}
	}

	// 2. Insert new + update kept rows for position/display.
	var pos int16
	for _, t := range targets {
		existing, kept := currentByID[t.id]
		if !kept {
			if err := uc.repo.InsertBinding(ctx, q, postID, t.id, t.display, pos); err != nil {
				return ApplyResult{}, err
			}
		} else if existing.DisplayName != t.display || existing.Position != pos {
			if err := uc.repo.UpdateBinding(ctx, q, postID, t.id, t.display, pos); err != nil {
				return ApplyResult{}, err
			}
		}
		pos++
	}

	// 3. Counter delta (only if publicly visible). Merge added (+1) and
	// removed (-1) into a single ID-sorted pass — spec §4.2 requires lock
	// acquisition order to be ASC across the UNION of touched IDs, not two
	// independent ASC passes (which could deadlock with a concurrent Apply
	// whose added/removed sets are inverted).
	if publicVisible {
		type counterDelta struct {
			id    int64
			delta int32
		}
		deltas := make([]counterDelta, 0, len(addedIDs)+len(removedIDs))
		for _, id := range addedIDs {
			deltas = append(deltas, counterDelta{id, 1})
		}
		for _, id := range removedIDs {
			deltas = append(deltas, counterDelta{id, -1})
		}
		// added/removed sets are disjoint, so id is unique across the merged slice.
		sort.Slice(deltas, func(i, j int) bool { return deltas[i].id < deltas[j].id })
		for _, d := range deltas {
			if err := uc.repo.AdjustPostCount(ctx, q, d.id, d.delta); err != nil {
				return ApplyResult{}, err
			}
		}
	}

	final := make([]hashtag.Normalized, 0, len(targets))
	for _, t := range targets {
		final = append(final, hashtag.Normalized{Canonical: t.canonical, Display: t.display})
	}
	return ApplyResult{Added: addedIDs, Removed: removedIDs, Final: final}, nil
}

// GetTagsByPostIDs returns canonical hashtag names per post for batch list/detail
// rendering. Result map key = postID; value = []string of canonical names in
// position order (per ListByPostIDs SQL ORDER BY post_id, position).
//
// Returns (nil, nil) for empty input to match the Go convention used by other
// batch helpers in this codebase. When postIDs is non-empty but no bindings
// exist, returns an empty map (not nil) so callers can distinguish "no input"
// from "no tags".
//
// Cost: 2 SQL calls regardless of list size (ListByPostIDs + ListHashtagsByIDs)
// — avoids N+1.
func (uc *HashtagUseCase) GetTagsByPostIDs(ctx context.Context, postIDs []int64) (map[int64][]string, error) {
	if len(postIDs) == 0 {
		return nil, nil
	}
	bindings, err := uc.repo.ListByPostIDs(ctx, postIDs)
	if err != nil {
		return nil, err
	}
	// Collect all hashtag_ids referenced.
	idSet := make(map[int64]struct{})
	for _, rows := range bindings {
		for _, r := range rows {
			idSet[r.HashtagID] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return map[int64][]string{}, nil
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	tagByID, err := uc.repo.ListHashtagsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64][]string, len(bindings))
	for postID, rows := range bindings {
		// rows are already ordered by post_id, position from the SQL ORDER BY.
		names := make([]string, 0, len(rows))
		for _, r := range rows {
			if h, ok := tagByID[r.HashtagID]; ok {
				names = append(names, h.Name)
			}
		}
		out[postID] = names
	}
	return out, nil
}

// AdjustCountersForVisibilityChange applies +delta to every hashtag bound to
// the post, in hashtag.id ASC order (deadlock-free). Used for hide / restore /
// soft-delete flows.
//
// Returns the hashtag IDs whose post_count moved, so callers can bust the
// trending-hashtag cache for exactly those (後端服務責任邊界規範 §9.3). On error
// the returned slice holds the IDs adjusted before the failure — those counters
// are already committed, so busting them is still correct.
//
// delta=0 is a no-op (returns nil, nil without touching the repo).
func (uc *HashtagUseCase) AdjustCountersForVisibilityChange(ctx context.Context, q gen.DBTX, postID int64, delta int32) ([]int64, error) {
	if delta == 0 {
		return nil, nil
	}
	// ListHashtagIDsByPostID (like ListByPostID) is tx-aware on HashtagRepository —
	// returns IDs in ASC order so we can iterate directly.
	ids, err := uc.repo.ListHashtagIDsByPostID(ctx, q, postID)
	if err != nil {
		return nil, err
	}
	adjusted := make([]int64, 0, len(ids))
	for _, id := range ids {
		if err := uc.repo.AdjustPostCount(ctx, q, id, delta); err != nil {
			return adjusted, err
		}
		adjusted = append(adjusted, id)
	}
	return adjusted, nil
}

// ============================================================
// Search (auto-complete) — spec 34 §5.1
// ============================================================

// SearchMode toggles between prefix and contain matching.
type SearchMode int

const (
	SearchModePrefix  SearchMode = 0
	SearchModeContain SearchMode = 1
)

const (
	defaultSearchLimit   = 10
	maxSearchLimit       = 20
	defaultPopularLimit  = 20
	maxPopularLimit      = 50
	maxPopularRangeYrs   = 1
	defaultPopularMonths = 1
)

// SuggestItem is one auto-complete row.
type SuggestItem struct {
	Name      string
	PostCount int32
}

// Search returns auto-complete suggestions. The query is server-normalized
// (any case, leading '#' stripped); a normalize-to-empty input returns nil
// rather than an error so frontends can render "no suggestions" naturally.
// limit < 1 → default 10; limit > 20 → cap 20.
func (uc *HashtagUseCase) Search(ctx context.Context, q string, mode SearchMode, limit int32) ([]SuggestItem, error) {
	canonical, _, ok := hashtag.Normalize(q)
	if !ok {
		return nil, nil
	}
	if limit < 1 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}
	if mode == SearchModeContain {
		rows, err := uc.repo.SearchByContain(ctx, canonical, limit)
		if err != nil {
			return nil, err
		}
		out := make([]SuggestItem, len(rows))
		for i, r := range rows {
			out[i] = SuggestItem{Name: r.Name, PostCount: r.PostCount}
		}
		return out, nil
	}
	rows, err := uc.repo.SearchByPrefix(ctx, canonical, limit)
	if err != nil {
		return nil, err
	}
	out := make([]SuggestItem, len(rows))
	for i, r := range rows {
		out[i] = SuggestItem{Name: r.Name, PostCount: r.PostCount}
	}
	return out, nil
}

// ============================================================
// Popular (trending) — spec 34 §5.2
// ============================================================

// PopularItem is one trending row.
type PopularItem struct {
	Hashtag string
	Count   int32
}

const popularDateFormat = "2006-01-02"

// Popular returns trending hashtags for the [startRaw, endRaw] inclusive
// range (both YYYY-MM-DD, both optional per §5.2.2). SQL bounds are
// left-closed, right-open: [start_at = startDate 00:00 UTC, end_at = (endDate+1) 00:00 UTC).
//
// Validation failures (parse / range order / range > 1y) return a *DomainError
// (InvalidInput) — caller wraps as gateway 400.
func (uc *HashtagUseCase) Popular(ctx context.Context, startRaw, endRaw string, limit int32) ([]PopularItem, error) {
	if limit < 1 {
		limit = defaultPopularLimit
	}
	if limit > maxPopularLimit {
		limit = maxPopularLimit
	}

	startDate, endDate, err := resolvePopularRange(startRaw, endRaw, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	sqlStart := startDate
	sqlEnd := endDate.AddDate(0, 0, 1) // end-exclusive

	rows, err := uc.repo.PopularInRange(ctx, sqlStart, sqlEnd, limit)
	if err != nil {
		return nil, err
	}
	out := make([]PopularItem, len(rows))
	for i, r := range rows {
		out[i] = PopularItem{Hashtag: r.Hashtag, Count: r.Count}
	}
	return out, nil
}

// resolvePopularRange computes [startDate, endDate] (both inclusive, days)
// per spec §5.2.2. now should be the current UTC instant (truncated to day
// internally). Returns DomainError(InvalidInput) on validation failures.
func resolvePopularRange(startRaw, endRaw string, now time.Time) (start, end time.Time, err error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	parse := func(s, field string) (time.Time, error) {
		t, e := time.ParseInLocation(popularDateFormat, s, time.UTC)
		if e != nil {
			return time.Time{}, pkgerrors.NewInvalidInput("invalid date format: " + field + " must be YYYY-MM-DD")
		}
		return t, nil
	}

	switch {
	case startRaw == "" && endRaw == "":
		end = today
		start = today.AddDate(0, -defaultPopularMonths, 0)
	case startRaw != "" && endRaw == "":
		start, err = parse(startRaw, "start_from")
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		plus1y := start.AddDate(maxPopularRangeYrs, 0, 0)
		if plus1y.Before(today) {
			end = plus1y
		} else {
			end = today
		}
	case startRaw == "" && endRaw != "":
		end, err = parse(endRaw, "end_to")
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		// Clamp future end to today — mirrors the start-only branch and
		// avoids issuing SQL that scans a guaranteed-empty future range.
		if end.After(today) {
			end = today
		}
		start = end.AddDate(-maxPopularRangeYrs, 0, 0)
	default:
		start, err = parse(startRaw, "start_from")
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		end, err = parse(endRaw, "end_to")
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	if start.After(end) {
		return time.Time{}, time.Time{}, pkgerrors.NewInvalidInput("start_from must not be later than end_to")
	}
	if end.After(start.AddDate(maxPopularRangeYrs, 0, 0)) {
		return time.Time{}, time.Time{}, pkgerrors.NewInvalidInput("popular query range must not exceed 1 year")
	}
	return start, end, nil
}
