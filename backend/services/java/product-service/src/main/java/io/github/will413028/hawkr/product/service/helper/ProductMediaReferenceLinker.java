package io.github.will413028.hawkr.product.service.helper;

import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.grpc.client.MediaGrpcClient;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.List;

/**
 * 商品 Media 引用（asset_reference）建立共用元件。
 *
 * <p>封裝「從 gallery / description 收集 assetId」+「呼叫 Media {@code LinkImages} 建立引用」邏輯，
 * 由 {@code ProductService}（互動式建立 / 更新）與 {@code ProductImportService}（匯入轉正）共用，
 * 避免重複實作（docs/開發規範/Java 微服務開發規範.md §15）。
 *
 * <p><b>失敗語意（重要）</b>：本元件的 {@code linkGallery} / {@code linkDescription} <b>不吞例外</b>，
 * 且**所有 caller 都必須讓它傳播** → {@code @Transactional} rollback（互動式建立/更新與匯入轉正一致）。
 * <ul>
 *   <li>吞掉 = commit 出「{@code product.gallery} 指著 assetId、卻沒有 asset_reference」的列，
 *       7 天後被 media GC 連同 R2 物件實體刪除（#1203 / #2222）。</li>
 *   <li>傳播 = product 未落庫，剛轉正的資產無人引用，正好落在 GC 的孤兒回收範圍。</li>
 * </ul>
 * 契約全文見 Media 08 號設計書 §4.6.2（媒體生命週期不變式）與 02 號設計書 §9.3a。
 */
@Component
@RequiredArgsConstructor
public class ProductMediaReferenceLinker {

    private final MediaGrpcClient mediaGrpcClient;

    /**
     * 從 gallery 收集所有有效的 assetId。
     */
    public List<String> collectGalleryAssetIds(List<ProductEntity.GalleryItem> gallery) {
        if (gallery == null || gallery.isEmpty()) {
            return new ArrayList<>();
        }
        List<String> ids = new ArrayList<>();
        for (ProductEntity.GalleryItem item : gallery) {
            if (item != null && item.getAssetId() != null && !item.getAssetId().isEmpty()) {
                ids.add(item.getAssetId());
            }
        }
        return ids;
    }

    /**
     * 從 description 的 image blocks 收集所有有效的 assetId。
     */
    public List<String> collectDescriptionAssetIds(ProductEntity.ProductDescription description) {
        if (description == null || description.getBlocks() == null) {
            return new ArrayList<>();
        }
        List<String> ids = new ArrayList<>();
        for (ProductEntity.DescriptionBlock block : description.getBlocks()) {
            if (block != null && "image".equals(block.getType())
                    && block.getAssetId() != null && !block.getAssetId().isEmpty()) {
                ids.add(block.getAssetId());
            }
        }
        return ids;
    }

    /**
     * 建立 gallery 圖片引用（refType=PRODUCT）。失敗拋出，由 caller 決定是否吞例外。
     */
    public void linkGallery(Long productId, List<ProductEntity.GalleryItem> gallery) {
        List<String> assetIds = collectGalleryAssetIds(gallery);
        if (assetIds.isEmpty()) {
            return;
        }
        mediaGrpcClient.linkImages(assetIds, AssetReferenceType.PRODUCT.value(), productId.toString());
    }

    /**
     * 建立 description 內嵌圖片引用（refType=PRODUCT_DESC）。失敗拋出，由 caller 決定是否吞例外。
     */
    public void linkDescription(Long productId, ProductEntity.ProductDescription description) {
        List<String> assetIds = collectDescriptionAssetIds(description);
        if (assetIds.isEmpty()) {
            return;
        }
        mediaGrpcClient.linkImages(assetIds, AssetReferenceType.PRODUCT_DESC.value(), productId.toString());
    }
}
