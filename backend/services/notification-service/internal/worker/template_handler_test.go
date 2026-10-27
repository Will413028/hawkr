package worker

import (
	"context"
	"encoding/json"
	"testing"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
)

// --- mock template repository ---

type mockTemplateRepo struct {
	templates []*domain.NotificationTemplate
}

func (r *mockTemplateRepo) FindAll(_ context.Context) ([]*domain.NotificationTemplate, error) {
	return r.templates, nil
}

func (r *mockTemplateRepo) FindEnabled(_ context.Context) ([]*domain.NotificationTemplate, error) {
	var enabled []*domain.NotificationTemplate
	for _, t := range r.templates {
		if t.Enabled {
			enabled = append(enabled, t)
		}
	}
	return enabled, nil
}

func (r *mockTemplateRepo) FindByID(_ context.Context, id int64) (*domain.NotificationTemplate, error) {
	for _, t := range r.templates {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, nil
}

func (r *mockTemplateRepo) Insert(_ context.Context, t *domain.NotificationTemplate) (int64, error) {
	return 1, nil
}

func (r *mockTemplateRepo) Update(_ context.Context, _ *domain.NotificationTemplate) error {
	return nil
}

func (r *mockTemplateRepo) UpdateEnabled(_ context.Context, _ int64, _ bool) error { return nil }

// --- capturing notifier ---

type capturingNotifier struct {
	calls []notifierCall
}

type notifierCall struct {
	data     map[string]string
	title    string
	body     string
	imageURL string
	tokens   []string
}

func (n *capturingNotifier) Send(_ context.Context, tokens []string, title, body, imageURL string, data map[string]string) (map[string]error, error) {
	n.calls = append(n.calls, notifierCall{tokens: tokens, title: title, body: body, imageURL: imageURL, data: data})
	return map[string]error{}, nil
}

func (n *capturingNotifier) SendDataOnly(_ context.Context, _ []string, _ map[string]string) (map[string]error, error) {
	return map[string]error{}, nil
}

func (n *capturingNotifier) SendToTopic(_ context.Context, _, _, _, _ string, _ map[string]string) (string, error) {
	return "test-msg-id", nil
}

// --- helpers ---

func strPtr(s string) *string { return &s }

func buildTestCache(templates []*domain.NotificationTemplate) *TemplateCache {
	repo := &mockTemplateRepo{templates: templates}
	cache := NewTemplateCache(repo)
	if _, err := cache.LoadFromDB(context.Background()); err != nil {
		panic(err)
	}
	return cache
}

func makePayload(pairs ...interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i].(string)] = pairs[i+1]
	}
	return m
}

// --- renderTemplate tests ---

func TestRenderTemplate_AllFieldsPresent(t *testing.T) {
	payload := makePayload("orderNo", "ORD-001", "sellerName", "鮮果農場")
	got, missing := RenderTemplate("訂單 #{orderNo} 由 #{sellerName} 準備中", payload)
	if len(missing) != 0 {
		t.Errorf("expected no missing fields, got %v", missing)
	}
	if got != "訂單 ORD-001 由 鮮果農場 準備中" {
		t.Errorf("unexpected rendered: %q", got)
	}
}

func TestRenderTemplate_MissingField(t *testing.T) {
	payload := makePayload("orderNo", "ORD-001")
	got, missing := RenderTemplate("訂單 #{orderNo}，賣家 #{sellerName}", payload)
	if len(missing) != 1 || missing[0] != "sellerName" {
		t.Errorf("expected missing=[sellerName], got %v", missing)
	}
	if got != "訂單 ORD-001，賣家 #{sellerName}" {
		t.Errorf("unexpected rendered: %q", got)
	}
}

func TestRenderTemplate_NumericValue(t *testing.T) {
	payload := makePayload("buyerId", float64(12345))
	got, _ := RenderTemplate("用戶 #{buyerId}", payload)
	if got != "用戶 12345" {
		t.Errorf("expected '用戶 12345', got %q", got)
	}
}

// 回歸 issue #1316：64-bit TSID id（> 2^53）經 event JSON 進入通知時，
// 必須用 DecodePayload(UseNumber) 保精度；plain json.Unmarshal 走 float64 會四捨五入，
// 導致深連結 postId 指向不存在的貼文（彈窗開不了）+ 收件人 id 失真（通知送錯人）。
func TestDecodePayload_PreservesTSIDPrecision(t *testing.T) {
	const postID = "831982371627364501"           // 奇數，float64 無法精確表示
	const wantAuthorID int64 = 770123456789012345 // 收件人 TSID
	raw := []byte(`{"postId":` + postID + `,"postAuthorId":770123456789012345,"contentPreview":"hi"}`)

	payload, err := DecodePayload(raw)
	if err != nil {
		t.Fatalf("DecodePayload error: %v", err)
	}

	// 1) 深連結資料欄位經 toStringVal 必須精確
	if got := toStringVal(payload["postId"]); got != postID {
		t.Errorf("postId toStringVal = %q, want %q（精度丟失）", got, postID)
	}

	// 2) 收件人欄位經 extractUserID 必須精確
	gotID, err := extractUserID(payload, "postAuthorId")
	if err != nil {
		t.Fatalf("extractUserID error: %v", err)
	}
	if gotID != wantAuthorID {
		t.Errorf("extractUserID = %d, want %d（收件人精度丟失=通知送錯人）", gotID, wantAuthorID)
	}

	// 3) 對照：舊的 plain json.Unmarshal（float64）確實失真，坐實 root cause 與本修復必要性
	var lossy map[string]interface{}
	if err := json.Unmarshal(raw, &lossy); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if toStringVal(lossy["postId"]) == postID {
		t.Errorf("測試前提失效：預期 plain json.Unmarshal 使 postId 失真，卻得到精確值")
	}
}

func TestRenderBraces_SubstitutesDataKey(t *testing.T) {
	data := map[string]string{"order_id": "ORD-999"}
	got := RenderBraces("hawkr://orders/{order_id}", data)
	if got != "hawkr://orders/ORD-999" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestRenderBraces_MissingKeyKeptAsIs(t *testing.T) {
	got := RenderBraces("hawkr://orders/{order_id}", map[string]string{})
	if got != "hawkr://orders/{order_id}" {
		t.Errorf("unexpected: %q", got)
	}
}

// --- TemplateHandler integration tests ---

func TestTemplateHandler_NormalSend(t *testing.T) {
	tmpl := &domain.NotificationTemplate{
		ID:                  1,
		Exchange:            "hawkr.order",
		RoutingKey:          "order.paid",
		EventType:           "order.paid",
		TargetUserType:      "buyer",
		TargetUserField:     "buyer_id",
		Category:            "order_update",
		TitleTemplate:       "訂單成立",
		BodyTemplate:        "感謝您的訂購！訂單 ##{orderNo} 正在處理中。",
		DataType:            "ORDER_PAID",
		DataFields:          map[string]domain.FieldSpec{"order_id": fieldSpec("orderNo")},
		SchemaURL:           strPtr("hawkr://orders/{order_id}"),
		ReferenceIDTemplate: strPtr("#{orderNo}"),
		Enabled:             true,
	}

	cache := buildTestCache([]*domain.NotificationTemplate{tmpl})
	sendPush, pushRepo := newTestSendPushWithRepo()
	inboxUC, inboxRepo := newTestInboxUC()
	logger := zap.NewNop()

	handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.order", "https://test-media.example.com", logger)

	payload := makePayload("buyer_id", float64(42), "orderNo", "ORD-001")
	body, _ := json.Marshal(payload)

	if err := handler(context.Background(), "order.paid", body); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if pushRepo.record == nil {
		t.Fatal("expected a push record to be created")
	}
	if pushRepo.record.UserID != 42 {
		t.Errorf("expected UserID=42, got %d", pushRepo.record.UserID)
	}
	if pushRepo.record.EventType != "order.paid" {
		t.Errorf("expected EventType=order.paid, got %q", pushRepo.record.EventType)
	}
	if pushRepo.record.ReferenceID != "ORD-001" {
		t.Errorf("expected ReferenceID=ORD-001, got %q", pushRepo.record.ReferenceID)
	}

	if inboxRepo.createdItem == nil {
		t.Fatal("expected inbox item to be written")
	}
	if inboxRepo.createdItem.UserID != 42 {
		t.Errorf("inbox UserID = %d, want 42", inboxRepo.createdItem.UserID)
	}
	if inboxRepo.createdItem.EventType != "order.paid" {
		t.Errorf("inbox EventType = %q, want order.paid", inboxRepo.createdItem.EventType)
	}
	if inboxRepo.createdItem.ReferenceID != "ORD-001" {
		t.Errorf("inbox ReferenceID = %q, want ORD-001", inboxRepo.createdItem.ReferenceID)
	}
}

func TestTemplateHandler_FallbackBody(t *testing.T) {
	fallback := "您有新通知，請開啟 App 查看。"
	tmpl := &domain.NotificationTemplate{
		ID:              1,
		Exchange:        "hawkr.content",
		RoutingKey:      "follow.added",
		EventType:       "follow.added",
		TargetUserType:  "seller",
		TargetUserField: "merchantId",
		Category:        "follower",
		TitleTemplate:   "你有新粉絲！",
		BodyTemplate:    "#{followerName} 開始追蹤你了！",
		BodyFallback:    &fallback,
		DataType:        "FOLLOWER_ADDED",
		Enabled:         true,
	}

	cache := buildTestCache([]*domain.NotificationTemplate{tmpl})
	sendPush, pushRepo := newTestSendPushWithRepo()

	inboxUC, _ := newTestInboxUC()
	handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.content", "https://test-media.example.com", zap.NewNop())

	// followerName is missing from payload → should use fallback
	payload := makePayload("merchantId", float64(99))
	body, _ := json.Marshal(payload)

	if err := handler(context.Background(), "follow.added", body); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if pushRepo.record == nil {
		t.Fatal("expected push record")
	}
	if pushRepo.record.Body != fallback {
		t.Errorf("expected fallback body, got %q", pushRepo.record.Body)
	}
}

func TestTemplateHandler_MissingUserIDSkips(t *testing.T) {
	tmpl := &domain.NotificationTemplate{
		ID:              1,
		Exchange:        "hawkr.order",
		RoutingKey:      "order.paid",
		EventType:       "order.paid",
		TargetUserType:  "buyer",
		TargetUserField: "buyer_id",
		TitleTemplate:   "訂單成立",
		BodyTemplate:    "您的訂單已成立",
		DataType:        "ORDER_PAID",
		Enabled:         true,
	}

	cache := buildTestCache([]*domain.NotificationTemplate{tmpl})
	sendPush, pushRepo := newTestSendPushWithRepo()

	inboxUC, _ := newTestInboxUC()
	handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.order", "https://test-media.example.com", zap.NewNop())

	// buyer_id missing → handler should skip without creating push record
	payload := makePayload("orderNo", "ORD-001")
	body, _ := json.Marshal(payload)

	if err := handler(context.Background(), "order.paid", body); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if pushRepo.record != nil {
		t.Error("expected no push record when user_id field is missing")
	}
}

func TestTemplateHandler_NoTemplateFound(t *testing.T) {
	cache := buildTestCache(nil) // empty cache
	sendPush, pushRepo := newTestSendPushWithRepo()

	inboxUC, _ := newTestInboxUC()
	handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.order", "https://test-media.example.com", zap.NewNop())

	if err := handler(context.Background(), "order.paid", []byte(`{}`)); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if pushRepo.record != nil {
		t.Error("expected no push record when no template found")
	}
}

func TestTemplateHandler_CategoryInDataPayload(t *testing.T) {
	tmpl := &domain.NotificationTemplate{
		ID:              1,
		Exchange:        "hawkr.content",
		RoutingKey:      "product.viewed",
		EventType:       "product.viewed",
		TargetUserType:  "seller",
		TargetUserField: "sellerId",
		Category:        "follower",
		TitleTemplate:   "瀏覽您的商品",
		BodyTemplate:    "有人正在看您的商品",
		DataType:        "PRODUCT_VIEWED",
		DataFields:      map[string]domain.FieldSpec{"product_id": fieldSpec("productId")},
		Enabled:         true,
	}

	cache := buildTestCache([]*domain.NotificationTemplate{tmpl})
	sendPush, pushRepo := newTestSendPushWithRepo()

	inboxUC, _ := newTestInboxUC()
	handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.content", "https://test-media.example.com", zap.NewNop())

	payload := makePayload("sellerId", float64(100), "productId", float64(999), "productName", "有機蘋果")
	body, _ := json.Marshal(payload)

	if err := handler(context.Background(), "product.viewed", body); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if pushRepo.record == nil {
		t.Fatal("expected push record")
	}
	if pushRepo.record.Data["category"] != "follower" {
		t.Errorf("expected data[category]=follower, got %q", pushRepo.record.Data["category"])
	}
	if pushRepo.record.Data["type"] != "PRODUCT_VIEWED" {
		t.Errorf("expected data[type]=PRODUCT_VIEWED, got %q", pushRepo.record.Data["type"])
	}
}

func TestTemplateHandler_InboxWrittenEvenWhenNoTokens(t *testing.T) {
	tmpl := &domain.NotificationTemplate{
		ID:                  1,
		Exchange:            "hawkr.payment",
		RoutingKey:          "payment.success",
		EventType:           "payment.success",
		TargetUserType:      "buyer",
		TargetUserField:     "notify_buyer_account_id",
		Category:            "order_update",
		TitleTemplate:       "付款成功",
		BodyTemplate:        "您的付款已完成",
		DataType:            "PAYMENT_SUCCESS",
		ReferenceIDTemplate: strPtr("#{orderNo}"),
		Enabled:             true,
	}

	cache := buildTestCache([]*domain.NotificationTemplate{tmpl})
	// noTokenRepo → GetActiveByUser returns empty → push status=skipped
	sendPush := usecase.NewSendPushUseCase(
		&noTokenRepo{},
		&capturingPushRepo{},
		&noopPreferenceRepo{},
		&alwaysSuccessNotifier{},
		func(error) bool { return false },
	)
	inboxUC, inboxRepo := newTestInboxUC()

	handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.payment", "https://test-media.example.com", zap.NewNop())

	payload := makePayload("notify_buyer_account_id", float64(77), "orderNo", "ORD-PAID-001")
	body, _ := json.Marshal(payload)

	if err := handler(context.Background(), "payment.success", body); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if inboxRepo.createdItem == nil {
		t.Fatal("expected inbox item to be written even when user has no device tokens")
	}
	if inboxRepo.createdItem.UserID != 77 {
		t.Errorf("inbox UserID = %d, want 77", inboxRepo.createdItem.UserID)
	}
	if inboxRepo.createdItem.ReferenceID != "ORD-PAID-001" {
		t.Errorf("inbox ReferenceID = %q, want ORD-PAID-001", inboxRepo.createdItem.ReferenceID)
	}
	_ = sendPush
}

func TestTemplateHandler_BuyerOrderPaidAndSellerShipmentCreatedRouteIndependently(t *testing.T) {
	// 對齊 2026-05-31 binding 調整：buyer 推播由 hawkr.order:order.paid 觸發、
	// seller「新訂單」推播改由 hawkr.shipping:shipment.created 觸發；
	// 兩者 queue 名稱不同，handler 必須以 (queue, routing_key) 為 key 獨立分派。
	buyerTmpl := &domain.NotificationTemplate{
		ID: 1, Exchange: "hawkr.order", RoutingKey: "order.paid",
		EventType:      "order.paid",
		TargetUserType: "buyer", TargetUserField: "buyer_id",
		TitleTemplate: "買家通知", BodyTemplate: "買家訂單成立",
		DataType: "ORDER_PAID", Enabled: true,
	}
	sellerTmpl := &domain.NotificationTemplate{
		ID: 2, Exchange: "hawkr.shipping", RoutingKey: "shipment.created",
		EventType:      "shipment.created",
		TargetUserType: "seller", TargetUserField: "seller_account_id",
		TitleTemplate: "賣家通知", BodyTemplate: "賣家收到新訂單",
		DataType: "ORDER_CREATED", Enabled: true,
	}

	cache := buildTestCache([]*domain.NotificationTemplate{buyerTmpl, sellerTmpl})

	// buyer queue 應只命中 buyerTmpl
	sendPush, pushRepo := newTestSendPushWithRepo()
	inboxUC, _ := newTestInboxUC()
	buyerHandler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.order", "https://test-media.example.com", zap.NewNop())

	payload := makePayload("buyer_id", float64(10), "seller_account_id", float64(20), "orderNo", "ORD-001")
	body, _ := json.Marshal(payload)

	if err := buyerHandler(context.Background(), "order.paid", body); err != nil {
		t.Fatalf("buyer handler error: %v", err)
	}
	if pushRepo.record == nil {
		t.Fatal("expected buyer push record")
	}
	if pushRepo.record.UserType != "buyer" {
		t.Errorf("expected buyer notification, got user_type=%q", pushRepo.record.UserType)
	}
	if pushRepo.record.EventType != "order.paid" {
		t.Errorf("expected EventType=order.paid, got %q", pushRepo.record.EventType)
	}

	// seller queue 走獨立的 shipment.created routing key
	sellerSendPush, sellerPushRepo := newTestSendPushWithRepo()
	sellerInboxUC, _ := newTestInboxUC()
	sellerHandler := newTemplateHandler(cache, sellerSendPush, sellerInboxUC, "hawkr.shipping", "https://test-media.example.com", zap.NewNop())

	if err := sellerHandler(context.Background(), "shipment.created", body); err != nil {
		t.Fatalf("seller handler error: %v", err)
	}
	if sellerPushRepo.record == nil {
		t.Fatal("expected seller push record")
	}
	if sellerPushRepo.record.UserType != "seller" {
		t.Errorf("expected seller notification, got user_type=%q", sellerPushRepo.record.UserType)
	}
	if sellerPushRepo.record.EventType != "shipment.created" {
		t.Errorf("expected EventType=shipment.created, got %q", sellerPushRepo.record.EventType)
	}
}

func TestTemplateHandler_SellerNewOrderViaShipmentCreated(t *testing.T) {
	tmpl := &domain.NotificationTemplate{
		ID:              1,
		Exchange:        "hawkr.shipping",
		RoutingKey:      "shipment.created",
		EventType:       "shipment.created",
		TargetUserType:  "seller",
		TargetUserField: "seller_account_id",
		Category:        "order_update",
		TitleTemplate:   "買家已下單",
		BodyTemplate:    "🧧 您有新訂單！買家已支付，請儘速準備商品並在 2 天內出貨。",
		DataType:        "ORDER_CREATED",
		DataFields: map[string]domain.FieldSpec{
			"refType":       constSpec("ORDER"),
			"refId":         fieldSpec("order_no"),
			"order_id":      fieldSpec("order_no"),
			"buyerNickname": fieldSpec("buyer_nickname"),
			"buyerAvatar":   fieldSpec("buyer_avatar"),
		},
		SchemaURL:           strPtr("hawkr://orders/{order_id}"),
		ReferenceIDTemplate: strPtr("#{order_no}"),
		Enabled:             true,
	}

	cache := buildTestCache([]*domain.NotificationTemplate{tmpl})
	sendPush, pushRepo := newTestSendPushWithRepo()
	inboxUC, inboxRepo := newTestInboxUC()

	handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.shipping", "https://test-media.example.com", zap.NewNop())

	// 模擬 ShipmentCreatedPayload（含 2026-05-31 新增 5 欄；buyerNickname/buyerAvatar
	// 由 shipping-service BuyerGrpcClient best-effort enrich 取得）
	payload := map[string]interface{}{
		"order_no":            "ORD-SHIP-001",
		"shipment_no":         "SHP-001",
		"ship_trade_no":       "TRK-001",
		"shipping_method":     "3001",
		"status":              "91",
		"label_url":           "",
		"processing_deadline": "2026-06-03T00:00:00Z",
		"seller_account_id":   "1001",
		"buyer_account_id":    "2001",
		"total_actual_amount": "1200",
		"buyer_nickname":      "小明",
		"buyer_avatar":        "https://cdn.example/u/2001.jpg",
	}
	body, _ := json.Marshal(payload)

	if err := handler(context.Background(), "shipment.created", body); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if pushRepo.record == nil {
		t.Fatal("expected seller push record")
	}
	if pushRepo.record.UserID != 1001 {
		t.Errorf("expected UserID=1001 (seller_account_id), got %d", pushRepo.record.UserID)
	}
	if pushRepo.record.UserType != "seller" {
		t.Errorf("expected UserType=seller, got %q", pushRepo.record.UserType)
	}
	if pushRepo.record.EventType != "shipment.created" {
		t.Errorf("expected EventType=shipment.created, got %q", pushRepo.record.EventType)
	}
	if pushRepo.record.ReferenceID != "ORD-SHIP-001" {
		t.Errorf("expected ReferenceID=ORD-SHIP-001, got %q", pushRepo.record.ReferenceID)
	}
	if pushRepo.record.Data["type"] != "ORDER_CREATED" {
		t.Errorf("expected data[type]=ORDER_CREATED, got %q", pushRepo.record.Data["type"])
	}
	if pushRepo.record.Data["refType"] != "ORDER" {
		t.Errorf("expected data[refType]=ORDER, got %q", pushRepo.record.Data["refType"])
	}
	if pushRepo.record.Data["refId"] != "ORD-SHIP-001" {
		t.Errorf("expected data[refId]=ORD-SHIP-001, got %q", pushRepo.record.Data["refId"])
	}
	if pushRepo.record.Data["url"] != "hawkr://orders/ORD-SHIP-001" {
		t.Errorf("expected data[url]=hawkr://orders/ORD-SHIP-001, got %q", pushRepo.record.Data["url"])
	}
	// issue #686：buyerNickname / buyerAvatar 必須帶到 FCM data，供前端 _personalize 替換通用「買家」字
	if pushRepo.record.Data["buyerNickname"] != "小明" {
		t.Errorf("expected data[buyerNickname]=小明, got %q", pushRepo.record.Data["buyerNickname"])
	}
	if pushRepo.record.Data["buyerAvatar"] != "https://cdn.example/u/2001.jpg" {
		t.Errorf("expected data[buyerAvatar]=https://cdn.example/u/2001.jpg, got %q", pushRepo.record.Data["buyerAvatar"])
	}

	if inboxRepo.createdItem == nil {
		t.Fatal("expected seller inbox item to be written")
	}
	if inboxRepo.createdItem.UserType != "seller" || inboxRepo.createdItem.EventType != "shipment.created" {
		t.Errorf("inbox UserType=%q, EventType=%q", inboxRepo.createdItem.UserType, inboxRepo.createdItem.EventType)
	}
}

func TestTemplateHandler_SellerNewOrderEnrichFallback(t *testing.T) {
	// shipping-service BuyerGrpcClient enrich 失敗時，buyerNickname / buyerAvatar
	// 為 null；FieldSpec.Resolve 略過該欄位 → FCM data 不含這兩個 key →
	// 前端 _personalize helper 取不到 → fallback 至通用「買家」字樣。
	// title/body 主文不受影響，refType/refId 等 const/必要欄位仍正常渲染。
	tmpl := &domain.NotificationTemplate{
		ID:              1,
		Exchange:        "hawkr.shipping",
		RoutingKey:      "shipment.created",
		EventType:       "shipment.created",
		TargetUserType:  "seller",
		TargetUserField: "seller_account_id",
		TitleTemplate:   "買家已下單",
		BodyTemplate:    "🧧 您有新訂單！買家已支付，請儘速準備商品並在 2 天內出貨。",
		DataType:        "ORDER_CREATED",
		DataFields: map[string]domain.FieldSpec{
			"refType":       constSpec("ORDER"),
			"refId":         fieldSpec("order_no"),
			"order_id":      fieldSpec("order_no"),
			"buyerNickname": fieldSpec("buyer_nickname"),
			"buyerAvatar":   fieldSpec("buyer_avatar"),
		},
		ReferenceIDTemplate: strPtr("#{order_no}"),
		Enabled:             true,
	}

	cache := buildTestCache([]*domain.NotificationTemplate{tmpl})
	sendPush, pushRepo := newTestSendPushWithRepo()
	inboxUC, _ := newTestInboxUC()
	handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.shipping", "https://test-media.example.com", zap.NewNop())

	// 模擬 enrich 失敗：payload 無 buyer_nickname / buyer_avatar
	payload := map[string]interface{}{
		"order_no":            "ORD-SHIP-002",
		"seller_account_id":   "1001",
		"buyer_account_id":    "2001",
		"total_actual_amount": "888",
	}
	body, _ := json.Marshal(payload)

	if err := handler(context.Background(), "shipment.created", body); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if pushRepo.record == nil {
		t.Fatal("expected push record even when buyer enrich fields missing")
	}
	if pushRepo.record.Title != "買家已下單" || pushRepo.record.Body != "🧧 您有新訂單！買家已支付，請儘速準備商品並在 2 天內出貨。" {
		t.Errorf("title/body should remain static, got title=%q body=%q", pushRepo.record.Title, pushRepo.record.Body)
	}
	if _, ok := pushRepo.record.Data["buyerNickname"]; ok {
		t.Errorf("data[buyerNickname] should be omitted when payload field missing, got %q", pushRepo.record.Data["buyerNickname"])
	}
	if _, ok := pushRepo.record.Data["buyerAvatar"]; ok {
		t.Errorf("data[buyerAvatar] should be omitted when payload field missing, got %q", pushRepo.record.Data["buyerAvatar"])
	}
	if pushRepo.record.Data["refType"] != "ORDER" {
		t.Errorf("expected data[refType]=ORDER, got %q", pushRepo.record.Data["refType"])
	}
}

// constSpec / fieldSpec are test helpers that build domain.FieldSpec literals
// without each call site needing to take the address of a string.
func constSpec(v string) domain.FieldSpec    { return domain.FieldSpec{Const: &v} }
func fieldSpec(name string) domain.FieldSpec { return domain.FieldSpec{Field: &name} }

func TestBuildDataMap_FieldOnly(t *testing.T) {
	dataFields := map[string]domain.FieldSpec{
		"order_id": fieldSpec("orderNo"),
		"refId":    fieldSpec("orderNo"),
	}
	payload := map[string]interface{}{"orderNo": "O992559076"}

	result := BuildDataMap(dataFields, payload)

	if got := result["order_id"]; got != "O992559076" {
		t.Errorf("order_id = %q, want O992559076", got)
	}
	if got := result["refId"]; got != "O992559076" {
		t.Errorf("refId = %q, want O992559076", got)
	}
}

func TestBuildDataMap_ConstAndField(t *testing.T) {
	dataFields := map[string]domain.FieldSpec{
		"refType":  constSpec("ORDER"),
		"refId":    fieldSpec("orderNo"),
		"order_id": fieldSpec("orderNo"),
	}
	payload := map[string]interface{}{"orderNo": "O992559076"}

	result := BuildDataMap(dataFields, payload)

	if got := result["refType"]; got != "ORDER" {
		t.Errorf("refType = %q, want ORDER (literal const)", got)
	}
	if got := result["refId"]; got != "O992559076" {
		t.Errorf("refId = %q, want O992559076", got)
	}
	if got := result["order_id"]; got != "O992559076" {
		t.Errorf("order_id = %q, want O992559076", got)
	}
}

func TestBuildDataMap_MissingPayloadField(t *testing.T) {
	dataFields := map[string]domain.FieldSpec{
		"refType": constSpec("ORDER"),
		"refId":   fieldSpec("orderNo"),
	}
	payload := map[string]interface{}{} // orderNo missing

	result := BuildDataMap(dataFields, payload)

	if got, ok := result["refType"]; !ok || got != "ORDER" {
		t.Errorf("refType should resolve from const, got %q (ok=%v)", got, ok)
	}
	if _, ok := result["refId"]; ok {
		t.Errorf("refId should be omitted when payload field is missing")
	}
}

func TestTemplateHandler_ImageURLRendered(t *testing.T) {
	tmpl := &domain.NotificationTemplate{
		ID:               2,
		Exchange:         "hawkr.order",
		RoutingKey:       "order.paid",
		EventType:        "order.paid",
		TargetUserType:   "buyer",
		TargetUserField:  "buyer_id",
		Category:         "order_update",
		TitleTemplate:    "訂單成立",
		BodyTemplate:     "嗨 #{sellerName}",
		DataType:         "ORDER_PAID",
		DataFields:       map[string]domain.FieldSpec{},
		ImageKeyTemplate: strPtr("#{sellerAvatar}"),
		Enabled:          true,
	}
	cache := buildTestCache([]*domain.NotificationTemplate{tmpl})
	notifier := &capturingNotifier{}
	pushRepo := &capturingPushRepo{}
	prefRepo := &noopPreferenceRepo{}
	inboxRepo := &capturingInboxRepo{}
	// Use alwaysActiveTokenRepo so GetActiveByUser returns a token → notifier.Send is called.
	sendPush := usecase.NewSendPushUseCase(&alwaysActiveTokenRepo{}, pushRepo, prefRepo, notifier, func(error) bool { return false })
	inboxUC := usecase.NewNotificationInboxUseCase(inboxRepo, nil, nil, nil, nil)

	handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.order", "https://test-media.example.com", zap.NewNop())
	body, _ := json.Marshal(map[string]interface{}{
		"buyer_id": float64(1), "orderNo": "O1", "sellerName": "Alice", "sellerAvatar": "https://cdn/a.jpg",
	})
	if err := handler(context.Background(), "order.paid", body); err != nil {
		t.Fatal(err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 send, got %d", len(notifier.calls))
	}
	got := notifier.calls[0]
	if got.imageURL != "https://cdn/a.jpg" {
		t.Errorf("imageURL not rendered: got %q", got.imageURL)
	}
	if got.data["imageUrl"] != "https://cdn/a.jpg" {
		t.Errorf("data.imageUrl not injected: got %q", got.data["imageUrl"])
	}
}

// 49 §5.3 #10：payload 圖片欄位值為 objectKey 時，渲染端以 mediaurl.BuildURL 組完整顯示 URL。
func TestTemplateHandler_ImageKeyComposedToURL(t *testing.T) {
	tmpl := &domain.NotificationTemplate{
		ID: 2, Exchange: "hawkr.order", EventType: "order.paid", RoutingKey: "order.paid",
		TargetUserType: "buyer", TargetUserField: "buyer_id", Category: "order_update",
		TitleTemplate: "訂單成立", BodyTemplate: "嗨", DataType: "ORDER_PAID",
		DataFields: map[string]domain.FieldSpec{}, ImageKeyTemplate: strPtr("#{sellerAvatar}"), Enabled: true,
	}
	cache := buildTestCache([]*domain.NotificationTemplate{tmpl})
	notifier := &capturingNotifier{}
	pushRepo := &capturingPushRepo{}
	sendPush := usecase.NewSendPushUseCase(&alwaysActiveTokenRepo{}, pushRepo, &noopPreferenceRepo{}, notifier, func(error) bool { return false })
	inboxUC := usecase.NewNotificationInboxUseCase(&capturingInboxRepo{}, nil, nil, nil, nil)

	handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.order", "https://test-media.example.com", zap.NewNop())
	body, _ := json.Marshal(map[string]interface{}{
		"buyer_id": float64(1), "orderNo": "O1", "sellerAvatar": "public/avatar/1/a.jpg",
	})
	if err := handler(context.Background(), "order.paid", body); err != nil {
		t.Fatal(err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 send, got %d", len(notifier.calls))
	}
	want := "https://test-media.example.com/i/original/public/avatar/1/a.jpg"
	if notifier.calls[0].imageURL != want {
		t.Errorf("imageURL = %q, want composed %q", notifier.calls[0].imageURL, want)
	}
}

func TestTemplateHandler_ImageURLPlaceholderResidueFallsBack(t *testing.T) {
	tmpl := &domain.NotificationTemplate{
		ID: 3, Exchange: "hawkr.synthetic", EventType: "x.y", RoutingKey: "x.y", TargetUserType: "buyer", TargetUserField: "buyerId",
		Category: "system", TitleTemplate: "t", BodyTemplate: "b", DataType: "X",
		DataFields: map[string]domain.FieldSpec{}, ImageKeyTemplate: strPtr("#{missingField}"), Enabled: true,
	}
	cache := buildTestCache([]*domain.NotificationTemplate{tmpl})
	notifier := &capturingNotifier{}
	pushRepo := &capturingPushRepo{}
	prefRepo := &noopPreferenceRepo{}
	inboxRepo := &capturingInboxRepo{}
	// Use alwaysActiveTokenRepo so GetActiveByUser returns a token → notifier.Send is called.
	sendPush := usecase.NewSendPushUseCase(&alwaysActiveTokenRepo{}, pushRepo, prefRepo, notifier, func(error) bool { return false })
	inboxUC := usecase.NewNotificationInboxUseCase(inboxRepo, nil, nil, nil, nil)

	handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.synthetic", "https://test-media.example.com", zap.NewNop())
	body, _ := json.Marshal(map[string]interface{}{"buyerId": float64(1)})
	_ = handler(context.Background(), "x.y", body)

	if len(notifier.calls) != 1 {
		t.Fatal("expected 1 call")
	}
	if notifier.calls[0].imageURL != "" {
		t.Errorf("expected empty imageURL when placeholder unresolved, got %q", notifier.calls[0].imageURL)
	}
	if _, ok := notifier.calls[0].data["imageUrl"]; ok {
		t.Errorf("expected no data.imageUrl key when unresolved")
	}
}

func TestBuildDataMap_EmptySpecSkipped(t *testing.T) {
	dataFields := map[string]domain.FieldSpec{
		"keep":  fieldSpec("orderNo"),
		"empty": {}, // neither Const nor Field set
	}
	payload := map[string]interface{}{"orderNo": "O1"}

	result := BuildDataMap(dataFields, payload)

	if _, ok := result["empty"]; ok {
		t.Errorf("empty spec should produce no entry, got %q", result["empty"])
	}
	if got := result["keep"]; got != "O1" {
		t.Errorf("keep = %q, want O1", got)
	}
}

func TestMatchesFilter_NilFilter(t *testing.T) {
	if !matchesFilter(nil, map[string]interface{}{"status": "IN_TRANSIT"}) {
		t.Error("nil filter should match anything")
	}
}

func TestMatchesFilter_EmptyFilter(t *testing.T) {
	if !matchesFilter(map[string]domain.FilterCondition{}, map[string]interface{}{"status": "IN_TRANSIT"}) {
		t.Error("empty filter should match anything")
	}
}

func TestMatchesFilter_StringMatch(t *testing.T) {
	f := map[string]domain.FilterCondition{"status": domain.Eq("IN_TRANSIT")}
	p := map[string]interface{}{"status": "IN_TRANSIT", "orderNo": "O1"}
	if !matchesFilter(f, p) {
		t.Error("expected match for status=IN_TRANSIT")
	}
}

func TestMatchesFilter_StringMismatch(t *testing.T) {
	f := map[string]domain.FilterCondition{"status": domain.Eq("IN_TRANSIT")}
	p := map[string]interface{}{"status": "PROCESSING"}
	if matchesFilter(f, p) {
		t.Error("expected no match for status=PROCESSING")
	}
}

func TestMatchesFilter_MissingKey(t *testing.T) {
	f := map[string]domain.FilterCondition{"status": domain.Eq("IN_TRANSIT")}
	p := map[string]interface{}{"orderNo": "O1"}
	if matchesFilter(f, p) {
		t.Error("expected no match when filter key missing from payload")
	}
}

// --- $ne (exclusion) ---

func TestMatchesFilter_NeExcludesMatchingValue(t *testing.T) {
	f := map[string]domain.FilterCondition{"cancel_reason": domain.Ne("9104")}
	p := map[string]interface{}{"cancel_reason": "9104"}
	if matchesFilter(f, p) {
		t.Error("expected no match: cancel_reason equals excluded value 9104")
	}
}

func TestMatchesFilter_NePassesDifferentValue(t *testing.T) {
	f := map[string]domain.FilterCondition{"cancel_reason": domain.Ne("9104")}
	p := map[string]interface{}{"cancel_reason": "9105"}
	if !matchesFilter(f, p) {
		t.Error("expected match: cancel_reason differs from excluded value")
	}
}

func TestMatchesFilter_NeMissingKeyPasses(t *testing.T) {
	// 買家自願取消未付款單：cancel_reason 完全不在 payload 內。
	// $ne 語意「必然不等於排除值」→ 應通過，讓泛用模板（id=7 類）仍然觸發。
	f := map[string]domain.FilterCondition{"cancel_reason": domain.Ne("9104")}
	p := map[string]interface{}{"order_no": "O1"}
	if !matchesFilter(f, p) {
		t.Error("expected match when $ne key absent from payload")
	}
}

func TestMatchesFilter_NeNullValuePasses(t *testing.T) {
	// key 存在但值為 JSON null（Jackson 對 null String 欄位的預設序列化行為）
	// —— DecodePayload 會把它解成 Go nil，toStringVal(nil) == ""，不等於排除值 → 通過。
	f := map[string]domain.FilterCondition{"cancel_reason": domain.Ne("9104")}
	p := map[string]interface{}{"cancel_reason": nil}
	if !matchesFilter(f, p) {
		t.Error("expected match when $ne key present but null")
	}
}

func TestMatchesFilter_EqAndNeCombined(t *testing.T) {
	// 正向等值與排除混用：同一 filter 內兩個 key，一個等值一個 $ne，兩者都要通過。
	f := map[string]domain.FilterCondition{
		"event_type":    domain.Eq("order.cancelled"),
		"cancel_reason": domain.Ne("9104"),
	}
	if !matchesFilter(f, map[string]interface{}{"event_type": "order.cancelled", "cancel_reason": "9105"}) {
		t.Error("expected match: eq satisfied, ne satisfied")
	}
	if matchesFilter(f, map[string]interface{}{"event_type": "order.cancelled", "cancel_reason": "9104"}) {
		t.Error("expected no match: ne violated (cancel_reason == excluded value)")
	}
	if matchesFilter(f, map[string]interface{}{"event_type": "order.shipped", "cancel_reason": "9105"}) {
		t.Error("expected no match: eq violated (event_type differs)")
	}
}

// TestMatchesFilter_CommentLikeChangedAuthorTypeFilter 直接拿
// 20260820120000_comment_like_changed_author_type_filter.sql 寫進 DB 的那串 JSON
// 字面值，驗它解得開且四種 payload 的分流都對（#1626）。
//
// 錨在「migration 的字面值」而不是 domain.Eq/Ne 建構子：真正上線的是那串 JSON，
// 打錯字（`$neq`、多一層物件）會讓 template 載入失敗或 filter 語意改變，而
// 建構子版的測試對此完全免疫。
func TestMatchesFilter_CommentLikeChangedAuthorTypeFilter(t *testing.T) {
	const migrationLiteral = `{"action": "LIKE", "commentAuthorType": {"$ne": "buyer"}}`
	var f map[string]domain.FilterCondition
	if err := json.Unmarshal([]byte(migrationLiteral), &f); err != nil {
		t.Fatalf("migration 的 payload_filter 字面值解不開: %v", err)
	}

	tests := []struct {
		name    string
		payload map[string]interface{}
		want    bool
	}{
		{
			// 賣家作者：commentAuthorId 已是 account_id，賣家收件匣讀得到。
			name:    "seller author LIKE → 推播",
			payload: map[string]interface{}{"action": "LIKE", "commentAuthorType": "seller"},
			want:    true,
		},
		{
			// 買家作者：commentAuthorId 是 buyer_id，寫進 user_type='seller' 的列
			// 永遠沒有人讀得到 —— 這正是本次要擋掉的那一種。
			name:    "buyer author LIKE → 擋掉（否則寫出讀不到的 inbox 列）",
			payload: map[string]interface{}{"action": "LIKE", "commentAuthorType": "buyer"},
			want:    false,
		},
		{
			// 部署窗口：migration 已套用、social-service 還跑舊版（不發這個欄位）。
			// $ne 對缺 key 視為通過 ⇒ 維持現況行為，而不是讓賣家通知整條停掉。
			// 這一條就是「為什麼用 $ne 而不是等值 seller」的機械證明，改成等值即紅。
			name:    "舊 publisher 未發 commentAuthorType → 仍推播（部署順序相容）",
			payload: map[string]interface{}{"action": "LIKE"},
			want:    true,
		},
		{
			name:    "UNLIKE → 擋掉（既有 action filter 不受影響）",
			payload: map[string]interface{}{"action": "UNLIKE", "commentAuthorType": "seller"},
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesFilter(f, tt.payload); got != tt.want {
				t.Errorf("matchesFilter(%v) = %v, want %v", tt.payload, got, tt.want)
			}
		})
	}
}

func TestMatchesFilter_NeNumericPayloadValueCoerced(t *testing.T) {
	// payload 值是 float64（JSON unmarshal 常見型別），$ne 用字串比對，toStringVal 轉換後應相等而排除。
	f := map[string]domain.FilterCondition{"cancel_reason": domain.Ne("9104")}
	p := map[string]interface{}{"cancel_reason": float64(9104)}
	if matchesFilter(f, p) {
		t.Error("expected no match: numeric payload value coerces to excluded string value")
	}
}

func TestFilterCondition_NeValueAcceptsNumberInJSON(t *testing.T) {
	// migration 作者可能誤寫成 {"$ne": 9104}（無引號的數字）而非 {"$ne": "9104"}；
	// UnmarshalJSON 需容錯，兩種寫法解出的 FilterCondition 必須一致。
	var stringForm, numberForm domain.FilterCondition
	if err := json.Unmarshal([]byte(`{"$ne":"9104"}`), &stringForm); err != nil {
		t.Fatalf("unmarshal string form: %v", err)
	}
	if err := json.Unmarshal([]byte(`{"$ne":9104}`), &numberForm); err != nil {
		t.Fatalf("unmarshal number form: %v", err)
	}
	if stringForm != numberForm {
		t.Errorf("expected string form %+v == number form %+v", stringForm, numberForm)
	}
	if stringForm.Op != domain.OpNe || stringForm.Value != "9104" {
		t.Errorf("unexpected decoded condition: %+v", stringForm)
	}
}

func TestFilterCondition_UnmarshalPlainEquality(t *testing.T) {
	var c domain.FilterCondition
	if err := json.Unmarshal([]byte(`"IN_TRANSIT"`), &c); err != nil {
		t.Fatalf("unmarshal plain string: %v", err)
	}
	if c != domain.Eq("IN_TRANSIT") {
		t.Errorf("expected Eq(IN_TRANSIT), got %+v", c)
	}
}

func TestFilterCondition_UnmarshalUnsupportedOperatorErrors(t *testing.T) {
	var c domain.FilterCondition
	if err := json.Unmarshal([]byte(`{"$gt":"1"}`), &c); err == nil {
		t.Error("expected error for unsupported operator $gt")
	}
}

func TestFilterCondition_UnmarshalMultiKeyObjectErrors(t *testing.T) {
	var c domain.FilterCondition
	if err := json.Unmarshal([]byte(`{"$ne":"1","extra":"2"}`), &c); err == nil {
		t.Error("expected error for multi-key filter condition object")
	}
}

func TestFilterCondition_RoundTripMarshal(t *testing.T) {
	eq := domain.Eq("IN_TRANSIT")
	b, err := json.Marshal(eq)
	if err != nil {
		t.Fatalf("marshal eq: %v", err)
	}
	if string(b) != `"IN_TRANSIT"` {
		t.Errorf("expected plain string JSON, got %s", string(b))
	}
	ne := domain.Ne("9104")
	b, err = json.Marshal(ne)
	if err != nil {
		t.Fatalf("marshal ne: %v", err)
	}
	if string(b) != `{"$ne":"9104"}` {
		t.Errorf("expected {\"$ne\":\"9104\"} JSON, got %s", string(b))
	}
}

func TestTemplateHandler_PayloadFilterDispatch(t *testing.T) {
	// Two templates share queue + routing_key but differ in payload_filter.
	// Only the one matching payload.status should fire.
	tmplShipped := &domain.NotificationTemplate{
		ID: 1, Exchange: "hawkr.shipping",
		RoutingKey:      "shipment.status.changed",
		EventType:       "shipment.shipped",
		TargetUserType:  "buyer",
		TargetUserField: "buyer_id",
		Category:        "order_update",
		DataType:        "ORDER_SHIPPED",
		TitleTemplate:   "出貨通知",
		BodyTemplate:    "🚚 您的商品出貨囉！",
		Enabled:         true,
		PushMode:        "visible",
		PayloadFilter:   map[string]domain.FilterCondition{"status": domain.Eq("IN_TRANSIT")},
	}
	tmplArrived := &domain.NotificationTemplate{
		ID: 2, Exchange: "hawkr.shipping",
		RoutingKey:      "shipment.status.changed",
		EventType:       "shipment.arrived",
		TargetUserType:  "buyer",
		TargetUserField: "buyer_id",
		Category:        "order_update",
		DataType:        "ORDER_ARRIVED",
		TitleTemplate:   "到貨通知",
		BodyTemplate:    "📦 包裹到貨囉！",
		Enabled:         true,
		PushMode:        "visible",
		PayloadFilter:   map[string]domain.FilterCondition{"status": domain.Eq("PICKED_UP")},
	}

	cache := buildTestCache([]*domain.NotificationTemplate{tmplShipped, tmplArrived})
	sendPush, pushRepo := newTestSendPushWithRepo()
	inboxUC, _ := newTestInboxUC()

	handler := newTemplateHandler(cache, sendPush, inboxUC,
		"hawkr.shipping", "https://test-media.example.com", zap.NewNop())

	body, _ := json.Marshal(makePayload(
		"buyer_id", float64(999),
		"orderNo", "O100",
		"status", "IN_TRANSIT",
	))

	if err := handler(context.Background(), "shipment.status.changed", body); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// 斷言剛好 1 條（不是 >= 1）— 同時驗證 tmplArrived 被 filter 過濾掉沒 fire。
	if len(pushRepo.records) != 1 {
		t.Fatalf("expected 1 push record (ORDER_SHIPPED only), got %d", len(pushRepo.records))
	}
	if pushRepo.records[0].EventType != "shipment.shipped" {
		t.Errorf("expected ORDER_SHIPPED template fired, got EventType=%q", pushRepo.records[0].EventType)
	}
}

func TestTemplateHandler_OrderCancelledBuyerExcludesSellerSuspended(t *testing.T) {
	tmplGeneric := &domain.NotificationTemplate{
		ID: 7, Exchange: "hawkr.order",
		RoutingKey:      "order.cancelled",
		EventType:       "order.cancelled.buyer",
		TargetUserType:  "buyer",
		TargetUserField: "buyer_id",
		Category:        "order_update",
		DataType:        "ORDER_CANCELLED",
		TitleTemplate:   "訂單已取消",
		BodyTemplate:    "您的訂單 ##{order_no} 已取消，如已付款將原路退回，如有疑問請洽客服。",
		Enabled:         true,
		PushMode:        "visible",
		PayloadFilter:   map[string]domain.FilterCondition{"cancel_reason": domain.Ne("9104")},
	}
	tmplSuspended := &domain.NotificationTemplate{
		ID: 403, Exchange: "hawkr.order",
		RoutingKey:      "order.cancelled",
		EventType:       "order.cancelled.seller_suspended",
		TargetUserType:  "buyer",
		TargetUserField: "buyer_id",
		Category:        "order_update",
		DataType:        "ORDER_CANCELLED",
		TitleTemplate:   "訂單已取消（賣家停權）",
		BodyTemplate:    "訂單 ##{order_no} 因賣家帳號停權已為您取消，若已付款將自動原路退回，無需另行處理。",
		Enabled:         true,
		PushMode:        "visible",
		PayloadFilter:   map[string]domain.FilterCondition{"cancel_reason": domain.Eq("9104")},
	}

	cache := buildTestCache([]*domain.NotificationTemplate{tmplGeneric, tmplSuspended})

	run := func(t *testing.T, cancelReasonPresent bool) *capturingPushRepo {
		t.Helper()
		sendPush, pushRepo := newTestSendPushWithRepo()
		inboxUC, _ := newTestInboxUC()
		handler := newTemplateHandler(cache, sendPush, inboxUC,
			"hawkr.order", "https://test-media.example.com", zap.NewNop())

		pairs := []interface{}{
			"buyer_id", float64(999),
			"notify_buyer_account_id", float64(7488000000000000999),
			"order_no", "O100",
		}
		if cancelReasonPresent {
			pairs = append(pairs, "cancel_reason", "9104")
		}
		body, _ := json.Marshal(makePayload(pairs...))

		if err := handler(context.Background(), "order.cancelled", body); err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		return pushRepo
	}

	t.Run("seller_suspended_cancel_fires_only_403", func(t *testing.T) {
		pushRepo := run(t, true)
		if len(pushRepo.records) != 1 {
			t.Fatalf("expected exactly 1 push (no double notification), got %d", len(pushRepo.records))
		}
		if pushRepo.records[0].EventType != "order.cancelled.seller_suspended" {
			t.Errorf("expected seller_suspended template fired, got EventType=%q", pushRepo.records[0].EventType)
		}
		if pushRepo.records[0].UserID != 999 {
			t.Errorf("expected UserID=999 (buyer_id), got %d — read notify_buyer_account_id instead?", pushRepo.records[0].UserID)
		}
	})

	t.Run("generic_cancel_without_cancel_reason_fires_only_7", func(t *testing.T) {
		pushRepo := run(t, false)
		if len(pushRepo.records) != 1 {
			t.Fatalf("expected exactly 1 push, got %d", len(pushRepo.records))
		}
		if pushRepo.records[0].EventType != "order.cancelled.buyer" {
			t.Errorf("expected generic buyer template fired, got EventType=%q", pushRepo.records[0].EventType)
		}
		if pushRepo.records[0].UserID != 999 {
			t.Errorf("expected UserID=999 (buyer_id), got %d — read notify_buyer_account_id instead?", pushRepo.records[0].UserID)
		}
	})
}

func TestTemplateHandler_ShipmentStatusChangedBuyerFiresOnlyOnInTransit(t *testing.T) {
	tmpl := &domain.NotificationTemplate{
		ID: 3, Exchange: "hawkr.shipping",
		RoutingKey:      "shipment.status.changed",
		EventType:       "shipment.status.changed",
		TargetUserType:  "buyer",
		TargetUserField: "buyer_id",
		Category:        "order_update",
		DataType:        "ORDER_SHIPPED",
		TitleTemplate:   "出貨通知",
		BodyTemplate:    "🚚 您的商品出貨囉！包裹已交給物流，請留意簡訊通知。",
		Enabled:         true,
		PushMode:        "visible",
		PayloadFilter:   map[string]domain.FilterCondition{"status": domain.Eq("31")},
	}
	cache := buildTestCache([]*domain.NotificationTemplate{tmpl})

	run := func(t *testing.T, status string) *capturingPushRepo {
		t.Helper()
		sendPush, pushRepo := newTestSendPushWithRepo()
		inboxUC, _ := newTestInboxUC()
		handler := newTemplateHandler(cache, sendPush, inboxUC,
			"hawkr.shipping", "https://test-media.example.com", zap.NewNop())

		body, _ := json.Marshal(makePayload(
			"buyer_id", float64(999),
			"notify_buyer_account_id", float64(1234567890123456789),
			"order_no", "O100",
			"status", status,
		))
		if err := handler(context.Background(), "shipment.status.changed", body); err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		return pushRepo
	}

	t.Run("in_transit_31_fires", func(t *testing.T) {
		pushRepo := run(t, "31")
		if len(pushRepo.records) != 1 {
			t.Fatalf("expected exactly 1 push for IN_TRANSIT (\"31\"), got %d", len(pushRepo.records))
		}
		if pushRepo.records[0].EventType != "shipment.status.changed" {
			t.Errorf("expected shipment.status.changed template fired, got EventType=%q", pushRepo.records[0].EventType)
		}
		if pushRepo.records[0].UserID != 999 {
			t.Errorf("expected UserID=999 (buyer_id), got %d — read notify_buyer_account_id instead?", pushRepo.records[0].UserID)
		}
	})

	t.Run("pending_shipment_21_does_not_fire", func(t *testing.T) {
		pushRepo := run(t, "21")
		if len(pushRepo.records) != 0 {
			t.Fatalf("expected 0 pushes for non-IN_TRANSIT status (\"21\"), got %d", len(pushRepo.records))
		}
	})

	t.Run("legacy_enum_name_no_longer_matches", func(t *testing.T) {
		// 防止有人「修好」migration 又手滑改回列舉名字串——這個值從未真正出現在
		// wire payload 上，filter 不該對它放行。
		pushRepo := run(t, "IN_TRANSIT")
		if len(pushRepo.records) != 0 {
			t.Fatalf("expected 0 pushes for the old broken filter value (\"IN_TRANSIT\", never emitted on the real wire), got %d", len(pushRepo.records))
		}
	})
}

func TestMatchesFilter_NumericPayloadValueCoerced(t *testing.T) {
	// payload 值是 float64（JSON unmarshal 預設）但 filter 是字串 "31" — 應 match
	f := map[string]domain.FilterCondition{"statusCode": domain.Eq("31")}
	p := map[string]interface{}{"statusCode": float64(31)}
	if !matchesFilter(f, p) {
		t.Error("expected numeric payload to coerce to string and match")
	}
}

func TestMatchesFilter_BoolPayloadValueCoerced(t *testing.T) {
	// payload bool 透過 toStringVal 轉 "true"/"false"，filter 用字串比對
	f := map[string]domain.FilterCondition{"silent": domain.Eq("true")}
	p := map[string]interface{}{"silent": true}
	if !matchesFilter(f, p) {
		t.Error("expected bool payload to coerce to string and match")
	}
}

// --- chat.message.sent (B-178) ---

// chatTemplates mirrors the two notification_templates rows seeded by
// migration 20260525000001 (buyer/seller receivers, split by receiverType).
func chatTemplates() []*domain.NotificationTemplate {
	mk := func(id int64, userType, receiverType string) *domain.NotificationTemplate {
		return &domain.NotificationTemplate{
			ID:              id,
			Exchange:        "hawkr.im",
			RoutingKey:      "chat.message.sent",
			EventType:       "chat.message.sent",
			TargetUserType:  userType,
			TargetUserField: "receiverId",
			Category:        "chat",
			TitleTemplate:   "#{senderName}",
			BodyTemplate:    "#{contentPreview}",
			BodyFallback:    strPtr("您有一則新訊息"),
			DataType:        "chat_message",
			DataFields: map[string]domain.FieldSpec{
				"refType":        constSpec("CHAT"),
				"refId":          fieldSpec("conversationId"),
				"senderId":       fieldSpec("senderId"),
				"senderName":     fieldSpec("senderName"),
				"senderAvatar":   fieldSpec("senderAvatar"),
				"conversationId": fieldSpec("conversationId"),
				"messageId":      fieldSpec("messageId"),
				"sentAt":         fieldSpec("sentAt"),
			},
			ReferenceIDTemplate: strPtr("#{messageId}"),
			PayloadFilter:       map[string]domain.FilterCondition{"receiverType": domain.Eq(receiverType)},
			PushMode:            "visible",
			Enabled:             true,
		}
	}
	return []*domain.NotificationTemplate{mk(1, "buyer", "buyer"), mk(2, "seller", "seller")}
}

// Both chat templates share the same queue + routing key; payload_filter on
// receiverType must route each message to exactly one (the receiver's role),
// and push_mode=visible must produce a notification+data Send (not SendDataOnly).
func TestTemplateHandler_ChatMessage_VisiblePushRoutedByReceiverType(t *testing.T) {
	cases := []struct {
		name         string
		receiverType string
	}{
		{"receiver is buyer", "buyer"},
		{"receiver is seller", "seller"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cache := buildTestCache(chatTemplates())
			pushRepo := &capturingPushRepo{}
			notifier := &capturingNotifier{}
			sendPush := usecase.NewSendPushUseCase(&alwaysActiveTokenRepo{}, pushRepo, &noopPreferenceRepo{}, notifier, func(error) bool { return false })
			inboxUC, inboxRepo := newTestInboxUC()

			payload := makePayload(
				"messageId", "555",
				"conversationId", "b42_s100",
				"senderId", "42",
				"senderName", "Alice",
				"senderAvatar", "https://cdn/alice.jpg",
				"receiverId", "100",
				"receiverType", tc.receiverType,
				"contentPreview", "你好",
				"sentAt", "2026-05-25T10:00:00Z",
			)
			body, _ := json.Marshal(payload)

			handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.im", "https://test-media.example.com", zap.NewNop())
			if err := handler(context.Background(), "chat.message.sent", body); err != nil {
				t.Fatalf("handler returned error: %v", err)
			}

			// Exactly one visible Send — proves the filter selected a single template
			// (not both) and push_mode=visible used Send rather than SendDataOnly.
			if len(notifier.calls) != 1 {
				t.Fatalf("expected exactly 1 visible Send call, got %d", len(notifier.calls))
			}
			call := notifier.calls[0]
			if call.title != "Alice" {
				t.Errorf("title = %q, want Alice (senderName)", call.title)
			}
			if call.body != "你好" {
				t.Errorf("body = %q, want 你好 (contentPreview)", call.body)
			}
			if call.data["type"] != "chat_message" {
				t.Errorf("data[type] = %q, want chat_message", call.data["type"])
			}
			// Explicit refType/refId contract → gateway inbox uses the primary nav path.
			if call.data["refType"] != "CHAT" || call.data["refId"] != "b42_s100" {
				t.Errorf("data refType/refId = %q/%q, want CHAT/b42_s100", call.data["refType"], call.data["refId"])
			}
			if call.data["conversationId"] != "b42_s100" || call.data["messageId"] != "555" {
				t.Errorf("data conversationId/messageId = %q/%q, want b42_s100/555", call.data["conversationId"], call.data["messageId"])
			}

			// Pushed to the receiver (id 100) as the role matching receiverType,
			// idempotency keyed on messageId; visible push also writes inbox.
			if pushRepo.record == nil || pushRepo.record.UserID != 100 {
				t.Fatalf("expected push record for user 100, got %+v", pushRepo.record)
			}
			if pushRepo.record.UserType != tc.receiverType {
				t.Errorf("push UserType = %q, want %q", pushRepo.record.UserType, tc.receiverType)
			}
			if pushRepo.record.ReferenceID != "555" {
				t.Errorf("ReferenceID = %q, want 555 (messageId)", pushRepo.record.ReferenceID)
			}
			if inboxRepo.createdItem == nil || inboxRepo.createdItem.ReferenceID != "555" {
				t.Errorf("expected inbox item with ReferenceID 555, got %+v", inboxRepo.createdItem)
			}
		})
	}
}

func TestRenderTemplate_DefaultUsedWhenMissing(t *testing.T) {
	payload := makePayload("orderNo", "ORD-001")
	got, missing := RenderTemplate("買家 #{buyerNickname|買家} 已下單", payload)
	if got != "買家 買家 已下單" {
		t.Errorf("got %q", got)
	}
	if len(missing) != 0 {
		t.Errorf("placeholder with default must not be reported missing, got %v", missing)
	}
}

func TestRenderTemplate_DefaultUsedWhenNil(t *testing.T) {
	payload := makePayload("buyerNickname", nil)
	got, missing := RenderTemplate("#{buyerNickname|買家} 已下單", payload)
	if got != "買家 已下單" {
		t.Errorf("got %q", got)
	}
	if len(missing) != 0 {
		t.Errorf("expected no missing, got %v", missing)
	}
}

func TestRenderTemplate_DefaultUsedWhenEmptyString(t *testing.T) {
	payload := makePayload("buyerNickname", "")
	got, _ := RenderTemplate("#{buyerNickname|買家} 已下單", payload)
	if got != "買家 已下單" {
		t.Errorf("got %q", got)
	}
}

func TestRenderTemplate_DefaultIgnoredWhenValuePresent(t *testing.T) {
	payload := makePayload("buyerNickname", "王小明")
	got, _ := RenderTemplate("#{buyerNickname|買家} 已下單", payload)
	if got != "王小明 已下單" {
		t.Errorf("got %q", got)
	}
}

func TestRenderTemplate_DefaultContainingPipe(t *testing.T) {
	payload := map[string]interface{}{}
	got, _ := RenderTemplate("#{x|a|b}", payload)
	if got != "a|b" {
		t.Errorf("got %q", got)
	}
}

func TestRenderTemplate_MissingWithoutDefaultStillReported(t *testing.T) {
	payload := makePayload("orderNo", "ORD-001")
	got, missing := RenderTemplate("訂單 #{orderNo}，賣家 #{sellerName}", payload)
	if got != "訂單 ORD-001，賣家 #{sellerName}" {
		t.Errorf("got %q", got)
	}
	if len(missing) != 1 || missing[0] != "sellerName" {
		t.Errorf("expected missing=[sellerName], got %v", missing)
	}
}

func TestRenderTemplate_EmptyDefaultRendersEmpty(t *testing.T) {
	payload := map[string]interface{}{}
	got, missing := RenderTemplate("a#{x|}b", payload)
	if got != "ab" {
		t.Errorf("got %q", got)
	}
	if len(missing) != 0 {
		t.Errorf("expected no missing, got %v", missing)
	}
}

// commentLikeChangedSellerTemplate 是 20260608100000_rebind_comment_liked_to_like_changed.sql
// ＋ 20260820120000_comment_like_changed_author_type_filter.sql ＋ issue #2707 Phase B
// batch3 retarget migration 之後在 DB 裡那一列的形狀（target_user_type='seller'、
// target_user_field='commentAuthorStoreId'、identity_migrated=true、payload_filter
// 取自 migration 字面值）。self-skip 的鑑別力必須在**這個** shape 上驗，因為它才是
// 線上真正跑的那一張。
func commentLikeChangedSellerTemplate(t *testing.T) *domain.NotificationTemplate {
	t.Helper()
	const migrationLiteral = `{"action": "LIKE", "commentAuthorType": {"$ne": "buyer"}}`
	var filter map[string]domain.FilterCondition
	if err := json.Unmarshal([]byte(migrationLiteral), &filter); err != nil {
		t.Fatalf("migration 的 payload_filter 字面值解不開: %v", err)
	}
	return &domain.NotificationTemplate{
		ID:                  851819712504397824,
		Exchange:            "hawkr.content",
		RoutingKey:          "comment.like_changed",
		EventType:           "comment.like_changed",
		TargetUserType:      "seller",
		TargetUserField:     "commentAuthorStoreId",
		IdentityMigrated:    true,
		Category:            "social",
		TitleTemplate:       "❤️ 買家喜歡你的回覆！",
		BodyTemplate:        "#{likerName} 覺得你的解答很有幫助。",
		DataType:            "COMMENT_LIKED",
		PayloadFilter:       filter,
		ReferenceIDTemplate: strPtr("#{commentId}"),
		Enabled:             true,
	}
}

// TestTemplateHandler_CommentLikeSelfSkip_SellerLikesOwnComment 原是 #1626 修復引入的
// 退化回歸測試（那次 commentAuthorId 對賣家作者解析成 account_id，但 self-skip 仍拿
// userId 這個 store_id surrogate 去比，兩值不同空間永遠比不中）。issue #2707 Phase B
// batch3 把 target_user_field 從 commentAuthorId（account_id）改成 commentAuthorStoreId
// （raw store_id）之後，比對邏輯整個鏡射：現在换成 actor 也必須用未解析的 userId
// （而非 likerPrincipalId）才能落在同一個 store_id 空間，這條測試同步改成守住新形狀。
//
// 錨在「沒有寫 inbox row、沒有產生 push record」——不是「有沒有印 log」。
// 同時附一個 positive control（別人按讚時必須照常寫），否則「什麼都沒發生」
// 也會讓 skip 那半邊變綠。
func TestTemplateHandler_CommentLikeSelfSkip_SellerLikesOwnComment(t *testing.T) {
	// 賣家 store_id 8888 / account_id 3001。social-service payload 兩組欄位並存：
	// commentAuthorId/likerPrincipalId 是已解析的 account_id（issue #2707 之前的
	// target/actor 配對，現在兩個都不再被讀），commentAuthorStoreId/userId 才是現在
	// target/actor 實際比對用的原始 store_id 欄位。
	const authorAccountID = 3001
	const authorStoreID = 8888
	const otherLikerStoreID = 9999

	cache := buildTestCache([]*domain.NotificationTemplate{commentLikeChangedSellerTemplate(t)})

	t.Run("賣家按自己的留言 → 不寫 inbox、不推播", func(t *testing.T) {
		sendPush, pushRepo := newTestSendPushWithRepo()
		inboxUC, inboxRepo := newTestInboxUC()
		handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.content", "https://test-media.example.com", zap.NewNop())

		body, _ := json.Marshal(makePayload(
			"action", "LIKE",
			"commentAuthorType", "seller",
			"commentAuthorId", float64(authorAccountID), // 已不再被讀，僅還原真實 payload 形狀
			"commentAuthorStoreId", float64(authorStoreID),
			"likerPrincipalId", float64(authorAccountID), // 同上，已不再被讀
			"userId", float64(authorStoreID), // 同一個人，跟 commentAuthorStoreId 同值
			"commentId", "777",
			"likerName", "自己",
		))
		if err := handler(context.Background(), "comment.like_changed", body); err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if inboxRepo.createdItem != nil {
			t.Errorf("self-notification 仍寫了 inbox row（user_id=%d）—— actor 欄位（userId）與 target 欄位"+
				"（commentAuthorStoreId）必須落在同一個 store_id 身分空間",
				inboxRepo.createdItem.UserID)
		}
		if pushRepo.record != nil {
			t.Errorf("self-notification 仍推了 FCM（user_id=%d）", pushRepo.record.UserID)
		}
	})

	// Positive control：換一個按讚者，同一張 template、同一份 payload 形狀，
	// 必須照常寫 inbox。沒有這一條，上面的斷言在 handler 整條壞掉時也會綠。
	t.Run("別人按賣家的留言 → 照常寫 inbox", func(t *testing.T) {
		sendPush, pushRepo := newTestSendPushWithRepo()
		inboxUC, inboxRepo := newTestInboxUC()
		handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.content", "https://test-media.example.com", zap.NewNop())

		body, _ := json.Marshal(makePayload(
			"action", "LIKE",
			"commentAuthorType", "seller",
			"commentAuthorId", float64(authorAccountID),
			"commentAuthorStoreId", float64(authorStoreID),
			"likerPrincipalId", float64(otherLikerStoreID),
			"userId", float64(otherLikerStoreID),
			"commentId", "777",
			"likerName", "別人",
		))
		if err := handler(context.Background(), "comment.like_changed", body); err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if inboxRepo.createdItem == nil {
			t.Fatal("別人按讚應該要寫 inbox row")
		}
		if inboxRepo.createdItem.UserID != authorStoreID {
			t.Errorf("inbox UserID = %d, want %d", inboxRepo.createdItem.UserID, authorStoreID)
		}
		if pushRepo.record == nil {
			t.Error("別人按讚應該要產生 push record")
		}
	})
}

// TestTemplateHandler_CommentLikeSelfSkip_BuyerLikesOwnComment 守住「原本就正確
// 的那一半」：買家作者的 commentAuthorId 是 buyer_id、likerPrincipalId 也是
// buyer_id，比對照樣成立。若哪天把 likerPrincipalId 改成「一律解析成 account_id」，
// 這一條會紅。
//
// ⚠️ 誠實標註：線上今天沒有買家版 template（唯一那張是 target_user_type='seller'
// ＋ payload_filter commentAuthorType $ne "buyer"，買家作者的事件在 filter 就被擋掉，
// 見該 migration 的 ⚠️ 說明）。所以這裡用的是「買家版 template 落地後應有的形狀」
// ——本測試守的是 self-skip 機制在買家身分空間仍然成立，不是一條現行的線上路徑。
func TestTemplateHandler_CommentLikeSelfSkip_BuyerLikesOwnComment(t *testing.T) {
	const authorBuyerID = 7777

	tmpl := commentLikeChangedSellerTemplate(t)
	tmpl.TargetUserType = "buyer"
	// 買家版 target_user_field 不受 issue #2707 Phase B batch3 影響（那次只改了
	// seller 側），維持原本解析後的 commentAuthorId（對買家而言就是 buyer_id，
	// 不需要額外解析）——覆寫掉 commentLikeChangedSellerTemplate 賣家版的
	// commentAuthorStoreId/identity_migrated，避免誤繼承。
	tmpl.TargetUserField = "commentAuthorId"
	tmpl.IdentityMigrated = false
	// 買家版 template 的 filter 會是等值 buyer（賣家版用 $ne buyer 把它排掉）。
	tmpl.PayloadFilter = map[string]domain.FilterCondition{
		"action":            domain.Eq("LIKE"),
		"commentAuthorType": domain.Eq("buyer"),
	}
	cache := buildTestCache([]*domain.NotificationTemplate{tmpl})

	sendPush, pushRepo := newTestSendPushWithRepo()
	inboxUC, inboxRepo := newTestInboxUC()
	handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.content", "https://test-media.example.com", zap.NewNop())

	body, _ := json.Marshal(makePayload(
		"action", "LIKE",
		"commentAuthorType", "buyer",
		"commentAuthorId", float64(authorBuyerID),
		"likerPrincipalId", float64(authorBuyerID), // 買家不解析，兩值同為 buyer_id
		"userId", float64(authorBuyerID),
		"commentId", "778",
		"likerName", "自己",
	))
	if err := handler(context.Background(), "comment.like_changed", body); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if inboxRepo.createdItem != nil {
		t.Errorf("買家自我通知仍寫了 inbox row（user_id=%d）—— 買家兩值同為 buyer_id，本來就該 skip",
			inboxRepo.createdItem.UserID)
	}
	if pushRepo.record != nil {
		t.Errorf("買家自我通知仍推了 FCM（user_id=%d）", pushRepo.record.UserID)
	}
}

// TestActorFieldByEventType_CommentLikeUsesResolvedLikerField 直接把「actor 欄位
// 必須跟 target_user_field 落在同一個身分空間」釘成斷言。上面兩支測的是行為，這一支
// 測的是**登記**：有人動了這個 map、卻沒意識到 target 已經在 issue #2707 Phase B
// batch3 改成 store_id 空間時，行為測試會紅，但這一支會直接指出原因。
func TestActorFieldByEventType_CommentLikeUsesResolvedLikerField(t *testing.T) {
	got := actorFieldByEventType["comment.like_changed"]
	if got != "userId" {
		t.Errorf("actorFieldByEventType[\"comment.like_changed\"] = %q, want \"userId\"；"+
			"target_user_field(commentAuthorStoreId) 對賣家作者是原始未解析的 store_id（issue #2707 Phase B batch3 起），"+
			"actor 必須用同樣未解析的 userId 才能落在同一個身分空間比對——用 likerPrincipalId（已解析成 account_id）"+
			"會讓比對永遠不成立", got)
	}
}

// ==================== comment.created / comment.replied self-skip ====================
//
// 這一組守兩件事，兩件都會靜默失效（沒有任何錯誤訊號）：
//   ① actorFieldByEventType 的 key 必須是 **event_type**。comment.replied 是 routing
//      key，掛在它上面的兩張 template 的 event_type 是 comment.replied.seller /
//      comment.replied.buyer（baseline id 24/25）—— 登記成 "comment.replied" 的話，
//      map 查不到、self-skip 從未被執行過。
//   ② actor 欄位必須與 target_user_field 落在同一身分空間。commenterId / replierId 是
//      留言者的 active-role surrogate（賣家＝store_id），而 postAuthorId / replyToUserId
//      對賣家已解析成 account_id ⇒ 拿未解析的去比永遠比不中。

// commentCreatedSellerTemplate 是 20260703130000_template_ssot_baseline.sql id=20
// ＋ 20260729130000_comment_dedup_payload_filter.sql 在 DB 裡那一列的形狀。
// 這裡 routing_key 與 event_type 同名純屬巧合，別據此推論 replied 那組。
func commentCreatedSellerTemplate() *domain.NotificationTemplate {
	return &domain.NotificationTemplate{
		ID:               20,
		Exchange:         "hawkr.content",
		RoutingKey:       "comment.created",
		EventType:        "comment.created",
		TargetUserType:   "seller",
		TargetUserField:  "postAuthorStoreId",
		IdentityMigrated: true,
		Category:         "content",
		TitleTemplate:    "收到新留言",
		BodyTemplate:     "#{commenterName}：「#{contentPreview}」",
		DataType:         "COMMENT_CREATED",
		PayloadFilter:    map[string]domain.FilterCondition{"postAuthorType": domain.Eq("seller")},
		Enabled:          true,
	}
}

// commentRepliedSellerTemplate 是 baseline id=24。**routing_key 是 comment.replied
// 而 event_type 是 comment.replied.seller** —— 這個落差正是 ① 要守的東西。
func commentRepliedSellerTemplate() *domain.NotificationTemplate {
	return &domain.NotificationTemplate{
		ID:                  24,
		Exchange:            "hawkr.content",
		RoutingKey:          "comment.replied",
		EventType:           "comment.replied.seller",
		TargetUserType:      "seller",
		TargetUserField:     "replyToUserStoreId",
		IdentityMigrated:    true,
		Category:            "content",
		TitleTemplate:       "貼文被回覆",
		BodyTemplate:        "#{replierName} 回覆了您的留言：「#{contentPreview}」",
		DataType:            "COMMENT_REPLIED",
		PayloadFilter:       map[string]domain.FilterCondition{"replyToUserType": domain.Eq("seller")},
		ReferenceIDTemplate: strPtr("reply_#{postId}_#{commentId}"),
		Enabled:             true,
	}
}

// 錨在「沒有寫 inbox row、沒有產生 push record」——不是「有沒有印 log」。
// 每條都附一個 positive control（換一個 actor，同 template 同 payload 形狀必須照常寫），
// 否則 handler 整條壞掉時 skip 那半邊也會綠。
func TestTemplateHandler_CommentCreatedSelfSkip_SellerCommentsOwnPost(t *testing.T) {
	// 賣家 store_id 8888 / account_id 3001。social-service payload 兩組欄位並存：
	// postAuthorId/commenterPrincipalId 是已解析的 account_id（issue #2707 之前的
	// target/actor 配對，現在兩個都不再被讀），postAuthorStoreId/commenterId 才是
	// 現在 target/actor 實際比對用的原始 store_id 欄位。
	const authorAccountID = 3001
	const authorStoreID = 8888
	const otherBuyerID = 4002

	cache := buildTestCache([]*domain.NotificationTemplate{commentCreatedSellerTemplate()})

	t.Run("賣家在自己貼文下留言 → 不寫 inbox、不推播", func(t *testing.T) {
		sendPush, pushRepo := newTestSendPushWithRepo()
		inboxUC, inboxRepo := newTestInboxUC()
		handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.content", "https://test-media.example.com", zap.NewNop())

		body, _ := json.Marshal(makePayload(
			"postAuthorType", "seller",
			"postAuthorId", float64(authorAccountID), // 已不再被讀，僅還原真實 payload 形狀
			"postAuthorStoreId", float64(authorStoreID),
			"commenterPrincipalId", float64(authorAccountID), // 同上，已不再被讀
			"commenterId", float64(authorStoreID), // 同一個人，跟 postAuthorStoreId 同值
			"postId", "10",
			"commentId", "777",
			"commenterName", "自己",
			"contentPreview", "自言自語",
		))
		if err := handler(context.Background(), "comment.created", body); err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if inboxRepo.createdItem != nil {
			t.Errorf("self-notification 仍寫了 inbox row（user_id=%d）—— actor 欄位（commenterId）與 target 欄位"+
				"（postAuthorStoreId）必須落在同一個 store_id 身分空間",
				inboxRepo.createdItem.UserID)
		}
		if pushRepo.record != nil {
			t.Errorf("self-notification 仍推了 FCM（user_id=%d）", pushRepo.record.UserID)
		}
	})

	t.Run("買家在賣家貼文下留言 → 照常寫 inbox", func(t *testing.T) {
		sendPush, pushRepo := newTestSendPushWithRepo()
		inboxUC, inboxRepo := newTestInboxUC()
		handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.content", "https://test-media.example.com", zap.NewNop())

		body, _ := json.Marshal(makePayload(
			"postAuthorType", "seller",
			"postAuthorId", float64(authorAccountID),
			"postAuthorStoreId", float64(authorStoreID),
			"commenterPrincipalId", float64(otherBuyerID),
			"commenterId", float64(otherBuyerID),
			"postId", "10",
			"commentId", "777",
			"commenterName", "買家",
			"contentPreview", "請問有貨嗎",
		))
		if err := handler(context.Background(), "comment.created", body); err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if inboxRepo.createdItem == nil {
			t.Fatal("別人留言應該要寫 inbox row")
		}
		if inboxRepo.createdItem.UserID != authorStoreID {
			t.Errorf("inbox UserID = %d, want %d", inboxRepo.createdItem.UserID, authorStoreID)
		}
		if pushRepo.record == nil {
			t.Error("別人留言應該要產生 push record")
		}
	})
}

// TestTemplateHandler_CommentRepliedSelfSkip_SellerRepliesOwnComment 同時是 ① 的行為
// 證明：payload 走的是 routing key "comment.replied"，而命中的 template 的 event_type
// 是 "comment.replied.seller"。若 map 仍登記在 "comment.replied"，skip 那半邊會紅。
func TestTemplateHandler_CommentRepliedSelfSkip_SellerRepliesOwnComment(t *testing.T) {
	// 賣家 store_id 8888 / account_id 3001。replyToUserId/replierPrincipalId 是已
	// 解析的 account_id（不再被讀），replyToUserStoreId/replierId 才是現在 target/actor
	// 實際比對用的原始 store_id 欄位。回覆是賣家管理動作，replier 恆為賣家本人。
	const targetAccountID = 3001
	const targetStoreID = 8888
	const otherReplierStoreID = 9999

	cache := buildTestCache([]*domain.NotificationTemplate{commentRepliedSellerTemplate()})

	t.Run("賣家回覆自己的留言 → 不寫 inbox、不推播", func(t *testing.T) {
		sendPush, pushRepo := newTestSendPushWithRepo()
		inboxUC, inboxRepo := newTestInboxUC()
		handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.content", "https://test-media.example.com", zap.NewNop())

		body, _ := json.Marshal(makePayload(
			"replyToUserType", "seller",
			"replyToUserId", float64(targetAccountID), // 已不再被讀，僅還原真實 payload 形狀
			"replyToUserStoreId", float64(targetStoreID),
			"replierPrincipalId", float64(targetAccountID), // 同上，已不再被讀
			"replierId", float64(targetStoreID), // 同一個人，跟 replyToUserStoreId 同值
			"postId", "10",
			"commentId", "777",
			"replierName", "自己",
			"contentPreview", "補充一下",
		))
		if err := handler(context.Background(), "comment.replied", body); err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if inboxRepo.createdItem != nil {
			t.Errorf("self-notification 仍寫了 inbox row（user_id=%d）—— actorFieldByEventType 的 key 必須是 event_type "+
				"(comment.replied.seller)，登記成 routing key (comment.replied) 會讓整條 self-skip 從未執行；"+
				"或 actor 欄位（replierId）與 target 欄位（replyToUserStoreId）沒有落在同一個 store_id 身分空間",
				inboxRepo.createdItem.UserID)
		}
		if pushRepo.record != nil {
			t.Errorf("self-notification 仍推了 FCM（user_id=%d）", pushRepo.record.UserID)
		}
	})

	t.Run("回覆別人的留言 → 照常寫 inbox", func(t *testing.T) {
		sendPush, pushRepo := newTestSendPushWithRepo()
		inboxUC, inboxRepo := newTestInboxUC()
		handler := newTemplateHandler(cache, sendPush, inboxUC, "hawkr.content", "https://test-media.example.com", zap.NewNop())

		body, _ := json.Marshal(makePayload(
			"replyToUserType", "seller",
			"replyToUserId", float64(targetAccountID),
			"replyToUserStoreId", float64(targetStoreID),
			"replierPrincipalId", float64(otherReplierStoreID),
			"replierId", float64(otherReplierStoreID),
			"postId", "10",
			"commentId", "777",
			"replierName", "別人",
			"contentPreview", "我也想知道",
		))
		if err := handler(context.Background(), "comment.replied", body); err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if inboxRepo.createdItem == nil {
			t.Fatal("別人回覆應該要寫 inbox row")
		}
		if inboxRepo.createdItem.UserID != targetStoreID {
			t.Errorf("inbox UserID = %d, want %d", inboxRepo.createdItem.UserID, targetStoreID)
		}
		if pushRepo.record == nil {
			t.Error("別人回覆應該要產生 push record")
		}
	})
}

// TestActorFieldByEventType_CommentEventsUseResolvedPrincipalFields 直接把登記本身
// 釘成斷言。上面幾支測的是行為，這一支測的是**登記**：有人動了這張 map、或把 key 寫回
// routing key 時，這一支會直接指出原因。
//
// issue #2707 Phase B batch3（2026-08-30 之後）起，seller 側三支的 target_user_field
// 已改成未解析的原始 store_id 欄位，actor 因此也跟著改用同樣未解析的 surrogate 欄位；
// buyer 側兩支不受影響，維持原本已解析的 *PrincipalId（見本檔上方 buyer self-skip
// 測試與 template_handler.go 的 actorFieldByEventType 註解）。
func TestActorFieldByEventType_CommentEventsUseResolvedPrincipalFields(t *testing.T) {
	want := map[string]string{
		"comment.created":        "commenterId",
		"comment.created.buyer":  "commenterPrincipalId",
		"comment.replied.seller": "replierId",
		"comment.replied.buyer":  "replierPrincipalId",
		"comment.like_changed":   "userId",
	}
	for eventType, wantField := range want {
		if got := actorFieldByEventType[eventType]; got != wantField {
			t.Errorf("actorFieldByEventType[%q] = %q, want %q；actor 欄位必須跟該 event_type 的 target_user_field "+
				"落在同一個身分空間（seller 三支自 issue #2707 Phase B batch3 起是未解析的 store_id surrogate，"+
				"buyer 兩支維持已解析的 buyer_id），空間不對會讓 self-skip 永遠比不中",
				eventType, got, wantField)
		}
	}
	// routing key 不得出現在 map（key 是 event_type）。留著它只會讓下一個人以為
	// self-skip 有在跑。
	if got, ok := actorFieldByEventType["comment.replied"]; ok {
		t.Errorf("actorFieldByEventType 仍登記了 routing key \"comment.replied\" = %q；"+
			"template 的 event_type 是 comment.replied.seller／.buyer，這個 key 在 DB 裡零命中", got)
	}
}
