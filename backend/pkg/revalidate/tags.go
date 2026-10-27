package revalidate

import "strconv"

// Tag builders mirror hawkr-website/lib/seo/tags.ts (SSOT).
// Keep these in sync with the documented webhook contract in 後端服務責任邊界規範 §9.

// ProductTag returns the cache tag for a product id ("product:<id>").
func ProductTag(productID int64) string {
	return "product:" + strconv.FormatInt(productID, 10)
}

// SellerTag returns the cache tag for a seller id ("seller:<id>").
func SellerTag(sellerID int64) string {
	return "seller:" + strconv.FormatInt(sellerID, 10)
}

// PostTag returns the cache tag for a post id ("post:<id>").
func PostTag(postID int64) string {
	return "post:" + strconv.FormatInt(postID, 10)
}

// HashtagTag returns the cache tag for a hashtag id ("hashtag:<id>").
//
// Deliberately keyed on the numeric id, never on the tag name. The website's
// allow-list only accepts entity tags matching
// `^(?:product|seller|post|category|hashtag):[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`
// (hawkr-website/lib/api/revalidate-request.ts), and a hashtag's canonical
// name keeps CJK and full-width characters verbatim
// (social-service/internal/domain/hashtag/normalizer.go Normalize). A name-keyed
// tag would therefore fail validation for any Chinese hashtag — and the endpoint
// rejects the *whole* request on one invalid tag, so it would also silently drop
// the post:<id> tags travelling in the same POST. The id is always ASCII digits.
//
// No web fetcher reads hashtag:<id> itself; it exists so the revalidate boundary
// expands it to the hashtags:list aggregate (producers never send domain
// collections directly — 後端服務責任邊界規範 §9.8).
func HashtagTag(hashtagID int64) string {
	return "hashtag:" + strconv.FormatInt(hashtagID, 10)
}

// HashtagTags returns one tag per hashtag id, skipping non-positive ids and
// collapsing repeats. See HashtagTag for why ids (not names) key these.
func HashtagTags(hashtagIDs ...int64) []string {
	return tagsFor(HashtagTag, hashtagIDs)
}

// CategoryTag returns the cache tag for a category id ("category:<id>").
func CategoryTag(categoryID int64) string {
	return "category:" + strconv.FormatInt(categoryID, 10)
}

// SellerTags returns the tags for every id space one seller may be cached
// under, in the order given.
//
// One seller needs several tags because the website keys its seller caches by
// the URL path segment (hawkr-website/lib/api/cached.ts feeds the [id]
// segment straight into sellerTag(id)), and a seller's public ids —
// store_code (canonical storefront segment), store_id (the segment legacy
// links still carry) — are independently minted values with no functional
// relation to each other or to the account_id that write paths hold. A bust
// keyed on one of them leaves the pages cached under the others serving stale
// data until their revalidate window expires.
//
// Non-positive ids are skipped and repeats collapse, so callers may pass
// whatever ids they hold without pre-filtering: a seller whose ids coincide
// (grandfathered rows) yields a single tag, and an unresolved id yields none.
func SellerTags(sellerIDs ...int64) []string {
	return tagsFor(SellerTag, sellerIDs)
}

// tagsFor maps ids through build in the order given, skipping non-positive ids
// (an unresolved id must not become the tag "seller:0") and collapsing repeats
// so a caller may pass whatever ids it holds without pre-filtering.
func tagsFor(build func(int64) string, ids []int64) []string {
	tags := make([]string, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		tags = append(tags, build(id))
	}
	return tags
}
