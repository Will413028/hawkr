//go:build integration

package notification

import (
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1"
)

const (
	buyer = pb.NotificationAudience_NOTIFICATION_AUDIENCE_BUYER
	store = pb.NotificationAudience_NOTIFICATION_AUDIENCE_STORE
)

func listAll(t *testing.T, s *stack, account int64, audience pb.NotificationAudience, category *pb.NotificationCategory, size int32, lang ...string) (ids []string, totals []int32) {
	t.Helper()
	token := ""
	for range 20 {
		resp, err := s.client.ListInboxItems(t.Context(), as(connect.NewRequest(&pb.ListInboxItemsRequest{Audience: audience, Category: category, PageSize: size, PageToken: token}), account, lang...))
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range resp.Msg.GetItems() {
			ids = append(ids, it.GetId())
		}
		totals = append(totals, resp.Msg.GetTotalSize())
		if token = resp.Msg.GetNextPageToken(); token == "" {
			return ids, totals
		}
	}
	t.Fatal("分頁沒有結束")
	return nil, nil
}

// 收件匣：新到舊、cursor 分頁讀完不重複不遺漏、total_size 是篩選下的全部筆數；只有自己與該受眾的列，沒有註冊的型別不顯示也不計數。
func TestListInboxItemsPaginatesNewestFirstAndTotalFollowsTheFilter(t *testing.T) {
	t.Parallel()
	s := setup(t)
	var want []string
	for i := range 5 {
		typ, params := "account_suspended", paramsSusp
		if i%2 == 1 {
			typ, params = "account_unsuspended", paramsLift
		}
		want = append([]string{s.item(t, 7, "BUYER", 0, typ, params, i, nil)}, want...)
	}
	s.item(t, 8, "BUYER", 0, "account_suspended", paramsSusp, 10, nil)
	s.item(t, 9, "STORE", 90, "account_suspended", paramsSusp, 11, nil)
	s.item(t, 7, "BUYER", 0, "retired_type", `{}`, 12, nil)

	for _, size := range []int32{1, 2, 5, 0} {
		ids, totals := listAll(t, s, 7, buyer, nil, size)
		if !slices.Equal(ids, want) {
			t.Fatalf("size=%d：%v，預期 %v（新到舊、不重複、不遺漏、只有自己的買家列、沒有退場型別）", size, ids, want)
		}
		for _, n := range totals {
			if n != 5 {
				t.Fatalf("size=%d：total_size = %d，預期 5", size, n)
			}
		}
	}

	account := pb.NotificationCategory_NOTIFICATION_CATEGORY_ACCOUNT
	if ids, totals := listAll(t, s, 7, buyer, &account, 2); !slices.Equal(ids, want) || totals[0] != 5 {
		t.Fatalf("ACCOUNT 篩選：%v %v", ids, totals)
	}
	// 篩選到沒有型別的分類：沒有列，total_size 是 0（不是整個收件匣的 5）。
	chat := pb.NotificationCategory_NOTIFICATION_CATEGORY_CHAT
	if ids, totals := listAll(t, s, 7, buyer, &chat, 2); len(ids) != 0 || totals[0] != 0 {
		t.Fatalf("CHAT 篩選：%v %v，預期沒有列、total_size 0", ids, totals)
	}
}

func TestListInboxItemsRejectsBadRequests(t *testing.T) {
	t.Parallel()
	s := setup(t)
	ctx := t.Context()
	payout := pb.NotificationCategory_NOTIFICATION_CATEGORY_PAYOUT
	unspecified := pb.NotificationCategory_NOTIFICATION_CATEGORY_UNSPECIFIED
	for name, tc := range map[string]struct {
		req   *pb.ListInboxItemsRequest
		field string
	}{
		"沒給受眾": {&pb.ListInboxItemsRequest{}, "audience"},
		"分類不屬於受眾（買家沒有撥款）": {&pb.ListInboxItemsRequest{Audience: buyer, Category: &payout}, "category"},
		"分類是 UNSPECIFIED": {&pb.ListInboxItemsRequest{Audience: buyer, Category: &unspecified}, "category"},
		"筆數是負數":           {&pb.ListInboxItemsRequest{Audience: buyer, PageSize: -1}, "page_size"},
		"page_token 亂寫":   {&pb.ListInboxItemsRequest{Audience: buyer, PageToken: "garbage"}, ""},
	} {
		_, err := s.client.ListInboxItems(ctx, as(connect.NewRequest(tc.req), 7))
		if tc.field == "" {
			wantCode(t, err, connect.CodeInvalidArgument)
		} else if got := invalidField(t, err); got != tc.field {
			t.Errorf("%s：欄位 = %q，預期 %q", name, got, tc.field)
		}
	}
	// page_token 換了 audience 或 category 就不能沿用。
	for i := range 3 {
		s.item(t, 9, "STORE", 90, "account_suspended", paramsSusp, i, nil)
		s.item(t, 9, "BUYER", 0, "account_suspended", paramsSusp, i, nil)
	}
	first, err := s.client.ListInboxItems(ctx, as(connect.NewRequest(&pb.ListInboxItemsRequest{Audience: buyer, PageSize: 1}), 9))
	if err != nil || first.Msg.GetNextPageToken() == "" {
		t.Fatal(err)
	}
	account := pb.NotificationCategory_NOTIFICATION_CATEGORY_ACCOUNT
	token := first.Msg.GetNextPageToken()
	for name, c := range map[string]struct {
		account int64
		req     *pb.ListInboxItemsRequest
	}{
		"換受眾": {9, &pb.ListInboxItemsRequest{Audience: store, PageSize: 1, PageToken: token}},
		"換分類": {9, &pb.ListInboxItemsRequest{Audience: buyer, Category: &account, PageSize: 1, PageToken: token}},
		"換帳號": {10, &pb.ListInboxItemsRequest{Audience: buyer, PageSize: 1, PageToken: token}},
	} {
		if _, err := s.client.ListInboxItems(ctx, as(connect.NewRequest(c.req), c.account)); err == nil {
			t.Errorf("%s：page_token 不能沿用", name)
		} else {
			wantCode(t, err, connect.CodeInvalidArgument)
		}
	}
}

// 文字依請求語系由伺服器組好；已讀的列有 read 與 read_time，未讀沒有。
func TestListInboxItemsRendersTheRequestLocale(t *testing.T) {
	t.Parallel()
	s := setup(t)
	readAt := epoch.Add(time.Hour)
	s.item(t, 7, "BUYER", 0, "account_unsuspended", paramsLift, 1, &readAt)
	s.item(t, 7, "BUYER", 0, "account_suspended", paramsSusp, 2, nil)

	get := func(lang string) []*pb.InboxItem {
		resp, err := s.client.ListInboxItems(t.Context(), as(connect.NewRequest(&pb.ListInboxItemsRequest{Audience: buyer}), 7, lang))
		if err != nil {
			t.Fatal(err)
		}
		return resp.Msg.GetItems()
	}
	zh, en := get("zh-TW"), get("en")
	if len(zh) != 2 || len(en) != 2 {
		t.Fatalf("zh=%d en=%d 則", len(zh), len(en))
	}
	if zh[0].GetTitle() != "帳號已被停權" || en[0].GetTitle() != "Your account has been suspended" {
		t.Fatalf("標題 zh=%q en=%q", zh[0].GetTitle(), en[0].GetTitle())
	}
	if zh[1].GetTitle() != "帳號已解除停權" || en[1].GetTitle() != "Your account suspension has been lifted" {
		t.Fatalf("標題 zh=%q en=%q", zh[1].GetTitle(), en[1].GetTitle())
	}
	if zh[0].GetBody() == "" || en[0].GetBody() == zh[0].GetBody() {
		t.Fatalf("內文要依語系：zh=%q en=%q", zh[0].GetBody(), en[0].GetBody())
	}
	if zh[0].GetCategory() != pb.NotificationCategory_NOTIFICATION_CATEGORY_ACCOUNT || zh[0].GetPresentation() != pb.InboxPresentation_INBOX_PRESENTATION_ACCOUNT {
		t.Fatalf("分類 %v 呈現 %v", zh[0].GetCategory(), zh[0].GetPresentation())
	}
	if zh[0].GetRead() || zh[0].ReadTime != nil || !zh[1].GetRead() || !zh[1].GetReadTime().AsTime().Equal(readAt) {
		t.Fatalf("已讀欄位：未讀 %v %v、已讀 %v %v", zh[0].GetRead(), zh[0].ReadTime, zh[1].GetRead(), zh[1].GetReadTime())
	}
	if zh[0].GetDeepLink() != "" || zh[0].GetImage() != nil {
		t.Fatalf("停權通知沒有深連結也沒有圖：%q %v", zh[0].GetDeepLink(), zh[0].GetImage())
	}
	if !zh[0].GetCreateTime().AsTime().Equal(epoch.Add(2 * time.Minute)) {
		t.Fatalf("create_time = %v", zh[0].GetCreateTime().AsTime())
	}
}

// 未讀數：不分分類，只算自己、該受眾、未讀、已註冊型別的列。
func TestGetUnreadCountCountsOnlyTheCallersUnreadForTheAudience(t *testing.T) {
	t.Parallel()
	s := setup(t)
	readAt := epoch
	s.item(t, 9, "BUYER", 0, "account_suspended", paramsSusp, 1, nil)
	s.item(t, 9, "BUYER", 0, "account_suspended", paramsSusp, 2, nil)
	s.item(t, 9, "BUYER", 0, "account_suspended", paramsSusp, 3, &readAt)
	s.item(t, 9, "BUYER", 0, "retired_type", `{}`, 4, nil)
	s.item(t, 9, "STORE", 90, "account_suspended", paramsSusp, 5, nil)
	s.item(t, 9, "STORE", 91, "account_suspended", paramsSusp, 6, nil)
	s.item(t, 8, "BUYER", 0, "account_suspended", paramsSusp, 7, nil)
	for aud, want := range map[pb.NotificationAudience]int32{buyer: 2, store: 1} {
		resp, err := s.client.GetUnreadCount(t.Context(), as(connect.NewRequest(&pb.GetUnreadCountRequest{Audience: aud}), 9))
		if err != nil || resp.Msg.GetUnreadCount() != want {
			t.Fatalf("%v：%v %v，預期 %d", aud, resp.Msg, err, want)
		}
	}
}

// 已讀：已讀的再標成功且不改已讀時間；不存在與不是自己的回完全相同的 not_found，而且不會被標成已讀。
func TestMarkInboxItemReadIsOwnerOnlyAndIdempotent(t *testing.T) {
	t.Parallel()
	s := setup(t)
	mine := s.item(t, 7, "BUYER", 0, "account_suspended", paramsSusp, 1, nil)
	theirs := s.item(t, 8, "BUYER", 0, "account_suspended", paramsSusp, 1, nil)
	read := func(account int64, id string) error {
		_, err := s.client.MarkInboxItemRead(t.Context(), as(connect.NewRequest(&pb.MarkInboxItemReadRequest{Id: id}), account))
		return err
	}
	if err := read(7, mine); err != nil {
		t.Fatal(err)
	}
	first := s.count(t, `SELECT count(*) FROM notification.inbox_items WHERE id = $1 AND read_at = $2`, mine, epoch)
	s.now = epoch.Add(time.Hour)
	if err := read(7, mine); err != nil {
		t.Fatalf("已讀的再標：%v", err)
	}
	if first != 1 || s.count(t, `SELECT count(*) FROM notification.inbox_items WHERE id = $1 AND read_at = $2`, mine, epoch) != 1 {
		t.Fatal("已讀的再標不能改已讀時間")
	}

	notOwner, missing := read(7, theirs), read(7, uuid.NewV7().String())
	a, b := errCode(t, notOwner, connect.CodeNotFound), errCode(t, missing, connect.CodeNotFound)
	if a.Message() != b.Message() || len(a.Details()) != len(b.Details()) {
		t.Fatalf("不是自己的與不存在的回應要完全相同：%v / %v", a, b)
	}
	if s.count(t, `SELECT count(*) FROM notification.inbox_items WHERE id = $1 AND read_at IS NOT NULL`, theirs) != 0 {
		t.Fatal("別人的通知不能被標成已讀")
	}
	if got := invalidField(t, read(7, "not-a-uuid")); got != "id" {
		t.Fatalf("欄位 = %q", got)
	}
}

// 全部已讀：只改該帳號、該受眾的未讀，回傳這次改的筆數；再呼叫是 0。
func TestMarkAllInboxItemsReadUpdatesOnlyTheAudience(t *testing.T) {
	t.Parallel()
	s := setup(t)
	readAt := epoch
	s.item(t, 9, "BUYER", 0, "account_suspended", paramsSusp, 1, nil)
	s.item(t, 9, "BUYER", 0, "account_suspended", paramsSusp, 2, nil)
	s.item(t, 9, "BUYER", 0, "account_suspended", paramsSusp, 3, &readAt)
	s.item(t, 9, "STORE", 90, "account_suspended", paramsSusp, 4, nil)
	s.item(t, 8, "BUYER", 0, "account_suspended", paramsSusp, 5, nil)
	mark := func(aud pb.NotificationAudience) int32 {
		resp, err := s.client.MarkAllInboxItemsRead(t.Context(), as(connect.NewRequest(&pb.MarkAllInboxItemsReadRequest{Audience: aud}), 9))
		if err != nil {
			t.Fatal(err)
		}
		return resp.Msg.GetUpdatedCount()
	}
	if n := mark(buyer); n != 2 {
		t.Fatalf("updated_count = %d，預期 2", n)
	}
	if n := mark(buyer); n != 0 {
		t.Fatalf("再呼叫 updated_count = %d，預期 0", n)
	}
	if s.count(t, `SELECT count(*) FROM notification.inbox_items WHERE read_at IS NULL`) != 2 {
		t.Fatal("STORE 受眾與別人的帳號的未讀不能被動到")
	}
	if n := mark(store); n != 1 {
		t.Fatalf("STORE updated_count = %d，預期 1", n)
	}
}

// 刪除：全有或全無；空、重複、超過 100 個是 invalid_argument；任何一個不存在或不是自己的整批 not_found、一則都不刪。
func TestBatchDeleteInboxItemsIsAllOrNothing(t *testing.T) {
	t.Parallel()
	s := setup(t)
	a := s.item(t, 7, "BUYER", 0, "account_suspended", paramsSusp, 1, nil)
	b := s.item(t, 7, "BUYER", 0, "account_suspended", paramsSusp, 2, nil)
	c := s.item(t, 7, "BUYER", 0, "account_suspended", paramsSusp, 3, nil)
	theirs := s.item(t, 8, "BUYER", 0, "account_suspended", paramsSusp, 4, nil)
	del := func(ids ...string) error {
		_, err := s.client.BatchDeleteInboxItems(t.Context(), as(connect.NewRequest(&pb.BatchDeleteInboxItemsRequest{Ids: ids}), 7))
		return err
	}
	rows := func() int { return s.count(t, `SELECT count(*) FROM notification.inbox_items`) }

	wantCode(t, del(a, theirs), connect.CodeNotFound)
	wantCode(t, del(a, uuid.NewV7().String()), connect.CodeNotFound)
	if rows() != 4 {
		t.Fatalf("任何一個不存在或不是自己的，一則都不能刪：剩 %d 列", rows())
	}
	invalidField(t, del())
	invalidField(t, del(a, a))
	many := make([]string, 101)
	for i := range many {
		many[i] = uuid.NewV7().String()
	}
	invalidField(t, del(many...))
	if got := invalidField(t, del("nope")); got != "ids" {
		t.Fatalf("欄位 = %q", got)
	}
	if rows() != 4 {
		t.Fatalf("無效請求不能刪東西：剩 %d 列", rows())
	}
	if err := del(a, b); err != nil {
		t.Fatal(err)
	}
	if rows() != 2 || s.count(t, `SELECT count(*) FROM notification.inbox_items WHERE id IN ($1, $2)`, c, theirs) != 2 {
		t.Fatalf("只該刪 a、b：剩 %d 列", rows())
	}
}

// 受眾：STORE 受眾要呼叫者此刻有店，沒有店每支帶 audience 的方法都回 not_found；有店時只看得到那間店的列。
func TestStoreAudienceNeedsACurrentStoreAndIsIsolated(t *testing.T) {
	t.Parallel()
	s := setup(t)
	ctx := t.Context()
	s.item(t, 9, "STORE", 90, "account_suspended", paramsSusp, 1, nil)
	s.item(t, 9, "STORE", 89, "account_suspended", paramsSusp, 2, nil)
	s.item(t, 9, "BUYER", 0, "account_suspended", paramsSusp, 3, nil)
	s.item(t, 7, "STORE", 70, "account_suspended", paramsSusp, 4, nil)

	for name, call := range map[string]func() error{
		"ListInboxItems": func() error {
			_, err := s.client.ListInboxItems(ctx, as(connect.NewRequest(&pb.ListInboxItemsRequest{Audience: store}), 7))
			return err
		},
		"GetUnreadCount": func() error {
			_, err := s.client.GetUnreadCount(ctx, as(connect.NewRequest(&pb.GetUnreadCountRequest{Audience: store}), 7))
			return err
		},
		"MarkAllInboxItemsRead": func() error {
			_, err := s.client.MarkAllInboxItemsRead(ctx, as(connect.NewRequest(&pb.MarkAllInboxItemsReadRequest{Audience: store}), 7))
			return err
		},
		"GetNotificationPreferences": func() error {
			_, err := s.client.GetNotificationPreferences(ctx, as(connect.NewRequest(&pb.GetNotificationPreferencesRequest{Audience: store}), 7))
			return err
		},
		"UpdateNotificationPreference": func() error {
			_, err := s.client.UpdateNotificationPreference(ctx, as(connect.NewRequest(&pb.UpdateNotificationPreferenceRequest{
				Audience: store, Category: pb.NotificationCategory_NOTIFICATION_CATEGORY_ANNOUNCEMENT, Channel: pb.NotificationChannel_NOTIFICATION_CHANNEL_PUSH,
			}), 7))
			return err
		},
	} {
		wantCode(t, call(), connect.CodeNotFound)
		_ = name
	}
	// 沒有店的帳號 7 的 STORE 列（舊店）沒有被動到。
	if s.count(t, `SELECT count(*) FROM notification.inbox_items WHERE account_id = 7 AND read_at IS NOT NULL`) != 0 {
		t.Fatal("沒有店時 MarkAll 不能動到任何列")
	}

	ids, totals := listAll(t, s, 9, store, nil, 0)
	if len(ids) != 1 || totals[0] != 1 {
		t.Fatalf("STORE 受眾只看得到現任店（90）的列：%v %v", ids, totals)
	}
	if ids, _ := listAll(t, s, 9, buyer, nil, 0); len(ids) != 1 {
		t.Fatalf("BUYER 受眾看不到 STORE 列：%v", ids)
	}
}

// 偏好：entries 只有註冊表實際送達的組合（各受眾的型別所走的分類與渠道，公告是收件匣與推播）；
// 沒有型別走的分類與渠道不列、也不能設定。
func TestPreferencesListOnlyTheDeliveredPairs(t *testing.T) {
	t.Parallel()
	s := setup(t)
	type pair struct {
		c  pb.NotificationCategory
		ch pb.NotificationChannel
	}
	const (
		inbox = pb.NotificationChannel_NOTIFICATION_CHANNEL_INBOX
		push  = pb.NotificationChannel_NOTIFICATION_CHANNEL_PUSH
		email = pb.NotificationChannel_NOTIFICATION_CHANNEL_EMAIL
		acct  = pb.NotificationCategory_NOTIFICATION_CATEGORY_ACCOUNT
		ann   = pb.NotificationCategory_NOTIFICATION_CATEGORY_ANNOUNCEMENT
		chatC = pb.NotificationCategory_NOTIFICATION_CATEGORY_CHAT
		payC  = pb.NotificationCategory_NOTIFICATION_CATEGORY_PAYOUT
		cmtC  = pb.NotificationCategory_NOTIFICATION_CATEGORY_COMMENT
		likeC = pb.NotificationCategory_NOTIFICATION_CATEGORY_LIKE
		folC  = pb.NotificationCategory_NOTIFICATION_CATEGORY_FOLLOW
		revC  = pb.NotificationCategory_NOTIFICATION_CATEGORY_REVIEW
		procC = pb.NotificationCategory_NOTIFICATION_CATEGORY_PROCUREMENT
		fspC  = pb.NotificationCategory_NOTIFICATION_CATEGORY_FOLLOWED_STORE_POST
		rstC  = pb.NotificationCategory_NOTIFICATION_CATEGORY_RESTOCK
		coupC = pb.NotificationCategory_NOTIFICATION_CATEGORY_COUPON
	)
	both := func(c pb.NotificationCategory) []pair { return []pair{{c, inbox}, {c, push}} }
	join := func(parts ...[]pair) []pair {
		var out []pair
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	get := func(account int64, aud pb.NotificationAudience) map[pair]*pb.PreferenceEntry {
		resp, err := s.client.GetNotificationPreferences(t.Context(), as(connect.NewRequest(&pb.GetNotificationPreferencesRequest{Audience: aud}), account))
		if err != nil {
			t.Fatal(err)
		}
		out := map[pair]*pb.PreferenceEntry{}
		for _, e := range resp.Msg.GetEntries() {
			out[pair{e.GetCategory(), e.GetChannel()}] = e
		}
		if len(out) != len(resp.Msg.GetEntries()) {
			t.Fatalf("entries 有重複：%v", resp.Msg.GetEntries())
		}
		return out
	}
	check := func(name string, got map[pair]*pb.PreferenceEntry, want []pair, locked map[pair]bool) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s的 entries = %v，預期 %v", name, got, want)
		}
		for _, p := range want {
			e := got[p]
			if e == nil {
				t.Fatalf("%s缺 %v：%v", name, p, got)
			}
			if e.GetCanDisable() == locked[p] || !e.GetEnabled() {
				t.Errorf("%s %v：can_disable=%v enabled=%v，預期 can_disable=%v、預設開", name, p, e.GetCanDisable(), e.GetEnabled(), !locked[p])
			}
		}
	}
	b := get(9, buyer)
	check("買家", b, join([]pair{{acct, inbox}, {acct, push}, {acct, email}, {chatC, push}}, both(cmtC), both(likeC), both(fspC), both(rstC), both(coupC), both(ann)),
		map[pair]bool{{acct, inbox}: true, {acct, email}: true, {ann, inbox}: true})
	check("店家", get(9, store), join([]pair{{acct, inbox}, {acct, push}, {acct, email}, {payC, inbox}, {payC, push}, {payC, email}, {chatC, push}},
		both(revC), both(cmtC), both(likeC), both(folC), both(procC), both(ann)),
		map[pair]bool{{acct, inbox}: true, {acct, email}: true, {payC, inbox}: true, {payC, email}: true, {ann, inbox}: true})

	update := func(aud pb.NotificationAudience, c pb.NotificationCategory, ch pb.NotificationChannel, enabled bool) (*pb.PreferenceEntry, error) {
		resp, err := s.client.UpdateNotificationPreference(t.Context(), as(connect.NewRequest(&pb.UpdateNotificationPreferenceRequest{Audience: aud, Category: c, Channel: ch, Enabled: enabled}), 9))
		if err != nil {
			return nil, err
		}
		return resp.Msg.GetEntry(), nil
	}
	// 關掉公告推播：回傳更新後的那一項，之後的列表是關；重複設定成功。
	for range 2 {
		e, err := update(buyer, ann, push, false)
		if err != nil || e.GetEnabled() || !e.GetCanDisable() || e.GetCategory() != ann || e.GetChannel() != push {
			t.Fatalf("關公告推播：%v %v", e, err)
		}
	}
	if e := get(9, buyer)[pair{ann, push}]; e.GetEnabled() {
		t.Fatal("公告推播應該是關")
	}
	if s.count(t, `SELECT count(*) FROM notification.preference_overrides WHERE account_id = 9 AND audience = 'BUYER'`) != 1 {
		t.Fatal("關掉之後應有一列覆寫")
	}
	if get(9, store)[pair{ann, push}].GetEnabled() != true {
		t.Fatal("買家的偏好不能影響店家受眾")
	}
	// 設回預設值不留覆寫列。
	if e, err := update(buyer, ann, push, true); err != nil || !e.GetEnabled() {
		t.Fatalf("打開：%v %v", e, err)
	}
	if s.count(t, `SELECT count(*) FROM notification.preference_overrides`) != 0 {
		t.Fatal("設回預設值不該留覆寫列")
	}
	// 不可關閉的組合：關閉是 invalid_argument、打開成功。
	if got := invalidField(t, func() error { _, err := update(buyer, acct, inbox, false); return err }()); got != "enabled" {
		t.Fatalf("欄位 = %q", got)
	}
	invalidField(t, func() error { _, err := update(buyer, ann, inbox, false); return err }())
	if e, err := update(buyer, acct, email, true); err != nil || !e.GetEnabled() || e.GetCanDisable() {
		t.Fatalf("不可關閉的組合打開：%v %v", e, err)
	}
	// 註冊表不會送達的組合：公告沒有 email、CHAT 只有推播；受眾沒有的分類。
	chat, payout := chatC, payC
	for name, c := range map[string]struct {
		aud pb.NotificationAudience
		cat pb.NotificationCategory
		ch  pb.NotificationChannel
		f   string
	}{
		"公告沒有 email":     {buyer, ann, email, "channel"},
		"沒有型別的分類":        {buyer, chat, inbox, "channel"},
		"店家的 CHAT 沒有收件匣": {store, chat, inbox, "channel"},
		"買家沒有撥款":         {buyer, payout, inbox, "category"},
	} {
		if got := invalidField(t, func() error { _, err := update(c.aud, c.cat, c.ch, false); return err }()); got != c.f {
			t.Errorf("%s：欄位 = %q，預期 %q", name, got, c.f)
		}
	}
	if s.count(t, `SELECT count(*) FROM notification.preference_overrides`) != 0 {
		t.Fatal("被拒絕的設定不能寫入")
	}
}

// 裝置 token：註冊 upsert 並更新最後註冊時間；換帳號登入時改綁給呼叫者；註銷只刪自己的，不存在或別人的也成功。
func TestDeviceTokensRebindToTheCallerAndDeleteOnlyOwn(t *testing.T) {
	t.Parallel()
	s := setup(t)
	ctx := t.Context()
	register := func(account int64, token string, app pb.NotificationApp) error {
		_, err := s.client.RegisterDeviceToken(ctx, as(connect.NewRequest(&pb.RegisterDeviceTokenRequest{Token: token, App: app}), account))
		return err
	}
	del := func(account int64, token string) error {
		_, err := s.client.DeleteDeviceToken(ctx, as(connect.NewRequest(&pb.DeleteDeviceTokenRequest{Token: token}), account))
		return err
	}
	const buyerApp, sellerApp = pb.NotificationApp_NOTIFICATION_APP_BUYER_APP, pb.NotificationApp_NOTIFICATION_APP_SELLER_APP
	owner := func(token string) (account int64, app string, at time.Time) {
		if err := s.db.Pool().QueryRow(ctx, `SELECT account_id, app, last_registered_at FROM notification.device_tokens WHERE token = $1`, token).Scan(&account, &app, &at); err != nil {
			t.Fatal(err)
		}
		return account, app, at
	}

	if err := register(7, "tok", buyerApp); err != nil {
		t.Fatal(err)
	}
	if a, app, at := owner("tok"); a != 7 || app != "BUYER" || !at.Equal(epoch) {
		t.Fatalf("註冊後：%d %s %v", a, app, at)
	}
	s.now = epoch.Add(time.Hour)
	if err := register(7, "tok", buyerApp); err != nil {
		t.Fatal(err)
	}
	if _, _, at := owner("tok"); !at.Equal(s.now) || s.count(t, `SELECT count(*) FROM notification.device_tokens`) != 1 {
		t.Fatalf("每次註冊更新最後註冊時間、不新增列：%v", at)
	}
	if err := register(8, "tok", sellerApp); err != nil {
		t.Fatal(err)
	}
	if a, app, _ := owner("tok"); a != 8 || app != "SELLER" || s.count(t, `SELECT count(*) FROM notification.device_tokens`) != 1 {
		t.Fatalf("換帳號登入時改綁給呼叫者：%d %s", a, app)
	}
	// 別人的 token 與不存在的 token：成功，且別人的 token 還在。
	if err := del(7, "tok"); err != nil {
		t.Fatalf("刪別人的 token 也回成功：%v", err)
	}
	if err := del(7, "never-registered"); err != nil {
		t.Fatalf("刪不存在的 token 也回成功：%v", err)
	}
	if a, _, _ := owner("tok"); a != 8 {
		t.Fatal("別人的 token 不能被刪")
	}
	if err := del(8, "tok"); err != nil || s.count(t, `SELECT count(*) FROM notification.device_tokens`) != 0 {
		t.Fatalf("刪自己的 token：%v", err)
	}
	// 輸入檢查。
	for name, c := range map[string]struct {
		err   error
		field string
	}{
		"空 token":     {register(7, "", buyerApp), "token"},
		"token 太長":    {register(7, string(make([]byte, 4097)), buyerApp), "token"},
		"沒給 app":      {register(7, "x", pb.NotificationApp_NOTIFICATION_APP_UNSPECIFIED), "app"},
		"註銷的 token 空": {del(7, ""), "token"},
	} {
		if got := invalidField(t, c.err); got != c.field {
			t.Errorf("%s：欄位 = %q，預期 %q", name, got, c.field)
		}
	}
}

// 這個 service 只給登入的顧客帳號：沒登入是 unauthenticated；管理員（含代行）是 permission_denied；只帶代行憑證（X-HK-Operation-Session）
// 的請求不是登入，handler 不會被呼叫。
func TestOnlyCustomerAccountsReachTheService(t *testing.T) {
	t.Parallel()
	s := setup(t)
	ctx := t.Context()
	s.item(t, 7, "BUYER", 0, "account_suspended", paramsSusp, 1, nil)

	req := connect.NewRequest(&pb.GetUnreadCountRequest{Audience: buyer})
	_, err := s.client.GetUnreadCount(ctx, req)
	wantCode(t, err, connect.CodeUnauthenticated)

	req = connect.NewRequest(&pb.GetUnreadCountRequest{Audience: buyer})
	req.Header().Set("X-HK-Operation-Session", "delegated-session")
	_, err = s.client.GetUnreadCount(ctx, req)
	wantCode(t, err, connect.CodeUnauthenticated)

	req = connect.NewRequest(&pb.GetUnreadCountRequest{Audience: buyer})
	req.Header().Set("X-Test-Principal", "admin:3")
	req.Header().Set("X-HK-Operation-Session", "delegated-session")
	_, err = s.client.GetUnreadCount(ctx, req)
	wantCode(t, err, connect.CodePermissionDenied)

	if _, err := s.client.GetUnreadCount(ctx, as(connect.NewRequest(&pb.GetUnreadCountRequest{Audience: buyer}), 7)); err != nil {
		t.Fatal(err)
	}
}

// 輸入檢查的句子只有模組文字一個來源：依請求語系回，handler 不另寫句子。
func TestValidationSentencesFollowTheRequestLocale(t *testing.T) {
	t.Parallel()
	s := setup(t)
	for lang, want := range map[string]string{"zh-TW": "受眾", "en": "audience is not valid"} {
		_, err := s.client.ListInboxItems(t.Context(), as(connect.NewRequest(&pb.ListInboxItemsRequest{}), 7, lang))
		if got := invalidField(t, err); got != "audience" || !strings.Contains(err.Error(), want) {
			t.Errorf("%s：欄位 %q、錯誤 %v，預期含 %q", lang, got, err, want)
		}
	}
}
