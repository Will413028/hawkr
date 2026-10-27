package io.github.will413028.hawkr.productimport.service;

import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.common.utils.code.ProductPublicationState;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.grpc.GetProductsBriefInternalResponse;
import io.github.will413028.hawkr.product.grpc.ProductBriefInternal;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemErrorMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemSkuMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import org.junit.jupiter.api.Test;

import java.util.List;

import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.entity.projection.ImportItemBriefProjection;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

class ItemQueryServiceTest {

    private final ProductImportJobMapper jobMapper = mock(ProductImportJobMapper.class);
    private final ProductImportItemMapper itemMapper = mock(ProductImportItemMapper.class);
    private final ProductImportItemSkuMapper skuMapper = mock(ProductImportItemSkuMapper.class);
    private final ProductImportItemAssetMapper assetMapper = mock(ProductImportItemAssetMapper.class);
    private final ProductImportItemErrorMapper errorMapper = mock(ProductImportItemErrorMapper.class);
    private final ProductGrpcClient productClient = mock(ProductGrpcClient.class);

    private final ItemQueryService service = new ItemQueryService(
            jobMapper, itemMapper, skuMapper, assetMapper, errorMapper, productClient);

    @Test
    void getItemDetailReturnsNotFoundForCommittedItem() {
        ProductImportItemEntity item = new ProductImportItemEntity();
        item.setId(100L);
        item.setImportJobId(10L);
        item.setStoreId(20L);
        item.setStatus(ImportItemStatus.COMMITTED.value());
        when(itemMapper.selectById(100L)).thenReturn(item);

        assertThatThrownBy(() -> service.getItemDetail(20L, 10L, 100L))
                .isInstanceOf(StatusRuntimeException.class)
                .extracting(e -> ((StatusRuntimeException) e).getStatus().getCode())
                .isEqualTo(Status.Code.NOT_FOUND);

        verifyNoInteractions(skuMapper, assetMapper, errorMapper, productClient);
    }

    // ==================== #2819：三種狀態必須分得開 ====================
    //
    // 修復前，「正式商品已軟刪」與「對 product-service 的 RPC 失敗」都走
    // `formal == null` 分支、都回空字串，賣家的匯入紀錄無從分辨；而「開啟正式商品」
    // 按鈕照樣顯示、點下去是 404 / 3023「商品已永久移除」。
    //
    // ⚠️ 這三支刻意打**列表**路徑而不是詳情路徑：`getItemDetail` 對 COMMITTED item
    // 一律 NOT_FOUND（見上方 getItemDetailReturnsNotFoundForCommittedItem），
    // 詳情裡那段 formal 查詢是死碼，本次已刪除而不是擴充。

    private ImportItemBriefProjection committedBrief(long itemId, Long formalProductId) {
        return new ImportItemBriefProjection(
                itemId, "KEY-" + itemId, ImportItemStatus.COMMITTED.value(), "商品" + itemId,
                1L, "葉分類", 1, 0, 0, null,
                formalProductId, null, 1, 0, null, null, null);
    }

    private void stubJobAndItems(ImportItemBriefProjection... briefs) {
        ProductImportJobEntity job = new ProductImportJobEntity();
        job.setId(10L);
        job.setStoreId(20L);
        when(jobMapper.selectById(10L)).thenReturn(job);
        when(itemMapper.listBrief(eq(10L), anyBoolean(), anyInt(), anyInt()))
                .thenReturn(List.of(briefs));
        when(itemMapper.selectCount(any())).thenReturn((long) briefs.length);
        when(skuMapper.selectList(any())).thenReturn(List.of());
        when(assetMapper.selectList(any())).thenReturn(List.of());
    }

    @Test
    void listMarksSoftDeletedProductAsRemovedNotUnavailable() {
        stubJobAndItems(committedBrief(100L, 777L));
        // 底層改用不濾 deleted_at 的查詢後，已軟刪商品**會**回得出來，帶 REMOVED
        when(productClient.getProductsBriefInternal(anyLong(), anyList()))
                .thenReturn(GetProductsBriefInternalResponse.newBuilder()
                        .addRecords(ProductBriefInternal.newBuilder()
                                .setProductId("777")
                                .setName("已刪除的商品")
                                .setStatus(ProductStatus.INACTIVE.value())
                                .setStatusLabel(ProductStatus.INACTIVE.label())
                                .setPublicationState(ProductPublicationState.REMOVED.value())
                                .build())
                        .build());

        ItemQueryService.BriefView v = service.listItems(20L, 10L, null, false, false, 1, 20)
                .records().get(0);

        assertThat(v.formalProductPublicationState()).isEqualTo(ProductPublicationState.REMOVED.value());
        assertThat(v.formalProductStatusUnavailable())
                .as("查得到就不是 unavailable —— 「已移除」是確定的答案，不是不知道")
                .isFalse();
        assertThat(v.formalProductStatusLabel()).isNotEmpty();
    }

    @Test
    void listMarksRpcFailureAsUnavailableNotRemoved() {
        stubJobAndItems(committedBrief(100L, 888L));
        when(productClient.getProductsBriefInternal(anyLong(), anyList()))
                .thenThrow(new RuntimeException("product-service unavailable"));

        ItemQueryService.BriefView v = service.listItems(20L, 10L, null, false, false, 1, 20)
                .records().get(0);

        assertThat(v.formalProductStatusUnavailable())
                .as("RPC 失敗＝我方不知道，必須與「已移除」分開")
                .isTrue();
        assertThat(v.formalProductPublicationState())
                .as("不知道就不能宣稱已移除")
                .isEmpty();
    }

    @Test
    void listKeepsLiveProductStatusAndNotUnavailable() {
        stubJobAndItems(committedBrief(100L, 999L));
        when(productClient.getProductsBriefInternal(anyLong(), anyList()))
                .thenReturn(GetProductsBriefInternalResponse.newBuilder()
                        .addRecords(ProductBriefInternal.newBuilder()
                                .setProductId("999")
                                .setName("正常商品")
                                .setStatus(ProductStatus.ACTIVE.value())
                                .setStatusLabel(ProductStatus.ACTIVE.label())
                                .setPublicationState(ProductPublicationState.PUBLIC.value())
                                .build())
                        .build());

        ItemQueryService.BriefView v = service.listItems(20L, 10L, null, false, false, 1, 20)
                .records().get(0);

        assertThat(v.formalProductStatus()).isEqualTo(ProductStatus.ACTIVE.value());
        assertThat(v.formalProductPublicationState()).isEqualTo(ProductPublicationState.PUBLIC.value());
        assertThat(v.formalProductStatusUnavailable()).isFalse();
    }

    @Test
    void listDoesNotMarkNonCommittedItemAsUnavailable() {
        ImportItemBriefProjection pending = new ImportItemBriefProjection(
                101L, "KEY-101", ImportItemStatus.READY.value(), "尚未提交", 1L, "葉分類",
                1, 0, 0, null, null, null, 1, 0, null, null, null);
        stubJobAndItems(pending);

        ItemQueryService.BriefView v = service.listItems(20L, 10L, null, false, false, 1, 20)
                .records().get(0);

        assertThat(v.formalProductStatusUnavailable())
                .as("沒有正式商品的 item 本來就沒有狀態可言，不該被標成查詢失敗")
                .isFalse();
        verifyNoInteractions(productClient);
    }
}
