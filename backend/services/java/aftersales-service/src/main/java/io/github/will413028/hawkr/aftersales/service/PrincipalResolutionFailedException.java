package io.github.will413028.hawkr.aftersales.service;

/**
 * 身分欄位（{@code store_id}／{@code buyer_id} → 登入 {@code account_id}）預先解析失敗 sentinel
 * （issue #2294 P1-2）。
 *
 * <p>只由 {@link AfterSalesEventPublisher#resolveEventPrincipals(Long, Long)} 拋出，
 * 且該方法<b>只在 tx 之外</b>被呼叫。語意是「本輪這一筆不做」：
 * <ul>
 *   <li>還沒有進 tx ⇒ <b>沒有任何 CAS 發生</b>、沒有事件發出、沒有 row lock 被持有；</li>
 *   <li>候選述詞（{@code status='20'} / {@code process_deadline &lt; NOW()} / {@code auto_escalated=false}）
 *       <b>完全未被觸碰</b> ⇒ 下一個 tick 必然重撈同一筆。</li>
 * </ul>
 *
 * <p>⚠️ <b>不是靜默跳過</b>：呼叫端一律以 {@code log.error} 記錄後 continue，
 * 與「publish 失敗 → {@link EventPublishFailedException} → rollback」是同一種可觀測的重試，
 * 差別只在失敗發生在 tx 之前而非之中。
 *
 * <p>⚠️ 這裡刻意<b>不</b>沿用 {@code AfterSalesEventPublisherImpl.resolveNotifyBuyerAccountId}
 * 的 fail-closed 降級（回 {@code 0L}）：那是給 API 路徑用的——使用者的操作不該因為 buyer-service
 * 抖一下就整批失敗，帶 0 發出去至少不會誤投給別人。但排程路徑<b>有下一個 tick</b>，
 * 帶著 {@code 0} 把事件發出去等於用一則永遠路由不到收件人的通知，換掉一次本來會成功的重試。
 */
public class PrincipalResolutionFailedException extends RuntimeException {

    public PrincipalResolutionFailedException(String what, Long id, Throwable cause) {
        super("事件身分欄位解析失敗，本輪跳過等待下一個 tick 重試: " + what + ", id=" + id, cause);
    }

    public PrincipalResolutionFailedException(String what, Long id) {
        this(what, id, null);
    }
}
