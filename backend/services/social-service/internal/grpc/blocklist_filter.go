package grpc

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/blocklist"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/redis/go-redis/v9"
)

// filterPostsByBlocklist drops posts whose seller is mutually blocked with viewer.
// viewerID == 0 means "anonymous viewer" — no filtering (anon already sees public content).
func filterPostsByBlocklist(ctx context.Context, rdb *redis.Client, viewerID int64, posts []*gen.CmsPost) []*gen.CmsPost {
	if viewerID == 0 || len(posts) == 0 {
		return posts
	}
	out := posts[:0]
	for _, p := range posts {
		if blocklist.IsBlocked(ctx, rdb, viewerID, p.SellerID) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// filterCommentsByBlocklist drops comments whose author is mutually blocked with viewer.
func filterCommentsByBlocklist(ctx context.Context, rdb *redis.Client, viewerID int64, comments []*gen.CmsComment) []*gen.CmsComment {
	if viewerID == 0 || len(comments) == 0 {
		return comments
	}
	out := comments[:0]
	for _, c := range comments {
		if blocklist.IsBlocked(ctx, rdb, viewerID, c.UserID) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// filterForumByBlocklist drops forum entries whose author is mutually blocked with viewer.
func filterForumByBlocklist(ctx context.Context, rdb *redis.Client, viewerID int64, entries []*gen.CmsSellerForum) []*gen.CmsSellerForum {
	if viewerID == 0 || len(entries) == 0 {
		return entries
	}
	out := entries[:0]
	for _, e := range entries {
		if blocklist.IsBlocked(ctx, rdb, viewerID, e.UserID) {
			continue
		}
		out = append(out, e)
	}
	return out
}
