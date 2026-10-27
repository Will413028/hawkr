package io.github.will413028.hawkr.aftersales.service;

/**
 * 售後事件 payload 的兩個身分欄位，<b>預先解析</b>後的結果（issue #2294 P1-2）。
 *
 * <p><b>為什麼要有這個型別</b>：出路 4（CAS ＋ publish 同在一個短 tx）成立的前提是
 * 「tx 內只有一列 CAS 與一次 publish」（《多副本排程搶佔規範》§一）。但組 payload 需要
 * {@code store_id → seller account_id}、{@code buyer_id → notify account_id} 兩次 gRPC，
 * 若讓它們發生在 tx 內，seller/buyer service 掛住就會<b>無限期</b>持有 {@code aftersale_cases}
 * 的 row lock 與一條 Hikari 連線——短 tx 名存實亡。
 *
 * <p>⇒ 呼叫端（排程 worker）必須在<b>進 tx 之前</b>呼叫
 * {@link AfterSalesEventPublisher#resolveEventPrincipals(Long, Long)} 解析好，
 * 再把結果當參數傳進 tx 內的 publish。
 *
 * @param sellerAccountId      store_id 反解出的登入 account_id；{@code storeId} 為 null 時為 null
 * @param notifyBuyerAccountId buyer_id 反解出的登入 account_id（notification 路由鍵）；
 *                             {@code buyerId} 為 null 時為 {@code 0L}（安全的「查無此人」路由值）
 */
public record EventPrincipals(Long sellerAccountId, Long notifyBuyerAccountId) {
}
