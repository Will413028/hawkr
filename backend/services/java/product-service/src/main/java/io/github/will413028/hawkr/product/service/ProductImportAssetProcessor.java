package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.grpc.GrpcClientErrorTranslator;
import io.github.will413028.hawkr.media.grpc.FinalizeImportAssetRequest;
import io.github.will413028.hawkr.media.grpc.FinalizeImportAssetResponse;
import io.github.will413028.hawkr.media.grpc.MediaServiceGrpc;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;
import io.grpc.StatusRuntimeException;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.List;

/**
 * 商品匯入資產轉正處理器（02 §9.3）。
 *
 * <p>負責將 Product Import Service commit 階段帶來的 {@code import_asset_id} 透過 Media Service
 * {@code FinalizeImportAsset} RPC 轉成正式 PRODUCT / PRODUCT_DESC 資產，並回填 {@code assetId /
 * objectKey} 到對應的 gallery item 與 description block。
 *
 * <p>與既有 {@code processImages / processDescriptionImages}（走 stagedToken 路徑）並排存在：
 * 同一個 block / image 內 {@code stagedToken} 與 {@code importAssetId} 互斥，互斥檢查由呼叫端
 * （{@link ProductImportService}）負責，本處理器只處理已知為 import 路徑的條目。
 *
 * <p>失敗策略：任一 finalize 失敗即拋 {@link ProductBusinessException}，由呼叫端事務 rollback；
 * Import Service 可於後續 commit_revision +1 後重試。與既有 staged_token 路徑的「容錯吞例外」
 * 不同，因為 import 路徑沒有「賣家手上有原圖可手動補」的退路，必須讓事務 rollback 確保資料完整。
 */
@Component
public class ProductImportAssetProcessor {

    private static final ObsLogger log = ObsLogger.forClass(ProductImportAssetProcessor.class);

    private static final String MEDIA_TARGET = "media";

    private final GrpcStubFactory grpcStubFactory;

    public ProductImportAssetProcessor(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 處理商品描述中的 import-asset 圖片區塊（02 §9.3）。
     *
     * <p>對「已過濾出來的 import 路徑 block 清單」逐筆呼叫 {@code FinalizeImportAsset(refType=PRODUCT_DESC)}，
     * 把 {@code assetId / objectKey} 回填到 {@link ImportDescriptionBlock#getTargetBlock()}，並把
     * {@code stagedToken} 清空（確保 entity 落盤後不殘留 import 中介值）。
     *
     * <p>{@code stagedToken} 路徑的 block 不在本處理範圍（保持原樣，由
     * {@code ProductService.processDescriptionImages} 處理）。互斥檢查（同一 block 不可同時帶兩種值）
     * 由呼叫端 {@link ProductImportService} 負責。
     *
     * @param productId  正在建立的 product TSID（用於 finalized object key 路徑組裝）
     * @param blocks     已過濾出 import 路徑的 description block list（含 importAssetId + target block 引用）
     * @param storeId    賣家 store 代理鍵（FinalizeImportAsset 鑑權用；對應 media.proto 的
     *                   {@code FinalizeImportAssetRequest.store_id}）
     */
    public void processDescription(Long productId,
                                   List<ImportDescriptionBlock> blocks,
                                   Long storeId) {
        if (blocks == null || blocks.isEmpty()) {
            return;
        }
        for (ImportDescriptionBlock block : blocks) {
            FinalizeImportAssetResponse resp = callFinalize(
                    block.getImportAssetId(),
                    AssetReferenceType.PRODUCT_DESC.value(),
                    productId,
                    storeId);
            ProductEntity.DescriptionBlock target = block.getTargetBlock();
            target.setAssetId(resp.getAssetId());
            target.setObjectKey(resp.getObjectKey());
            target.setStagedToken(null);
        }
        log.info("ProductImportAssetProcessor.description.success",
                "description import 圖片轉正完成，product_id=" + productId + "，count=" + blocks.size());
    }

    /**
     * 處理 gallery 內的 import-asset 圖片（02 §9.3）。
     *
     * <p>輸入為「按 sort_order 排序後的 import 來源圖片清單」；本方法逐筆呼叫
     * {@code FinalizeImportAsset(refType=PRODUCT)}，將回傳的 {@code assetId / objectKey} 組成 GalleryItem
     * 並依輸入順序回傳。失敗即拋 {@link ProductBusinessException}，由呼叫端 rollback。
     */
    public List<ProductEntity.GalleryItem> processGallery(Long productId,
                                                         List<ImportGalleryImage> images,
                                                         Long storeId) {
        List<ProductEntity.GalleryItem> result = new ArrayList<>();
        if (images == null || images.isEmpty()) {
            return result;
        }
        for (ImportGalleryImage img : images) {
            FinalizeImportAssetResponse resp = callFinalize(
                    img.getImportAssetId(),
                    AssetReferenceType.PRODUCT.value(),
                    productId,
                    storeId);
            ProductEntity.GalleryItem item = new ProductEntity.GalleryItem();
            item.setAssetId(resp.getAssetId());
            item.setObjectKey(resp.getObjectKey());
            result.add(item);
        }
        log.info("ProductImportAssetProcessor.gallery.success",
                "gallery import 圖片轉正完成，product_id=" + productId + "，count=" + images.size());
        return result;
    }

    // ==================== 私有：實際呼叫 Media gRPC ====================

    private FinalizeImportAssetResponse callFinalize(String importAssetId,
                                                    String targetRefType,
                                                    Long productId,
                                                    Long storeId) {
        if (importAssetId == null || importAssetId.isBlank()) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "import_asset_id 不可為空");
        }
        // media.proto 的欄位語意為 store_id；此處傳入同一個 seller store scope。
        FinalizeImportAssetRequest request = FinalizeImportAssetRequest.newBuilder()
                .setImportAssetId(importAssetId)
                .setTargetRefType(targetRefType)
                .setStoreId(storeId)
                .setProductId(productId)
                .build();
        try {
            return getStub().finalizeImportAsset(request);
        } catch (StatusRuntimeException e) {
            log.error("ProductImportAssetProcessor.finalize.failed",
                    "FinalizeImportAsset 失敗，import_asset_id=" + importAssetId
                            + "，ref_type=" + targetRefType + "，error=" + e.getMessage());
            throw GrpcClientErrorTranslator.translate(e, ProductBusinessException::new);
        }
    }

    private MediaServiceGrpc.MediaServiceBlockingStub getStub() {
        return grpcStubFactory.blocking(MEDIA_TARGET, MediaServiceGrpc::newBlockingStub);
    }

    // ==================== 載體 record ====================

    /**
     * description 內單一 image block 的 import 載體：對應 proto {@code DescriptionBlock.import_asset_id}
     * 與將被回填的 entity block 引用。
     */
    public static final class ImportDescriptionBlock {
        private final String importAssetId;
        private final ProductEntity.DescriptionBlock targetBlock;

        public ImportDescriptionBlock(String importAssetId, ProductEntity.DescriptionBlock targetBlock) {
            this.importAssetId = importAssetId;
            this.targetBlock = targetBlock;
        }

        public String getImportAssetId() {
            return importAssetId;
        }

        public ProductEntity.DescriptionBlock getTargetBlock() {
            return targetBlock;
        }
    }

    /**
     * gallery 內單一 import 圖片的載體：對應 proto {@code ProductImageImport}。
     */
    public static final class ImportGalleryImage {
        private final String importAssetId;
        private final int sortOrder;

        public ImportGalleryImage(String importAssetId, int sortOrder) {
            this.importAssetId = importAssetId;
            this.sortOrder = sortOrder;
        }

        public String getImportAssetId() {
            return importAssetId;
        }

        public int getSortOrder() {
            return sortOrder;
        }
    }
}
