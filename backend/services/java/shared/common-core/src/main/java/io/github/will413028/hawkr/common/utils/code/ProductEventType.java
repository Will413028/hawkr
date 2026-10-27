package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 商品領域 MQ 事件類型，全部由 product-service 的 `ProductEventPublisher` 發布到 `hawkr.product` exchange。Code 與 Value 一致（UPPER_SNAKE_CASE），與 routing key 不同 — routing key 為點分式（`product.restock` / `product.restock.target` / `product.certification.*`），Java 側寫於 `MqConstants`、Go 側寫於 `pkg/mq/exchanges.go`。對齊 OrderEventType / BuyerEventType / SocialEventType / NotificationEventType 設計。⚠️ `PRODUCT_CERTIFICATION_VERIFY_REQUESTED` 是本 exchange 上**第一個「自己 publish、自己 consume」**的事件（product-service 現有 8 條 queue 全部消費別人的 exchange），它的 queue `product.certification.verify` **必須在同一個 PR 內登記進 `infra/mq/topology/topology.yaml`**——服務端已 passive declare，漏登記＝訂閱靜默失效（不 crash、不進 DLQ、無告警），而 `static-gates.yml` 的登記一致性檢查**只掃 Go**，Java 側沒有任何 CI 擋得住。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProductEventType implements CodeMeta {

    PRODUCT_RESTOCK("PRODUCT_RESTOCK", "PRODUCT_RESTOCK", "商品補貨", "Routing key `product.restock`；product-service `StockService.cascadeProductStock` 偵測 stock 0→>0 transition 或 `UpdateSellerProductStatus` 4003→4004 + stock>0 時，spring afterCommit publish；buyer-service `restock_consumer` 接後 fan-out"),
    PRODUCT_RESTOCK_TARGET("PRODUCT_RESTOCK_TARGET", "PRODUCT_RESTOCK_TARGET", "補貨通知目標（per-subscriber）", "Routing key `product.restock.target`；buyer-service fan-out 對每個 ACTIVE 訂閱者發一條；notification-service template-driven consumer 自動寫 inbox + 推 FCM；reference_id_template=`restock_#{subscriptionId}` 保 inbox 端冪等（fan-out retry 安全）"),
    PRODUCT_CERTIFICATION_VERIFY_REQUESTED("PRODUCT_CERTIFICATION_VERIFY_REQUESTED", "PRODUCT_CERTIFICATION_VERIFY_REQUESTED", "認證背景查驗請求", "Routing key `product.certification.verify.requested`；認證申請列建立後 afterCommit publish，**由 product-service 自己**消費（queue `product.certification.verify`，Java manual-ack ⇒ topology 走 `dlq:` 不走 `retry:`）去打政府 API 並寫回 `gov_verify_state`。🔴 **不得**為了救事件遺失而新增掃 `gov_verify_state='PENDING'` 的排程 job（同時踩「禁 reconciliation job」與「多副本排程搶佔」兩條規範），可靠性靠 MQ retry/DLQ、人工補救靠 ADM 的「重新查驗」端點"),
    PRODUCT_CERTIFICATION_APPROVED("PRODUCT_CERTIFICATION_APPROVED", "PRODUCT_CERTIFICATION_APPROVED", "認證已核准", "Routing key `product.certification.approved`；notification-service（既有 `notification.product` queue，binding key `#` 全收）通知賣家。⚠️ **同步快取命中即刻核准的情境不發**（賣家當下畫面就看到標章）；背景補課核准與人工核准才發，否則賣家畫面會永遠停在「審核中」"),
    PRODUCT_CERTIFICATION_REJECTED("PRODUCT_CERTIFICATION_REJECTED", "PRODUCT_CERTIFICATION_REJECTED", "認證已駁回", "Routing key `product.certification.rejected`；notification-service 通知賣家，payload 帶 `reasonCode` 與 `publicReason`。🔴 **`internalNote` 絕不進 payload**（僅 ADM 可見）"),
    PRODUCT_CERTIFICATION_REVOKED("PRODUCT_CERTIFICATION_REVOKED", "PRODUCT_CERTIFICATION_REVOKED", "認證已撤銷", "Routing key `product.certification.revoked`；notification-service 通知賣家。標章已從公開投影移除，必須告知");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProductEventType(String code, String value, String label, String description) {
        this.code = code;
        this.value = value;
        this.label = label;
        this.description = description;
    }

    @Override
    public String code() { return code; }

    @Override
    public String value() { return value; }

    @Override
    public String label() { return label; }

    @Override
    public String description() { return description; }

    private static final Map<String, ProductEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductEventType::code, e -> e));

    private static final Map<String, ProductEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProductEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProductEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
