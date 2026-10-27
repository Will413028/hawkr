package io.github.will413028.hawkr.order.grpc.client;

import java.util.List;

/**
 * 賣家售後分類結果（aftersales GetSellerCaseClassification 的 domain 映射）。
 * 供賣家工作台 Dashboard 四桶 count 使用（03 §2.1 SSOT）。
 *
 * @param activeCaseCount            active 售後 case 數（= Dashboard「售後處理」卡）
 * @param activeOrderNos             有 active 售後的訂單（生命週期三桶排除用）
 * @param completedOverrideOrderNos  terminal-override 售後且無 active 的訂單（歸第三桶「已結束」用）
 */
public record SellerCaseClassification(
        long activeCaseCount,
        List<String> activeOrderNos,
        List<String> completedOverrideOrderNos) {
}
