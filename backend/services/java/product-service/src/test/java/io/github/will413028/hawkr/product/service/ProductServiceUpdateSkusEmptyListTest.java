package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.product.converter.ProductSkuConverter;
import io.github.will413028.hawkr.product.dto.request.ProductSkuDTO;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * updateSkus 的兩條「不得靜默毀損既有 SKU」不變式。
 *
 * <p>① 空 SKU 清單必須是 no-op（原始 regression，dev 實測 2026-08-05）：proto3 的
 * {@code repeated} 無法區分「欄位缺席」與「空陣列」，所以「只改商品名稱」的
 * PUT /app/products/{id} 到 service 層就是一個空 List。舊實作把它讀成「清空 SKU」→
 * 一次局部更新實體刪光該商品所有 SKU（HTTP 仍然 200）。
 *
 * <p>② 帶了不屬於本商品的 skuId 必須整個請求失敗（#2544）：舊實作對 existingMap 查無的 id
 * 靜默略過 ⇒ 該 id 不進 keepIds ⇒ 刪除步驟把**全部**既有 SKU 算成「不在保留清單」而實體刪除，
 * 同事務再以空集合把 stock_qty 重算成 0。賣家送一個打錯的 skuId 就能清空自己商品的全部規格。
 */
@ExtendWith(MockitoExtension.class)
@org.mockito.junit.jupiter.MockitoSettings(strictness = org.mockito.quality.Strictness.LENIENT)
class ProductServiceUpdateSkusEmptyListTest {

    @Mock ProductSkuRepository productSkuRepository;
    @Mock ProductSkuConverter productSkuConverter;
    @InjectMocks ProductService productService;

    @SuppressWarnings("unchecked")
    private List<ProductSkuEntity> invoke(List<ProductSkuDTO> skuDTOs) throws Exception {
        Method m = ProductService.class.getDeclaredMethod("updateSkus", Long.class, List.class);
        m.setAccessible(true);
        return (List<ProductSkuEntity>) m.invoke(productService, 1L, skuDTOs);
    }

    private ProductSkuEntity existingSku(long id) {
        ProductSkuEntity e = new ProductSkuEntity();
        e.setId(id);
        e.setProductId(1L);
        e.setSkuName("sku-" + id);
        return e;
    }

    @Test
    @DisplayName("skus 為空清單（前端只改名稱）→ 既有 SKU 原封不動，不呼叫任何刪除")
    void emptyListIsNoOp() throws Exception {
        when(productSkuRepository.findByProductId(1L))
                .thenReturn(new ArrayList<>(List.of(existingSku(101L), existingSku(102L))));

        List<ProductSkuEntity> result = invoke(new ArrayList<>());

        assertEquals(List.of(101L, 102L),
                result.stream().map(ProductSkuEntity::getId).toList(),
                "空清單必須原樣回傳既有 SKU（供 updatePriceRange 重算，價格區間才不會被歸零）");
        verify(productSkuRepository, never()).deleteByProductId(anyLong());
        verify(productSkuRepository, never()).deleteByIdIn(org.mockito.ArgumentMatchers.anyList());
    }

    @Test
    @DisplayName("skus 為 null（同樣代表未提供）→ 既有 SKU 原封不動")
    void nullListIsNoOp() throws Exception {
        when(productSkuRepository.findByProductId(1L))
                .thenReturn(new ArrayList<>(List.of(existingSku(101L))));

        List<ProductSkuEntity> result = invoke(null);

        assertEquals(List.of(101L), result.stream().map(ProductSkuEntity::getId).toList());
        verify(productSkuRepository, never()).deleteByProductId(anyLong());
    }

    @Test
    @DisplayName("控制組：帶了 SKU 清單時，未列入的既有 SKU 仍然會被刪除（證明上面兩測不是 vacuously true）")
    void listedSkusStillPruneTheRest() throws Exception {
        when(productSkuRepository.findByProductId(1L))
                .thenReturn(new ArrayList<>(List.of(existingSku(101L), existingSku(102L))));
        when(productSkuRepository.updateById(any())).thenReturn(1);

        List<ProductSkuDTO> dtos = List.of(ProductSkuDTO.builder().id("101").build());
        invoke(dtos);

        verify(productSkuRepository).deleteByIdIn(List.of(102L));
    }

    /** 取出 reflection 呼叫包起來的真實例外。 */
    private Throwable causeOf(List<ProductSkuDTO> dtos) {
        InvocationTargetException wrapped =
                assertThrows(InvocationTargetException.class, () -> invoke(dtos));
        return wrapped.getCause();
    }

    @Test
    @DisplayName("#2544 送了不屬於本商品的 skuId → 拒絕整個請求，既有 SKU 一個都不能被刪")
    void unknownSkuIdRejectsRequestAndKeepsExistingSkus() {
        when(productSkuRepository.findByProductId(1L))
                .thenReturn(new ArrayList<>(List.of(existingSku(101L), existingSku(102L))));

        Throwable cause = causeOf(List.of(ProductSkuDTO.builder().id("999").build()));

        assertInstanceOf(ProductBusinessException.class, cause,
                "必須以業務例外拒絕，才走得到 ProductGrpcService 的 ProductBusinessException catch 分支");
        assertEquals(GrpcResponseCode.PRODUCT_SKU_NOT_FOUND.code(),
                ((ProductBusinessException) cause).code());

        verify(productSkuRepository, never()).deleteByIdIn(org.mockito.ArgumentMatchers.anyList());
        verify(productSkuRepository, never()).deleteByProductId(anyLong());
    }

    @Test
    @DisplayName("#2544 核心不變式：非法 skuId 不得造成既有 SKU 被刪除（舊實作在此把 101/102 刪光）")
    void unknownSkuIdNeverDeletesExistingSkus() {
        when(productSkuRepository.findByProductId(1L))
                .thenReturn(new ArrayList<>(List.of(existingSku(101L), existingSku(102L))));

        try {
            invoke(List.of(ProductSkuDTO.builder().id("999").build()));
        } catch (Exception rejected) {
        }

        verify(productSkuRepository, never()).deleteByIdIn(org.mockito.ArgumentMatchers.anyList());
        verify(productSkuRepository, never()).deleteByProductId(anyLong());
    }

    @Test
    @DisplayName("#2544 fail-fast：非法 id 混在合法 id 之後時，合法的那筆也不得先被寫入")
    void unknownSkuIdRejectsBeforeAnyWrite() {
        when(productSkuRepository.findByProductId(1L))
                .thenReturn(new ArrayList<>(List.of(existingSku(101L))));

        Throwable cause = causeOf(List.of(
                ProductSkuDTO.builder().id("101").build(),
                ProductSkuDTO.builder().id("999").build()));

        assertInstanceOf(ProductBusinessException.class, cause);
        verify(productSkuRepository, never()).updateById(any());
        verify(productSkuRepository, never()).insert(any());
        verify(productSkuRepository, never()).deleteByIdIn(org.mockito.ArgumentMatchers.anyList());
    }

    @Test
    @DisplayName("結構性不變式：updateSkus 的結果集恆非空（商品永遠至少一個 SKU）")
    void resultIsNeverEmpty() throws Exception {
        List<List<ProductSkuDTO>> inputs = new ArrayList<>();
        inputs.add(null);
        inputs.add(new ArrayList<>());
        inputs.add(List.of(ProductSkuDTO.builder().id("101").build()));
        inputs.add(List.of(ProductSkuDTO.builder().id("").build()));

        for (List<ProductSkuDTO> input : inputs) {
            when(productSkuRepository.findByProductId(1L))
                    .thenReturn(new ArrayList<>(List.of(existingSku(101L))));
            when(productSkuRepository.updateById(any())).thenReturn(1);
            when(productSkuConverter.toEntity(any(), anyLong())).thenReturn(existingSku(103L));

            assertFalse(invoke(input).isEmpty(),
                    "updateSkus 結果集為空 ⇒ 該商品會變成零 SKU（price_min 歸 NULL、不可購買），輸入=" + input);
        }
    }

    @Test
    @DisplayName("控制組：新增用的空白 id（無 id = 新增 SKU）不得被誤判成非法 id")
    void blankIdMeansInsertNotRejection() throws Exception {
        when(productSkuRepository.findByProductId(1L))
                .thenReturn(new ArrayList<>(List.of(existingSku(101L))));
        when(productSkuConverter.toEntity(any(), anyLong())).thenReturn(existingSku(103L));

        invoke(List.of(ProductSkuDTO.builder().id("").build()));

        verify(productSkuRepository).insert(any());
    }
}
