package reviewservice

import (
	"context"
	"errors"
	"slices"
	"testing"

	"connectrpc.com/connect"
	reviewpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/review"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/review/reviewconnect"
)

type fakeReviews struct {
	reviewconnect.ReviewServiceClient
	response *reviewpb.ListStoreRatingStatsResponse
	err      error
	seen     [][]string
}

func (f *fakeReviews) ListStoreRatingStats(ctx context.Context, req *connect.Request[reviewpb.ListStoreRatingStatsRequest]) (*connect.Response[reviewpb.ListStoreRatingStatsResponse], error) {
	if _, ok := ctx.Deadline(); !ok {
		return nil, errors.New("missing deadline")
	}
	f.seen = append(f.seen, req.Msg.GetStoreIds())
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.response), nil
}

func stat(id, count, avg string) *reviewpb.StoreRatingStat {
	return &reviewpb.StoreRatingStat{StoreId: id, TotalCount: count, AvgRating: avg}
}

func resp(stats ...*reviewpb.StoreRatingStat) *reviewpb.ListStoreRatingStatsResponse {
	return &reviewpb.ListStoreRatingStatsResponse{Stats: stats}
}

func TestStoreRatingStats(t *testing.T) {
	f := &fakeReviews{response: resp(stat("200", "22", "4.77"))}
	got, err := (&Client{rpc: f}).StoreRatingStats(context.Background(), []int64{200, 300, 200})
	if err != nil {
		t.Fatal(err)
	}
	// 沒有評論的店（300）不在 map 裡；重複的 ID 只送一次。
	if len(got) != 1 || got[200] != (RatingStats{Count: 22, Average: 4.77}) {
		t.Fatalf("got=%+v", got)
	}
	if len(f.seen) != 1 || !slices.Equal(f.seen[0], []string{"200", "300"}) {
		t.Fatalf("seen=%v", f.seen)
	}
}

func TestStoreRatingStatsEmptyBatchDoesNotCall(t *testing.T) {
	f := &fakeReviews{err: errors.New("must not be called")}
	got, err := (&Client{rpc: f}).StoreRatingStats(context.Background(), nil)
	if err != nil || len(got) != 0 || len(f.seen) != 0 {
		t.Fatalf("got=%v err=%v seen=%v", got, err, f.seen)
	}
}

func TestStoreRatingStatsFailsClosed(t *testing.T) {
	tooMany := make([]int64, MaxStoreIDs+1)
	for i := range tooMany {
		tooMany[i] = int64(i + 1)
	}
	for name, tc := range map[string]struct {
		ids []int64
		f   *fakeReviews
	}{
		"bad store":        {[]int64{1, 0}, &fakeReviews{response: resp()}},
		"too many":         {tooMany, &fakeReviews{response: resp()}},
		"transport error":  {[]int64{1}, &fakeReviews{err: errors.New("down")}},
		"business code":    {[]int64{1}, &fakeReviews{response: &reviewpb.ListStoreRatingStatsResponse{Code: 500}}},
		"unasked store":    {[]int64{1}, &fakeReviews{response: resp(stat("2", "1", "5"))}},
		"unparsable store": {[]int64{1}, &fakeReviews{response: resp(stat("x", "1", "5"))}},
		"duplicate store":  {[]int64{1}, &fakeReviews{response: resp(stat("1", "1", "5"), stat("1", "2", "4"))}},
		"bad count":        {[]int64{1}, &fakeReviews{response: resp(stat("1", "x", "4"))}},
		"zero count":       {[]int64{1}, &fakeReviews{response: resp(stat("1", "0", "0"))}},
		"negative count":   {[]int64{1}, &fakeReviews{response: resp(stat("1", "-1", "4"))}},
		"empty count":      {[]int64{1}, &fakeReviews{response: resp(stat("1", "", "4"))}},
		"bad average":      {[]int64{1}, &fakeReviews{response: resp(stat("1", "3", "abc"))}},
		"nan average":      {[]int64{1}, &fakeReviews{response: resp(stat("1", "3", "NaN"))}},
		"average over 5":   {[]int64{1}, &fakeReviews{response: resp(stat("1", "3", "5.5"))}},
		"negative average": {[]int64{1}, &fakeReviews{response: resp(stat("1", "3", "-1"))}},
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := (&Client{rpc: tc.f}).StoreRatingStats(context.Background(), tc.ids); err == nil {
				t.Fatalf("accepted: %+v", got)
			}
		})
	}
}

func TestNewRequiresAddress(t *testing.T) {
	if _, err := New("", "s"); err == nil {
		t.Fatal("empty address accepted")
	}
}
