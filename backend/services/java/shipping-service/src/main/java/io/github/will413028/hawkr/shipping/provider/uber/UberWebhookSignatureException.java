package io.github.will413028.hawkr.shipping.provider.uber;

/**
 * Uber webhook 驗簽失敗。
 *
 * <h2>🔴 為什麼是例外而不是「回一個 unknown 事件」</h2>
 * 66 §0.3 逐字：「我方 webhook 端點只有兩種合法回應：{@code 200}（已寫進 inbox）與
 * {@code 5xx}（沒寫進去，請重送）。<b>不得回 4xx</b>（Uber 不重試 4xx ⇒ 事件靜默丟失）。
 * <b>唯一例外是驗簽失敗回 {@code 401}</b>。」
 *
 * <p>⇒ 驗簽失敗是<b>唯一</b>要讓端點跳出那條規則的情況，它必須是一個端點層分辨得出來的訊號。
 * 把它表達成「一個無法解析的事件」會讓它落進 200 那一支，於是偽造的請求得到 200
 * 而真正的事件永遠不會被發現少了。
 *
 * <p>⚠️ ingress（webhook 端點）<b>必須</b>捕捉本例外並回 401；捕捉它之後<b>不得</b>再往下寫 inbox。
 */
public class UberWebhookSignatureException extends RuntimeException {

    public UberWebhookSignatureException(String message) {
        super(message);
    }
}
