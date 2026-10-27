package io.github.will413028.hawkr.product.dto.request;

import lombok.Data;

import java.util.List;

/**
 * 建立商品請求 DTO
 * 對應 gRPC CreateProductRequest
 */
@Data
public class CreateProductDTO {

    /**
     * 冪等鍵（必填）
     */
    private String idempotencyKey;

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
     * 商品圖片列表（使用 staged_token）
     */
    private List<ProductImageDTO> images;

    /**
     * SKU 列表
     */
    private List<ProductSkuDTO> skus;

    /**
     * 賣家 store 代理鍵（對應 proto 欄位 {@code store_id}）
     */
    private Long storeId;

    /**
     * 產地
     */
    private String origin;

    /**
     * 認證列表
     */
    private List<CertificationDTO> certifications;

    /**
     * 狀態（使用 ProductStatus.value()）
     * 4001=草稿, 4002=審核中, 4003=未上架, 4004=已上架
     * @see io.github.will413028.hawkr.common.utils.code.ProductStatus
     */
    private String status;

    /**
     * Hashtag 字串列表（賣家原始輸入；server normalize；上限 10 個，規則見 02 設計書 §3.9）。
     */
    private List<String> hashtags;

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
