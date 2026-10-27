package io.github.will413028.hawkr.webhook.uber;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.WebhookProvider;
import jakarta.annotation.PostConstruct;
import org.springframework.stereotype.Component;

/**
 * {@code webhook_inbox.provider} 要寫入的 Uber 碼值 —— <b>唯一的取值點</b>。
 *
 * <h2>🔴 為什麼是查表而不是一個字串常數</h2>
 * {@code webhook_inbox.provider} 存的是 CodeMeta 的 <b>Value（數字碼字串）</b>。
 * 這裡<b>不</b>硬編碼那個數字：硬編碼的症狀是「CodeMeta 之後改了號，而 inbox 裡已經有一批
 * 用舊號寫進去的列」—— 那時候的修法是資料修復，不是改一行常數。
 * 查表拿不到的症狀是：ingress 回 5xx、Uber 重送、log 直接指名缺什麼
 * ⇒ 貴，但是<b>看得見</b>，而且沒有任何一列髒資料。
 *
 * <p>本類別是為了「CodeMeta 還沒有 {@code UBER}」那段空窗而寫的（65 §4.5.1 代價 ②、36 §3.5）。
 * {@code UBER}（Value {@code 8242}）已於 2026-09-12 落地 ⇒ {@link #warnIfCodeMissing()} 與
 * {@link #value()} 的例外路徑現在是<b>回歸守衛</b>：有人把碼值拿掉或改名時，
 * 症狀會是開機 WARN ＋ 端點 5xx，而不是「靜默寫進一個 consumer 比不出來的值」。
 */
@Component
public class UberWebhookProviderCode {

    private static final ObsLogger log = ObsLogger.forClass(UberWebhookProviderCode.class);

    /** CodeMeta 的 <b>code name</b>（非 value）。65 §4.5.1／36 §3.5 指定為 {@code UBER}。 */
    public static final String CODE = "UBER";

    /**
     * 開機自檢：缺碼值時留一行 WARN。
     *
     * <p>刻意用 WARN 不用 ERROR：ERROR 會在每次開機觸發通用告警，而這是一個<b>已知且已登記</b>的
     * 缺口，不是新事故。真正要吵的時機是「有 Uber 流量打進來卻寫不進去」——那條路徑會
     * 拋例外＋回 5xx，吵得夠大聲。
     */
    @PostConstruct
    void warnIfCodeMissing() {
        if (!isAvailable()) {
            log.warn("UberWebhookProviderCode.codeMeta.missing",
                    "WebhookProvider CodeMeta 尚未新增 " + CODE
                            + "，Uber webhook ingress 目前無法寫入 webhook_inbox（會回 5xx）");
        }
    }

    /**
     * @return {@code WebhookProvider.UBER.value()} —— 寫進 DB 的是 <b>Value（數字碼字串）</b>，不是 name
     *         （66 §6.3.3 逐字：「寫 {@code 'UBER'} 會讓 consumer 拿字串去比數字碼」）
     * @throws IllegalStateException CodeMeta 尚未新增 {@code UBER}
     */
    public String value() {
        if (!isAvailable()) {
            throw new IllegalStateException(
                    "WebhookProvider CodeMeta 尚未新增 " + CODE + " —— Uber webhook 無法寫入 webhook_inbox。"
                            + "修法：docs/代碼管理（CodeMeta）/內部技術碼.md 的 WebhookProvider 區塊加一列，"
                            + "再跑 docs/代碼管理（CodeMeta）/scripts/generate.sh --all"
                            + "（見 65 §4.5.1 代價 ②、36 §3.5）");
        }
        return WebhookProvider.fromCode(CODE).value();
    }

    /** 已經可用時回 true；供開機自檢與測試用，<b>不得</b>拿來當「不可用就靜默跳過」的分支條件。 */
    public static boolean isAvailable() {
        return WebhookProvider.isValidCode(CODE);
    }
}
