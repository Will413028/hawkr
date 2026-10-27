package pagination

// integer is the set of integer types page/pageSize are typically stored as
// across services: usecase-layer code tends to use int, proto-generated
// request fields use int32, and database-facing code may use int64.
type integer interface {
	~int | ~int32 | ~int64
}

// NormalizePage clamps a 1-based page number and page size to sane bounds
// and derives the SQL OFFSET for them:
//   - page <= 0 becomes 1
//   - pageSize <= 0 becomes defaultSize
//   - pageSize > maxSize is clamped down to maxSize
//
// defaultSize and maxSize are caller-supplied rather than fixed constants
// because page-size policy already differs by endpoint in this codebase
// (e.g. review-service allows up to 500 results per page, most others cap at
// 100) — this only consolidates the repeated clamping/offset arithmetic
// duplicated across services, not the policy itself.
//
// Note: some existing call sites reset pageSize to defaultSize (instead of
// clamping to maxSize) when it exceeds maxSize. NormalizePage always clamps
// to maxSize. Migrating an existing call site to NormalizePage is a
// behavior change for an over-limit pageSize and should be a deliberate,
// reviewed decision, not a silent side effect of adopting this helper.
func NormalizePage[T integer](page, pageSize, defaultSize, maxSize T) (normPage, normPageSize, offset T) {
	normPage = page
	if normPage <= 0 {
		normPage = 1
	}

	normPageSize = pageSize
	switch {
	case normPageSize <= 0:
		normPageSize = defaultSize
	case normPageSize > maxSize:
		normPageSize = maxSize
	}

	offset = (normPage - 1) * normPageSize
	return normPage, normPageSize, offset
}

// NormalizePageSize applies NormalizePage's pageSize clamping alone, for
// callers with no page number to normalize — cursor-based pagination or a
// plain Top-N query, where there is no offset to compute.
func NormalizePageSize[T integer](pageSize, defaultSize, maxSize T) T {
	_, normPageSize, _ := NormalizePage(1, pageSize, defaultSize, maxSize)
	return normPageSize
}
