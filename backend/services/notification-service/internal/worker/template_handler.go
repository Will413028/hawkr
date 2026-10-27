package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
	"github.com/Will413028/hawkr/notification-service/internal/usecase/dto"
)

// newTemplateHandler returns a mq.HandlerFunc that dispatches each message to
// the matching DB-driven template(s) for the given source exchange.
func newTemplateHandler(
	cache *TemplateCache,
	sendPush *usecase.SendPushUseCase,
	inboxUC *usecase.NotificationInboxUseCase,
	exchange string,
	mediaBase string,
	logger *zap.Logger,
) mq.HandlerFunc {
	return func(ctx context.Context, routingKey string, body []byte) error {
		templates := cache.FindByExchangeAndRoutingKey(exchange, routingKey)
		if len(templates) == 0 {
			// Expected, not exceptional: queues are bound with "#", so every event
			// on the exchange arrives here and most have no notification template.
			// Dropped before payload decode, which is what keeps wildcard binding
			// cheap. Debug level — at Warn this would drown the logs.
			logger.Debug("[template] no template for routing key, skipping",
				zap.String("exchange", exchange),
				zap.String("routing_key", routingKey))
			return nil
		}

		payload, err := DecodePayload(body)
		if err != nil {
			logger.Error("[template] failed to unmarshal payload",
				zap.Error(err),
				zap.String("exchange", exchange),
				zap.String("routing_key", routingKey))
			return fmt.Errorf("unmarshal notification template payload: %w", err)
		}

		matched := 0
		for _, tmpl := range templates {
			if !matchesFilter(tmpl.PayloadFilter, payload) {
				continue
			}
			matched++
			handleTemplate(ctx, tmpl, payload, sendPush, inboxUC, mediaBase, logger)
		}
		if matched == 0 {
			logger.Debug("[template] all templates filtered out by payload_filter",
				zap.String("exchange", exchange),
				zap.String("routing_key", routingKey),
				zap.Int("candidate_templates", len(templates)))
		}
		return nil
	}
}

// handleTemplate renders the template for a single user and sends the push notification.
//
// Branches by tmpl.PushMode:
//   - "visible" (default): writes inbox + sends FCM with notification block (banner shown)
//   - "silent": skips inbox + sends FCM data-only (no banner, App handles in background
//     for refresh-only scenarios; uses tmpl.ReferenceIDTemplate for idempotency)
//
// actorFieldByEventType maps event_type → payload key holding the actor (who triggered the event)。
// 41 號設計書 §3.3.1:publisher 一律 publish,self-author skip 由 notification consumer 判斷。
// 若 actor == target user,skip 寫 inbox 與推 FCM(避免自我通知)。
//
// 新增 event_type 時若該事件需要 self-skip,在此 map 註冊對應的 payload key。
// 未列入 map 的 event_type 不做 self-skip。
//
// ⚠️ key 必須是 template 的 **event_type**，不是 routing key。comment.replied 這個
// routing key 上掛的兩張 template 的 event_type 是 comment.replied.seller /
// comment.replied.buyer（20260703130000_template_ssot_baseline.sql id 24/25），
// 登記成 "comment.replied" 等於整條 self-skip 從未被執行過。
//
// ⚠️ 這裡登記的 actor 欄位必須與該 template 的 target_user_field 落在**同一個
// 身分空間**，否則比對永遠不成立、self-skip 靜默失效（沒有任何錯誤訊號）：
//   - buyer 側（comment.created.buyer／comment.replied.buyer）不受 issue #2707 影響，
//     target_user_field 一直是解析後的 buyer_id 空間（postAuthorId／replyToUserId），
//     actor 沿用對應的 *PrincipalId 欄位（commenterPrincipalId／replierPrincipalId），
//     維持 #1626 當時定案的配對，這裡沒有任何改動。
//   - seller 側三支（comment.created／comment.replied.seller／comment.like_changed）
//     issue #2707 Phase B batch3（backend-payuni PR TBD）已把 target_user_field 從
//     解析後的 account_id 空間（postAuthorId／replyToUserId／commentAuthorId）改指向
//     Phase A 就已存在、未解析的原始 store_id 欄位（postAuthorStoreId／
//     replyToUserStoreId／commentAuthorStoreId）——因此 actor 現在也必須改用同樣未
//     解析的 surrogate 欄位（commenterId／replierId／userId），**不能**再用
//     *PrincipalId（那三個已解析欄位落在 account_id 空間，跟新 target 不同空間，
//     比對會永遠不成立，是 #1626 那個 bug class 的鏡像重演）。commenterId／
//     replierId／userId 這三個欄位本來就一直存在（gateway SSE 判斷 fromMe 用），
//     Phase B batch3 只是讓 self-skip 也改讀它們，沒有新增任何 payload 欄位。
//
// 部署順序：本次改動只發生在 notification-service 這一端（純資料 migration +
// 本檔這一次程式碼變更），social-service 的 payload 早在 Phase A（PR #2921）就已經
// 同時攜帶新舊兩種欄位，因此沒有部署窗口相容性問題。
var actorFieldByEventType = map[string]string{
	"comment.created":        "commenterId",
	"comment.created.buyer":  "commenterPrincipalId",
	"comment.replied.seller": "replierId",
	"comment.replied.buyer":  "replierPrincipalId",
	"comment.like_changed":   "userId",
}

func handleTemplate(
	ctx context.Context,
	tmpl *domain.NotificationTemplate,
	payload map[string]interface{},
	sendPush *usecase.SendPushUseCase,
	inboxUC *usecase.NotificationInboxUseCase,
	mediaBase string,
	logger *zap.Logger,
) {
	// #1871 OBS-09：有 order_no 的事件把它掛進這次呼叫期間所有 log 行（Loki
	// structured metadata，見 k8s/monitoring/promtail-daemonset.yaml）。多數事件
	// 沒有 order_no（payload 是 map[string]interface{}，不保證存在），只在真的
	// 有才附加，不強制。
	if orderNo, ok := payload["order_no"].(string); ok && orderNo != "" {
		logger = logger.With(zap.String("order_no", orderNo))
	}

	userID, err := extractTargetUserID(payload, tmpl)
	if err != nil {
		logger.Warn("[template] skipping: cannot extract user_id",
			zap.String("event_type", tmpl.EventType),
			zap.String("field", tmpl.TargetUserField),
			zap.Error(err))
		return
	}

	// 41 號設計書 §3.3.1:self-notification skip。actor (留言者/回覆者/按讚者) ==
	// target user (收通知者) 時不寫 inbox 也不推 FCM。SSE 路徑不受影響。
	if actorField, ok := actorFieldByEventType[tmpl.EventType]; ok {
		if actorID, aerr := extractUserID(payload, actorField); aerr == nil && actorID == userID {
			logger.Debug("[template] skipping self-notification",
				zap.String("event_type", tmpl.EventType),
				zap.Int64("user_id", userID),
				zap.String("actor_field", actorField))
			return
		}
	}

	silent := tmpl.PushMode == "silent"

	// title/body 對 silent 用不到，但 referenceID 仍要算（idempotency 用）
	var title, body string
	if !silent {
		title, _ = RenderTemplate(tmpl.TitleTemplate, payload)
		var missingBody []string
		body, missingBody = RenderTemplate(tmpl.BodyTemplate, payload)
		if len(missingBody) > 0 && tmpl.BodyFallback != nil {
			body = *tmpl.BodyFallback
		}
	}

	dataMap := BuildDataMap(tmpl.DataFields, payload)
	dataMap["type"] = tmpl.DataType
	dataMap["category"] = tmpl.Category
	if tmpl.SchemaURL != nil {
		dataMap["url"] = RenderBraces(*tmpl.SchemaURL, dataMap)
	}

	// 渲染 image key 模板（payload 圖片欄位值為 objectKey），現組顯示 URL（49 §5.3 #10/§6.3）。
	// BuildURL 對存量完整 URL 走 passthrough，對 objectKey 組裝——producer 遷移期皆不破圖。
	// 有殘留佔位符則降級為空字串。
	imageURL := ""
	if tmpl.ImageKeyTemplate != nil && *tmpl.ImageKeyTemplate != "" {
		rendered, missing := RenderTemplate(*tmpl.ImageKeyTemplate, payload)
		if len(missing) == 0 && !strings.Contains(rendered, "#{") {
			imageURL = mediaurl.BuildURL(mediaBase, rendered)
			dataMap["imageUrl"] = imageURL
		} else {
			logger.Warn("[template] imageUrl placeholder unresolved, omitting",
				zap.String("event_type", tmpl.EventType),
				zap.Strings("missing", missing))
		}
	}

	var referenceID string
	if tmpl.ReferenceIDTemplate != nil && *tmpl.ReferenceIDTemplate != "" {
		referenceID, _ = RenderTemplate(*tmpl.ReferenceIDTemplate, payload)
	}

	// 1. 寫 inbox（僅 visible；silent 不寫，避免占未讀數）
	if !silent {
		if err := inboxUC.CreateInboxItem(ctx, &domain.InboxItem{
			UserID:      userID,
			UserType:    tmpl.TargetUserType,
			EventType:   tmpl.EventType,
			ReferenceID: referenceID,
			Title:       title,
			Body:        body,
			Data:        dataMap,
		}); err != nil {
			logger.Error("[template] failed to write inbox",
				zap.Error(err),
				zap.String("event_type", tmpl.EventType),
				zap.Int64("user_id", userID))
			// 繼續推播，不因 inbox 失敗而中斷
		}
	}

	// 2. 送推播（visible / silent 共用 SendPushUseCase；它依 Silent 切 Send vs SendDataOnly）。
	// userID is the same canonical buyer_id/store_id used for the inbox write;
	// device_tokens and notification_preferences follow that same identity.
	if err := sendPush.Execute(ctx, &dto.SendPushRequest{
		UserID:      userID,
		UserType:    tmpl.TargetUserType,
		EventType:   tmpl.EventType,
		ReferenceID: referenceID,
		Title:       title,
		Body:        body,
		ImageURL:    imageURL,
		Data:        dataMap,
		Category:    tmpl.Category,
		Silent:      silent,
	}); err != nil {
		logger.Error("[template] failed to send push",
			zap.Error(err),
			zap.String("event_type", tmpl.EventType),
			zap.Int64("user_id", userID),
			zap.Bool("silent", silent))
	}
}

// RenderTemplate replaces #{fieldName} placeholders with payload values.
// A placeholder may carry a default: #{fieldName|default}. The default is split
// on the FIRST '|' only, so any further '|' characters are literal parts of the
// default (e.g. #{x|a|b} → field x, default "a|b"). The default is used when the
// field is missing, nil, or renders to an empty string; such a placeholder is
// treated as resolved and is NOT reported as missing (this includes an empty
// default #{field|}, which renders empty and is not reported missing).
//
// Placeholders WITHOUT a '|' keep the legacy contract: a field whose key is
// absent is reported in the missing list and its literal #{fieldName} is kept in
// the output, while a key that is present but nil/empty renders as "" and is NOT
// reported missing.
//
// Returns the rendered string and the list of field names that were missing from
// the payload (only placeholders WITHOUT a default are reported).
func RenderTemplate(tmpl string, payload map[string]interface{}) (string, []string) {
	var sb strings.Builder
	var missing []string
	remaining := tmpl

	for {
		start := strings.Index(remaining, "#{")
		if start == -1 {
			sb.WriteString(remaining)
			break
		}
		end := strings.Index(remaining[start:], "}")
		if end == -1 {
			sb.WriteString(remaining)
			break
		}
		end += start

		inner := remaining[start+2 : end]
		// Split on the FIRST '|' only; further '|' stay part of the default.
		fieldName, defaultVal, hasDefault := strings.Cut(inner, "|")
		sb.WriteString(remaining[:start])

		val, ok := payload[fieldName]
		rendered := ""
		if ok {
			rendered = toStringVal(val)
		}
		switch {
		case rendered != "":
			sb.WriteString(rendered)
		case hasDefault:
			sb.WriteString(defaultVal) // resolved via default; not "missing"
		case !ok:
			missing = append(missing, fieldName)
			sb.WriteString(remaining[start : end+1]) // keep #{fieldName} as-is
		default:
			// key present but empty/nil and no default: legacy behavior (write empty)
		}
		remaining = remaining[end+1:]
	}

	return sb.String(), missing
}

// RenderBraces replaces {key} placeholders with values from the data map.
// Used for schema_url rendering where keys refer to data_fields mapped values.
func RenderBraces(tmpl string, data map[string]string) string {
	var sb strings.Builder
	remaining := tmpl

	for {
		start := strings.Index(remaining, "{")
		if start == -1 {
			sb.WriteString(remaining)
			break
		}
		// Skip #{ sequences (those are renderTemplate's domain)
		if start > 0 && remaining[start-1] == '#' {
			sb.WriteString(remaining[:start+1])
			remaining = remaining[start+1:]
			continue
		}
		end := strings.Index(remaining[start:], "}")
		if end == -1 {
			sb.WriteString(remaining)
			break
		}
		end += start

		key := remaining[start+1 : end]
		sb.WriteString(remaining[:start])
		if val, ok := data[key]; ok {
			sb.WriteString(val)
		} else {
			sb.WriteString(remaining[start : end+1]) // keep {key} as-is
		}
		remaining = remaining[end+1:]
	}

	return sb.String()
}

// BuildDataMap builds the FCM data map by resolving each FieldSpec against the payload.
// See domain.FieldSpec for the const / field strategy contract.
func BuildDataMap(dataFields map[string]domain.FieldSpec, payload map[string]interface{}) map[string]string {
	result := make(map[string]string, len(dataFields)+2)
	for fcmKey, spec := range dataFields {
		val, ok := spec.Resolve(payload)
		if !ok {
			continue
		}
		result[fcmKey] = toStringVal(val)
	}
	return result
}

// extractTargetUserID reads the template's canonical recipient field. Missing or
// malformed recipient values are rejected by the caller, so a bad payload cannot
// create an inbox item or push notification under an unrelated identity.
func extractTargetUserID(payload map[string]interface{}, tmpl *domain.NotificationTemplate) (int64, error) {
	return extractUserID(payload, tmpl.TargetUserField)
}

// extractUserID reads the user ID from payload[field] and converts it to int64.
func extractUserID(payload map[string]interface{}, field string) (int64, error) {
	val, ok := payload[field]
	if !ok {
		return 0, fmt.Errorf("field %q not found in payload", field)
	}
	switch v := val.(type) {
	case float64:
		id := int64(v)
		if id <= 0 {
			return 0, fmt.Errorf("field %q has non-positive value %d", field, id)
		}
		return id, nil
	case json.Number:
		id, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("field %q cannot parse json.Number %q to int64: %w", field, v.String(), err)
		}
		if id <= 0 {
			return 0, fmt.Errorf("field %q has non-positive value %d", field, id)
		}
		return id, nil
	case string:
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("field %q cannot parse string %q to int64: %w", field, v, err)
		}
		if id <= 0 {
			return 0, fmt.Errorf("field %q has non-positive value %d", field, id)
		}
		return id, nil
	default:
		return 0, fmt.Errorf("field %q has unexpected type %T", field, val)
	}
}

// DecodePayload unmarshals an event / sample payload into a generic map while
// preserving integer precision: UseNumber decodes JSON numbers as json.Number
// (string-backed) instead of the lossy float64 that plain json.Unmarshal yields,
// keeping 64-bit TSID ids (> 2^53) exact for downstream data / recipient fields
// (見 issue #1316：postId 被 float64 四捨五入導致通知深連結指向不存在的貼文)。
func DecodePayload(data []byte) (map[string]interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var payload map[string]interface{}
	if err := dec.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// toStringVal converts a JSON-decoded value to its string representation.
func toStringVal(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case json.Number:
		return val.String()
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}

// matchesFilter 回傳 true 當 payload 滿足 filter 中所有 key 的條件。
// 空/nil filter 視為通過。值比對採字串等值（payload 數值會被 toStringVal 轉成字串）。
//
// 每個 key 的條件（domain.FilterCondition）支援兩種：
//   - 等值（Op==""，預設）：payload 必須存在該 key 且值等於 Value，否則不通過
//     （key 不存在 = 不通過，與既有行為一致）
//   - 排除（Op==domain.OpNe，即 $ne）：payload 值 != Value 才通過；key 不存在視為
//     「必然不等於」，通過（例：cancel_reason 在買家自願取消時不存在/為 null，
//     $ne 9104 應通過，讓泛用模板仍然觸發）
func matchesFilter(filter map[string]domain.FilterCondition, payload map[string]interface{}) bool {
	if len(filter) == 0 {
		return true
	}
	for key, cond := range filter {
		actual, ok := payload[key]
		switch cond.Op {
		case domain.OpNe:
			if !ok {
				continue // key 不存在，必然不等於 Value → 通過
			}
			if toStringVal(actual) == cond.Value {
				return false
			}
		default:
			if !ok {
				return false
			}
			if toStringVal(actual) != cond.Value {
				return false
			}
		}
	}
	return true
}
