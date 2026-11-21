package seller

import (
	"context"
	"errors"
	"uuid"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// LinkAvailability 回報內容快照的連結目標（貼文、討論串）此刻還在不在（social.Module 實作）。措施快照屬於 seller、連結目標屬於 social：
// seller 模組不讀 social，由這一層在讀取措施時把兩邊接起來（A7-D29）。
type LinkAvailability interface {
	AvailableLinks(ctx context.Context, postIDs []uuid.UUID, threads []social.ForumThread) (social.LinkAvailability, error)
}

var errLinksRequired = errors.New("httpapi/seller：措施有內容快照的連結，但沒有 LinkAvailability 可查目標還在不在")

// snapshotLink 回傳內容快照的連結；沒有快照或連結是零值（聊天訊息、評價回覆沒有連結目標）時第二個回傳值為假。
func snapshotLink(c model.MeasureContent) (model.ContentLink, bool) {
	if c.Snapshot == nil || c.Snapshot.Link == (model.ContentLink{}) {
		return model.ContentLink{}, false
	}
	return c.Snapshot.Link, true
}

// markLinkAvailability 把 ms 的內容快照連結目標是否還在寫進對應的 ContentRef.LinkAvailable：整批只問 social 一次（每類目標一個查詢），沒有連結目標時不問。
// refsOf(i) 回傳 ms[i] 轉成回應後的 Contents，順序與 ms[i].Contents 相同。
func markLinkAvailability(ctx context.Context, src LinkAvailability, ms []model.Measure, refsOf func(i int) []*pb.ContentRef) error {
	var posts []uuid.UUID
	var threads []social.ForumThread
	seenPost, seenThread := map[uuid.UUID]bool{}, map[social.ForumThread]bool{}
	for _, m := range ms {
		for _, c := range m.Contents {
			l, ok := snapshotLink(c)
			if !ok {
				continue
			}
			if l.PostID != uuid.Nil() {
				if !seenPost[l.PostID] {
					seenPost[l.PostID] = true
					posts = append(posts, l.PostID)
				}
			} else if t := (social.ForumThread{StoreID: l.StoreID, ThreadID: l.ThreadID}); !seenThread[t] {
				seenThread[t] = true
				threads = append(threads, t)
			}
		}
	}
	if len(posts) == 0 && len(threads) == 0 {
		return nil
	}
	if src == nil {
		return errLinksRequired
	}
	av, err := src.AvailableLinks(ctx, posts, threads)
	if err != nil {
		return err
	}
	for i, m := range ms {
		refs := refsOf(i)
		for j, c := range m.Contents {
			l, ok := snapshotLink(c)
			if !ok {
				continue
			}
			if l.PostID != uuid.Nil() {
				refs[j].LinkAvailable = av.Posts[l.PostID]
			} else {
				refs[j].LinkAvailable = av.Threads[social.ForumThread{StoreID: l.StoreID, ThreadID: l.ThreadID}]
			}
		}
	}
	return nil
}

// sellerMeasures 轉成賣家看到的措施（含內容快照連結是否還在）。
func sellerMeasures(ctx context.Context, src LinkAvailability, ms []model.Measure) ([]*pb.Measure, error) {
	l := uitext.LocaleFrom(ctx)
	out := make([]*pb.Measure, len(ms))
	for i, m := range ms {
		out[i] = toMeasure(l, m)
	}
	if err := markLinkAvailability(ctx, src, ms, func(i int) []*pb.ContentRef { return out[i].Contents }); err != nil {
		return nil, err
	}
	return out, nil
}

// adminMeasures 轉成後台看到的措施（含內容快照連結是否還在）。
func adminMeasures(ctx context.Context, src LinkAvailability, ms []model.Measure) ([]*adminpb.Measure, error) {
	out := toAdminMeasures(ms)
	if err := markLinkAvailability(ctx, src, ms, func(i int) []*pb.ContentRef { return out[i].Contents }); err != nil {
		return nil, err
	}
	return out, nil
}

func adminMeasure(ctx context.Context, src LinkAvailability, m model.Measure) (*adminpb.Measure, error) {
	out, err := adminMeasures(ctx, src, []model.Measure{m})
	if err != nil {
		return nil, err
	}
	return out[0], nil
}

func sellerMeasure(ctx context.Context, src LinkAvailability, m model.Measure) (*pb.Measure, error) {
	out, err := sellerMeasures(ctx, src, []model.Measure{m})
	if err != nil {
		return nil, err
	}
	return out[0], nil
}
