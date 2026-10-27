package io.github.will413028.hawkr.webhook.uber;

/**
 * 從 Uber webhook raw body 抽出的 <b>ingress 層路由資訊</b>（僅此，不含業務語意）。
 *
 * <p>⚠️ 這裡刻意<b>不</b>解析狀態值、courier 物件、manifest 等業務欄位 ——
 * 那是 {@code shipping-service} 側 {@code InboundCapability} 的職責
 * （66 §6.4.1：webhook 服務「只負責驗簽、入 inbox、發事件」）。
 * ingress 只需要回答兩個問題：<b>這是哪一種事件</b>（決定路由）、<b>去重鍵是什麼</b>。
 *
 * @param kind     provider 原生事件型別，原樣保留（寫進 {@code webhook_inbox.webhook_type}）
 * @param eventKey 去重鍵（66 §6.3.3 級 1 或級 2）
 * @param keyTier  去重鍵的級別，用於 log 與告警分辨（級 2 有已知的失效窗口）
 * @param route    本事件的落點（66 §6.3.2 訂閱決策表）
 */
public record UberWebhookEvent(String kind, String eventKey, KeyTier keyTier, Route route) {

    /** 66 §6.3.3 去重鍵級別。 */
    public enum KeyTier {
        /** 級 1：provider event id，全域唯一。 */
        PROVIDER_EVENT_ID,
        /**
         * 級 2：{@code {delivery_id}:{event_type}:{provider_timestamp}:{sha256(raw_body)[0:16]}}。
         * 🔴 已知失效窗口：同一秒內同 delivery 的同型事件且內容完全相同會被誤判重複
         *（{@code courier_update} 會、{@code delivery_status} 不會，因為狀態變了 body 就變）。
         */
        FALLBACK_COMPOSITE
    }

    /** 66 §6.3.2「四種事件與訂閱決策」的落點。 */
    public enum Route {
        /** 進 inbox ＋ 進 MQ。 */
        PUBLISH,
        /**
         * 進 inbox，<b>不</b>進 MQ。
         * {@code courier_update} 每 20 秒一發，全進 MQ ＝ 流量放大 14 倍而沒有 consumer 需要它（陷阱 A3）。
         */
        INBOX_ONLY,
        /**
         * 進 inbox 並標 IGNORED 終態，<b>不</b>進 MQ，且 log 到告警級。
         * 用於「我方未訂閱／尚無落點」的事件：留證據、不建半條處理路徑。
         */
        IGNORE
    }
}
