package io.github.will413028.hawkr.shipping.provider.uber;

import java.time.Instant;
import java.util.List;

/**
 * Uber Direct 呼叫的請求／回應模型。
 *
 * <h2>🔴 只建模「我方真的會讀的欄位」</h2>
 * API 總覽 §3.1 的回應欄位表有三十幾欄，而 66 的流程只用得到其中不到十欄。
 * 把全部欄位建成 record 的代價是：官方新增／改名欄位時我方會編譯失敗或靜默漏讀，
 * 而那些欄位<b>我們從來不看</b>。⇒ 解析走 {@code JsonNode}，只把用得到的取出來。
 *
 * <p>⚠️ 反過來，用得到的每一欄都必須有契約測試盯著欄名——那是
 * {@code UberInboundWireContractTest} 的職責。
 */
public final class UberApiModels {

    private UberApiModels() {
    }

    /**
     * 一個取件／送件端點。
     *
     * <p>🔴 {@code structuredAddressJson} 是<b>結構化物件</b>的 JSON 字串
     * （street／city／state／zip／country），不是我方 {@code orders.shipping_address} 的單一字串。
     * 66 §6.5 把這一格標為「本節最大落差」，而地址結構化解析器的落點在報價前置（結帳），
     * 不在本 client——到了這裡它<b>必須已經是結構化的</b>。
     *
     * <p>🔴 樓層／門牌補充進 {@link #notes} <b>不進 address</b>——塞進 street 會讓地理編碼失敗（66 §6.5）。
     */
    public record UberWaypoint(String structuredAddressJson,
                               String name,
                               String phone,
                               Double latitude,
                               Double longitude,
                               String notes) {
    }

    /**
     * 配送品項。
     *
     * @param size 🔴 官方是 <b>string enum</b>（U-K），不是向量。由 packing policy 產出
     * @param priceMinorUnits 最小單位（API 總覽 §3.1 逐字「{@code order_items[].price}：minor unit（分）」）
     */
    public record ManifestItem(String name,
                               int quantity,
                               String size,
                               Long priceMinorUnits,
                               String externalId) {
    }

    /** E1 Create Quote 的請求（官方必填：{@code pickup_address}／{@code dropoff_address}，總覽 §2.1）。 */
    public record QuoteRequest(String externalStoreId,
                               UberWaypoint pickup,
                               UberWaypoint dropoff,
                               String pickupReadyDt) {
    }

    /**
     * E1 的回應（總覽 §2.1 欄位表逐字：{@code id}／{@code fee}／{@code currency}／
     * {@code dropoff_eta}／{@code duration}／{@code pickup_duration}／{@code expires}）。
     *
     * @param expires 🔴 官方範例為 15 分鐘後到期（U-A）。capture 前剩餘 &lt; 180 秒即視為過期（T-2）
     */
    public record QuoteResult(String id, long fee, String currency, Instant expires, String dropoffEta) {
    }

    /**
     * E2 Create Delivery 的請求。
     *
     * @param externalId 🔴 我方 <b>pre-commit 鍵</b>（{@code ADR-014}：{@code uber_create_delivery:{order_no}:{leg}}）。
     *                   官方對 idempotency 零命中 ⇒ 去重由我方 DB 的 UNIQUE 保證，<b>不依賴對方</b>；
     *                   這個欄位的作用是讓 {@code 991} 有東西可以反查（總覽 §3.1／§7.2：
     *                   {@code external_order_id}／{@code external_id} 是官方明載會回傳的）
     */
    public record CreateDeliveryRequest(String quoteId,
                                        String externalId,
                                        String externalStoreId,
                                        UberWaypoint pickup,
                                        UberWaypoint dropoff,
                                        List<ManifestItem> manifestItems,
                                        String manifestReference,
                                        String pickupReadyDt) {
    }

    /**
     * E2／E3／E4 共用的配送回應。
     *
     * @param status           🔴 DaaS 家族的 7 值之一（總覽 §7.1）。未知值一律 fail-closed，見 {@link UberStatusMapper}
     * @param courierImminent  🔴 <b>boolean 欄位不是第 8 個狀態</b>（U-I）
     * @param courierPhone     🔴 e164 的<b>可撥打臨時號碼</b> ⇒ 落庫與 log 一律走 {@link UberFieldFormat#maskPhone}
     * @param relatedDeliveries 逆向 leg 的關聯（N2）。🔴 {@code relationship} 未知值 fail-closed
     *                          ⇒ 不建關聯 ＋ 建 case，<b>不做前綴比對、不猜</b>（66 §7.3.3）
     */
    public record DeliveryResult(String id,
                                 String status,
                                 boolean courierImminent,
                                 String externalId,
                                 String trackingUrl,
                                 String courierName,
                                 String courierPhone,
                                 Instant statusAt,
                                 String undeliverableReason,
                                 String cancellationReason,
                                 List<String> relatedDeliveries) {
    }

    /**
     * E7 Get POD 的回應。
     *
     * <p>🔴 {@code content} 是二進位（66 §4.5.2 逐字）。它<b>不得</b>以完整 URL 或 presigned URL
     * 的形式落庫（10 檔 §25）——呼叫端要把它上傳到我方儲存的 {@code secure/} 前綴並只存 objectKey。
     */
    public record PodDocument(byte[] content, String contentType) {
    }
}
