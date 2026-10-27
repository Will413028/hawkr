package io.github.will413028.hawkr.aftersales.service;

/**
 * 售後事件發佈介面。
 * MQ 事件發佈的抽象層，具體實作由 mq 套件提供。
 *
 * <p><b>兩組契約（issue #2294 前置 (i)）</b>：
 * <ul>
 *   <li>{@code publishXxx}（既有）：<b>best-effort</b>，發佈失敗只記 error log，呼叫端拿不到訊號。
 *       API 路徑刻意維持這個語意（MQ 故障不應讓使用者的操作整批失敗）。</li>
 *   <li>{@code tryPublishXxx}（本 issue 新增）：<b>會傳遞失敗</b>，回傳 {@code false} 代表事件沒送出去。
 *       給「永久狀態轉換 / 永久 stamp 之後才發事件」的排程路徑使用——那些路徑一旦事件遺失就
 *       永遠不會重來（設計書 09 §8.5）。呼叫端必須處置 {@code false}：例外 B 路徑不寫事後 stamp、
 *       出路 4 路徑拋例外讓短 tx rollback。</li>
 * </ul>
 * <b>不要把既有 best-effort 呼叫端改成 try 版</b>：那會讓 API 路徑在 MQ 故障時整批 fail-closed，
 * 遠大於本 issue 的範圍（唯一刻意的例外是 {@code createRefundTaskAndAdvanceStatus}，裁決見 09 §5.3.7）。
 */
public interface AfterSalesEventPublisher {

    /**
     * 發佈「等待買家操作 ATM 退款頁面」事件。
     *
     * @param orderNo      訂單編號
     * @param caseNo       售後單編號（賣家取消時為 null）
     * @param refundTaskId 退款任務 ID
     * @param buyerId      買家 ID，用於反解 notify_buyer_account_id（#1626）
     */
    void publishAwaitingBuyerRefundPage(String orderNo, String caseNo, Long refundTaskId, Long buyerId);

    /**
     * 發佈「賣家取消出貨完成」事件（未付款訂單直接完成）。
     *
     * @param orderNo  訂單編號
     * @param cancelId 取消請求 ID
     */
    void publishCancelCompleted(String orderNo, Long cancelId);

    /**
     * §10 發佈「買家已申訴」事件（通知 Seller + Admin channel）。
     */
    void publishAppealSubmitted(String caseNo, String orderNo, Long buyerId, Long storeId, String reason);

    /**
     * §10 發佈「平台裁決完成」事件（通知 Buyer + Seller）。
     *
     * @param refundAmount FULL/PARTIAL_REFUND 有值；REJECT_APPEAL 為 null
     */
    void publishPlatformDecisionMade(String caseNo, String orderNo, Long buyerId, Long storeId,
                                      String outcome, Long refundAmount);

    /**
     * §5.10 AUTO_REJECT（預設）：發佈「售後單因賣家處理逾時系統代為拒絕」事件。
     *
     * @param originalDeadlineHours 設計上的處理期限（小時）；前端 / Notification 顯示用
     */
    void publishCaseAutoRejected(String caseNo, String orderNo, Long buyerId,
                                  Long storeId, int originalDeadlineHours);

    /**
     * §5.10 AUTO_ESCALATE（備援）：發佈「售後單因賣家處理逾時自動升級至 admin 仲裁」事件。
     *
     * @param originalDeadlineHours 設計上的處理期限（小時）；前端 / Notification 顯示用
     */
    void publishCaseAutoEscalated(String caseNo, String orderNo, Long buyerId,
                                   Long storeId, int originalDeadlineHours);

    /**
     * 發佈售後主流程狀態變更事件（fine-grained，11 種 transition 共用同一方法）。
     *
     * <p>由 caller 傳入對應 transition 的 routing_key + event_type（見 MqConstants
     * `ROUTING_KEY_AFTERSALES_CASE_*` / `EVENT_TYPE_AFTERSALES_CASE_*`）。
     * Payload 統一帶 caseNo / orderNo / buyer/seller IDs / actor / occurredAt + eventId，
     * 由 notification-service 各自 template 訂閱與渲染。
     *
     * <p>對應 callsite：
     * <ul>
     *   <li>{@code AfterSaleCaseService.createCase}    → CASE_APPLIED（actor=buyer）</li>
     *   <li>{@code AfterSaleCaseService.cancelCase}    → CASE_CANCELLED_BY_BUYER（actor=buyer）</li>
     *   <li>{@code AfterSaleResolutionService.handleReturnRefund}        → CASE_RETURN_APPROVED（actor=seller）</li>
     *   <li>{@code AfterSaleResolutionService.handlePartialRefund}       → CASE_REFUND_APPROVED（actor=seller）</li>
     *   <li>{@code AfterSaleResolutionService.handleFullRefundNoReturn}  → CASE_REFUND_APPROVED（actor=seller）</li>
     *   <li>{@code AfterSaleResolutionService.handleCouponCompensation}  → CASE_COUPON_COMPENSATED（actor=seller）</li>
     *   <li>{@code AfterSaleResolutionService.handleReject}              → CASE_REJECTED（actor=seller）</li>
     *   <li>{@code ReturnReceiptService.confirmReceipt}                  → CASE_RETURN_RECEIVED（actor=seller）</li>
     * </ul>
     *
     * @param actor "buyer" | "seller" | "admin" | "system"
     */
    void publishCaseTransition(String routingKey, String eventType,
                                String caseNo, String orderNo,
                                Long buyerId, Long storeId,
                                String actor);

    /**
     * 2026-05-22 新增：發佈含逆物流上下文的售後事件，供 notification-service 渲染。
     *
     * <p>payload 在標準欄位之上額外帶入 caller 提供的欄位，例如：
     * <ul>
     *   <li>shipping_method：3001=宅配 / 3002=超商，用於 FCM 內文分流（key 一律 snake_case，對齊 CaseTransitionPayload wire 契約）</li>
     *   <li>deadline_at：ISO 8601 倒數時間（store_pickup_deadline_at 或 auto_confirm_at）</li>
     *   <li>自動完成原因由 event type 區分（CASE_AUTO_COMPLETED_STORE_PICKUP_TIMEOUT vs CASE_AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT），FCM 模板依此分流，不再透過 payload metadata 傳遞</li>
     * </ul>
     *
     * @param extraPayload 額外欄位；null / 空 Map 視為僅標準欄位
     */
    void publishCaseTransitionWithContext(String routingKey, String eventType,
                                           String caseNo, String orderNo,
                                           Long buyerId, Long storeId,
                                           String actor,
                                           java.util.Map<String, Object> extraPayload);

    // ==================== issue #2294 前置 (i)：會傳遞失敗的變體 ====================

    /**
     * {@link #publishAwaitingBuyerRefundPage} 的會傳遞失敗 ＋ <b>零 gRPC</b> 變體。
     *
     * <p>issue #2294 B7：唯一呼叫點 {@code ReturnReceiptService.createRefundTaskAndAdvanceStatus}
     * 在 {@code @Transactional} 內，所以本方法<b>不得</b>自己反解 {@code buyer_id}——
     * {@code principals} 必須由最外層呼叫端在 tx 之外備好
     * （排程走 {@link #resolveEventPrincipals}、API 走 {@link #resolveEventPrincipalsBestEffort}）。
     *
     * @return true = domain event 已送出；false = 送出失敗（已記 error log）
     */
    boolean tryPublishAwaitingBuyerRefundPage(String orderNo, String caseNo, Long refundTaskId,
                                               Long buyerId, EventPrincipals principals);

    /**
     * {@link #publishCancelCompleted} 的會傳遞失敗變體。
     *
     * <p>payload 只有 {@code orderNo} / {@code cancelId}，沒有需要反解的身分欄位
     * ⇒ 本方法內<b>本來就零 gRPC</b>，可直接在短 tx 內呼叫（出路 4）。
     *
     * <p>呼叫點：{@code RefundResultConsumer.closeParentAndPublish} 的賣家取消分支——
     * {@code completeIfNotCompleted} CAS 一旦提交，重投時回 0 ⇒ 事件永久遺失。
     *
     * @return true = 事件已送出；false = 送出失敗（已記 error log）
     */
    boolean tryPublishCancelCompleted(String orderNo, Long cancelId);

    /**
     * <b>在 tx 之外</b>預先解析事件 payload 需要的兩個身分欄位（issue #2294 P1-2）。
     *
     * <p>出路 4 的短 tx 內<b>不得有網路呼叫</b>（見 {@link EventPrincipals} 的說明），
     * 所以 B1／B2／B3 三支排程必須先呼叫本方法拿到 {@link EventPrincipals}，再把它傳進
     * tx 內的 {@code tryPublishXxx}。
     *
     * <p><b>失敗語意刻意與 best-effort 路徑相反</b>：任一側 gRPC 失敗一律拋
     * {@link PrincipalResolutionFailedException}，呼叫端記 error log 後跳過本筆。
     * 此時尚未進 tx ⇒ 候選述詞未變 ⇒ 下一個 tick 必然重來。理由見該例外的 javadoc。
     *
     * @param storeId 賣家 store_id（可為 null ⇒ {@code sellerAccountId} 為 null）
     * @param buyerId 買家 buyer_id（可為 null ⇒ {@code notifyBuyerAccountId} 為 {@code 0L}）
     * @throws PrincipalResolutionFailedException 任一側反解失敗（本筆本輪不做，下一 tick 重試）
     */
    EventPrincipals resolveEventPrincipals(Long storeId, Long buyerId);

    /**
     * {@link #resolveEventPrincipals} 的 <b>best-effort</b> 對應版（issue #2294 B5/B6/B7）。
     *
     * <p>同樣是「tx 外先解好、當參數傳進 tx」，差別只在<b>失敗語意</b>：
     * seller 側失敗回 {@code null}、buyer 側失敗回 {@code 0L}，<b>永不拋例外</b>。
     *
     * <p><b>只給 API 路徑用</b>（gRPC handler → {@code @Transactional} service）。
     * API 沒有「下一個 tick」，賣家按一次「確認收貨」不該因為 buyer-service 抖一下就整筆失敗；
     * 帶 {@code 0} 發出去至少不會誤投給別人。排程路徑一律用 strict 版
     * ——理由見 {@link PrincipalResolutionFailedException} 的 javadoc。
     */
    EventPrincipals resolveEventPrincipalsBestEffort(Long storeId, Long buyerId);

    /**
     * {@link #publishCaseAutoRejected} 的會傳遞失敗變體。
     *
     * <p>issue #2294 P1-2：{@code principals} 必須由呼叫端<b>在進 tx 之前</b>以
     * {@link #resolveEventPrincipals} 解析好，本方法內不再做任何 gRPC。
     *
     * @return true = domain event 已送出；false = 送出失敗（已記 error log）
     */
    boolean tryPublishCaseAutoRejected(String caseNo, String orderNo, Long buyerId,
                                        Long storeId, int originalDeadlineHours,
                                        EventPrincipals principals);

    /**
     * {@link #publishCaseAutoEscalated} 的會傳遞失敗變體。
     *
     * <p>issue #2294 P1-2：{@code principals} 同 {@link #tryPublishCaseAutoRejected}，
     * 必須在 tx 外預先解析。
     *
     * @return true = domain event 已送出；false = 送出失敗（已記 error log）
     */
    boolean tryPublishCaseAutoEscalated(String caseNo, String orderNo, Long buyerId,
                                         Long storeId, int originalDeadlineHours,
                                         EventPrincipals principals);

    /**
     * {@link #publishCaseTransitionWithContext} 的會傳遞失敗變體。
     *
     * <p>回傳值只反映 <b>domain event</b>（{@code hawkr.aftersales} exchange）的成敗；
     * 隨附的賣家工作區 realtime（{@code seller.after_sale.updated}）仍是 best-effort，
     * 它失敗不會讓本方法回 false——那只是前端快取失效提示，漏推有 REST 全刷兜底。
     *
     * <p>⚠️ <b>本多載會在方法內做兩次 gRPC 反解</b>，呼叫端若在 {@code @Transactional} 內
     * 就等於把網路呼叫拉進 tx。<b>tx 內的呼叫點一律改用
     * {@link #tryPublishCaseTransitionWithContext(String, String, String, String, Long, Long, String, java.util.Map, EventPrincipals)}</b>。
     *
     * <p>issue #2294 B5/B6/B7 修完後，本多載<b>唯一合法的生產呼叫端是
     * {@code ReturnTwentyFourHourWarnScheduler}（A1／A2／A3）</b>——那三處走設計書的<b>例外 B</b>：
     * publish 發生在 {@code @Transactional} <b>之外</b>（publish 成功才 stamp），
     * 沒有 tx、沒有 row lock，因此方法內的 gRPC 不構成 §四「出路 4 硬性邊界」的違例。
     * <b>新增任何 tx 內呼叫點都必須改用帶 {@link EventPrincipals} 的多載</b>。
     *
     * @return true = domain event 已送出；false = 送出失敗（已記 error log）
     */
    boolean tryPublishCaseTransitionWithContext(String routingKey, String eventType,
                                                 String caseNo, String orderNo,
                                                 Long buyerId, Long storeId,
                                                 String actor,
                                                 java.util.Map<String, Object> extraPayload);

    /**
     * {@link #tryPublishCaseTransitionWithContext} 的<b>零 gRPC</b> 變體（issue #2294 P1-2）。
     *
     * <p>給出路 4 的短 tx 用：{@code principals} 由呼叫端在 tx 外以
     * {@link #resolveEventPrincipals} 解析好，本方法內只有序列化與一次 AMQP publish。
     *
     * @return true = domain event 已送出；false = 送出失敗（已記 error log）
     */
    boolean tryPublishCaseTransitionWithContext(String routingKey, String eventType,
                                                 String caseNo, String orderNo,
                                                 Long buyerId, Long storeId,
                                                 String actor,
                                                 java.util.Map<String, Object> extraPayload,
                                                 EventPrincipals principals);
}
