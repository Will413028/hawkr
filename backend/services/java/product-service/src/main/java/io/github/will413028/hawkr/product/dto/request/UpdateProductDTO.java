package io.github.will413028.hawkr.product.dto.request;

import lombok.Data;

import java.util.List;

/**
 * 更新商品請求 DTO
 * 對應 gRPC UpdateProductRequest
 */
@Data
public class UpdateProductDTO {

    /**
     * 冪等鍵
     */
    private String idempotencyKey;

    /**
     * 商品 ID (UUID)
     */
    private String productId;

    /**
     * 用於權限驗證的賣家 store 代理鍵（對應 proto 欄位 {@code store_id}）
     */
    private Long storeId;

    /**
     * 商品名稱
     */
    private String name;

    /**
     * 分類 ID
     */
    private Long categoryId;

    /**
     * 商品描述（圖文混排）
     */
    private ProductDescriptionDTO description;

    /**
     * 商品圖片列表（混合清單：既有 asset_id + 新增 staged_token）
     */
    private List<ProductImageDTO> images;

    /**
     * SKU 列表
     */
    private List<ProductSkuDTO> skus;

    /**
     * 產地
     */
    private String origin;

    /**
     * 認證列表（賣家原始輸入；非法值由收斂點剔除，不回 4xx）。
     *
     * <p>⚠️ 語意已變更（19 號設計書 §6.1）：本欄<b>不再是公開投影的來源</b>。
     * 它是「賣家本次宣稱了什麼」，會被送進
     * {@code ProductCertificationApplicationService.syncFromProductWrite} 建成申請，
     * 只有 {@code APPROVED} 的申請才會進 {@code product.certifications} 公開投影。
     */
    private List<CertificationDTO> certifications;

    /**
     * proto {@code UpdateProductRequest.update_certifications} 的原值（19 號設計書 §6.3.0）。
     *
     * <p>🔴 <b>收斂點的 Step 0 閘門條件是「本旗標 OR {@link #certifications} 非空」，不是只看本旗標</b>：
     * <ul>
     *   <li>只看旗標 ⇒ 舊 App 送了認證但沒設旗標的請求會被整個略過，<b>賣家送出的認證靜默蒸發</b>
     *       （proto 該欄註解明訂「相容期：未設但 certifications 非空仍視為要更新」）。</li>
     *   <li>完全不看旗標 ⇒ 任何部分更新都會讓「本次送出的 type 集合」變成空集合，
     *       §6.4 把該商品全部 APPROVED 一次轉 WITHDRAWN，<b>法規標章消失且零訊號</b>。</li>
     * </ul>
     * {@code update_certifications = true} ＋ 空陣列是<b>合法且唯一</b>的「清空」表達。
     */
    private boolean updateCertifications;

    /**
     * 狀態
     */
    private String status;

    /**
     * Hashtag 字串列表（任意大小寫，server normalize；上限 10 個，規則見 02 設計書 §3.9）。
     * 僅在 {@link #updateHashtags} 為 true 時生效，否則維持當前綁定。
     */
    private List<String> hashtags;

    /**
     * true = 本次以 {@link #hashtags} 全量取代當前 hashtag 集合；false/null = 維持現狀（不變更）。
     * 對應 proto {@code UpdateProductRequest.update_hashtags}（02 §4.2.2）。
     */
    private boolean updateHashtags;

    /**
     * 樂觀鎖期望版本（56 §6.6.3，數字字串）。空字串／null = 不比對（賣家自助端既有行為）。
     * 對應 proto {@code UpdateProductRequest.expected_version}。
     */
    private String expectedVersion;

    /**
     * 本次寫入是否為 ADM 代行（proto {@code delegation} 有帶即為 true）。
     *
     * <p>只用來決定認證申請的 {@code source}（{@code DELEGATED_ADMIN} vs {@code SELLER}，19 §4.1）
     * ——賣家自助與代行是<b>不同的人</b>做同一件事，稽核必須分得出來。
     *
     * <p>🔴 <b>不得</b>順手拿 {@code DelegationContext.operator_admin_id} 去填
     * {@code submitted_by_account_id}：前者是 {@code admin_accounts.id}、後者是 {@code accounts.id}，
     * <b>兩個 id space 不可互換</b>（delegation.proto 該欄註解明載）。代行列的送出者一律留 NULL，
     * 身分由 {@code source} 與代行稽核列表達。
     */
    private boolean delegated;
}
