package io.github.will413028.hawkr.webhook.adapter.http;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.webhook.uber.UberWebhookReceiveService;
import io.github.will413028.hawkr.webhook.uber.UberWebhookSignatureException;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.Map;

/**
 * Uber Direct webhook 接收端點。
 *
 * <p>路徑沿用本服務既有慣例（{@code PayUniWebhookController} 是 {@code /webhooks/payuni/notify}）
 * ⇒ 本端點為 {@code /webhooks/uber/delivery}。
 *
 * <h2>🔴 {@code @RequestBody byte[]}，不是 {@code Map}、不是 record</h2>
 * 驗簽是 HMAC-SHA256 over <b>raw body</b>。只要在取得位元組之前經過一次 JSON 反序列化，
 * 後面就再也拿不回那一份位元組了 —— 而重新序列化出來的版本會因欄位順序不同而<b>偶發</b>
 * 驗簽失敗，失敗率取決於資料內容 ⇒ 測不出來、上線才炸（66 §6.3.1 陷阱 A2）。
 *
 * <h2>🔴 只有 200 與 5xx 兩種回應，永遠不回 4xx</h2>
 * Uber 只對 {@code 500/502/503/504} 與 timeout 重試（上限 3 次、窗口約 100 秒），
 * <b>4xx 不重試</b> ⇒ 我方回 4xx 等於宣告「這個事件永久丟失」（API 總覽 §6.3）。
 * 因此：
 * <ul>
 *   <li>本 controller <b>不做任何入參驗證</b>（沒有 {@code @Valid}、沒有 body 格式檢查）
 *       —— 那些檢查的失敗模式就是 4xx</li>
 *   <li>本 controller <b>沒有也不得有</b> {@code @ExceptionHandler}：例外必須一路往上，
 *       由 Spring 預設成 500，Uber 才會重送</li>
 *   <li>🔴 <b>驗簽失敗回 401 是唯一的 4xx 例外</b>（66 §0.3）。2026-09-12 補回本層：
 *       在此之前驗簽只在 shipping-service 的 {@code UberDirectAdapter.inbound()}，
 *       而 ingress 卻已經把事件寫進 {@code webhook_inbox} 並以
 *       {@code uq_inbox_event} 去重 ⇒ 任何人搶先送一個<b>同 event_id 的偽造事件</b>，
 *       就能讓真正的 Uber 事件被判為重複而丟棄
 *       ——<b>未認證的事件壓制攻擊面</b>。
 *       ⚠️ 對它回 401 而不是 200 是刻意的：那不是我方的事件，
 *       沒有理由讓它佔用去重鍵位</li>
 * </ul>
 *
 * <p>⚠️ {@code consumes} 刻意不限定 media type：限定了就會在 content-type 不符時回
 * <b>415</b>（4xx）⇒ 事件永久丟失。寧可收下一個不認得的 body 落進 inbox。
 */
@RestController
@RequestMapping("/webhooks/uber")
public class UberWebhookController {

    private static final ObsLogger log = ObsLogger.forClass(UberWebhookController.class);

    private final UberWebhookReceiveService receiveService;

    public UberWebhookController(UberWebhookReceiveService receiveService) {
        this.receiveService = receiveService;
    }

    /**
     * Uber Direct 回調統一入口（delivery_status／courier_update／refund_request…）。
     *
     * <p>事件分類與落點由 {@code UberWebhookParser} 依 66 §6.3.2 判定，controller 不分流。
     */
    @PostMapping("/delivery")
    public ResponseEntity<String> handleDeliveryWebhook(@RequestBody(required = false) byte[] rawBody,
                                                        @RequestHeader Map<String, String> headers) {
        log.info("UberWebhookController.receive.start", "收到 Uber Webhook");
        try {
            // 其餘例外刻意不 catch：往上拋 → 500 → Uber 重送（catch 成 4xx ＝ 事件永久丟失）
            receiveService.receive(rawBody, headers);
        } catch (UberWebhookSignatureException e) {
            // 🔴 **唯一允許的 4xx**。不記 body、不記簽章（66 §8.6）。
            //
            // ⚠️ 一連串 401 有**兩種完全不同的成因**，而它們在 log 上長得一樣：
            //    ① 有人在打我們 ② 我方 UBER_WEBHOOK_SIGNING_KEYS 沒設好。
            //    分辨的依據是 UberWebhookSignatureConfig 開機時記的
            //    `webhook_uber_signing_key_count` gauge：0 把 ⇒ 是 ②，事件正在全數遺失。
            log.warn("UberWebhookController.receive.unauthorized",
                    "Uber webhook 驗簽失敗，已拒絕（未寫入 inbox）");
            return ResponseEntity.status(401).body("UNAUTHORIZED");
        }
        return ResponseEntity.ok("SUCCESS");
    }
}
