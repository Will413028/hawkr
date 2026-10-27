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
        // 這一測會真的走進寫入迴圈。未 stub 的 mock 回 int 預設值 0，而 0 列在 #2545 之後
        // 的語意是「樂觀鎖 CAS 未命中」⇒ 會拋 9266（本測原本只是碰巧沒被這個預設值絆到）。
        // 真實 DB 對「PK 存在且 entity.version 為 null（MyBatis-Plus 跳過樂觀鎖）」回的是 1。
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

        // 唯一一筆是非法 id：舊實作在這裡靜默略過 → keepIds 為空 → 101/102 全被刪 → stock 歸零
        Throwable cause = causeOf(List.of(ProductSkuDTO.builder().id("999").build()));

        assertInstanceOf(ProductBusinessException.class, cause,
                "必須以業務例外拒絕，才走得到 ProductGrpcService 的 ProductBusinessException catch 分支");
        assertEquals(GrpcResponseCode.PRODUCT_SKU_NOT_FOUND.code(),
                ((ProductBusinessException) cause).code());

        // 核心不變式：既有 SKU 原封不動（不是只看回傳碼）
        verify(productSkuRepository, never()).deleteByIdIn(org.mockito.ArgumentMatchers.anyList());
        verify(productSkuRepository, never()).deleteByProductId(anyLong());
    }

    @Test
    @DisplayName("#2544 核心不變式：非法 skuId 不得造成既有 SKU 被刪除（舊實作在此把 101/102 刪光）")
    void unknownSkuIdNeverDeletesExistingSkus() {
        when(productSkuRepository.findByProductId(1L))
                .thenReturn(new ArrayList<>(List.of(existingSku(101L), existingSku(102L))));

        // 這一測刻意不斷言「有沒有拋例外」——它只守「既有 SKU 不能不見」這個不變式本身。
        // 上面 unknownSkuIdRejectsRequestAndKeepsExistingSkus 斷言的是「必須拋業務碼 3024」，
        // 但那個 assertThrows 會在舊實作上先失敗、看不到真正的損害；分開兩測，
        // 把修復還原時的紅字直接指向 deleteByIdIn([101, 102]) 這個實際後果。
        try {
            invoke(List.of(ProductSkuDTO.builder().id("999").build()));
        } catch (Exception rejected) {
            // 拒絕是正解，這裡吞掉不影響下方不變式檢查
        }

        verify(productSkuRepository, never()).deleteByIdIn(org.mockito.ArgumentMatchers.anyList());
        verify(productSkuRepository, never()).deleteByProductId(anyLong());
    }

    @Test
    @DisplayName("#2544 fail-fast：非法 id 混在合法 id 之後時，合法的那筆也不得先被寫入")
    void unknownSkuIdRejectsBeforeAnyWrite() {
        when(productSkuRepository.findByProductId(1L))
                .thenReturn(new ArrayList<>(List.of(existingSku(101L))));

        // 101 合法且排在前面；若驗證寫在寫入迴圈裡，101 會先 UPDATE 出去才拋
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
        // 這條不變式沒有任何一行程式碼表述它——它是兩個分支交互作用出來的湧現性質：
        //   ① 空／null 清單 → 直接回既有 SKU（no-op）
        //   ② 非空清單 → 刪除範圍是 existingSkus - keepIds，而清單裡每一筆都會進 resultSkus
        // ⇒ 兩條路都不可能回空集合。搭配 createProduct 的 skus_count 守衛（建立側恆 ≥1），
        //   product.price_min IS NULL ⟺ 零 SKU 這個狀態在線上不可達。
        //
        // 下面三種輸入涵蓋全部分支組合。若哪天有人加了「清空」分支，這一測就會轉紅。
        List<List<ProductSkuDTO>> inputs = new ArrayList<>();
        inputs.add(null);                                                  // 未提供
        inputs.add(new ArrayList<>());                                     // 空陣列
        inputs.add(List.of(ProductSkuDTO.builder().id("101").build()));    // 更新既有
        inputs.add(List.of(ProductSkuDTO.builder().id("").build()));       // 新增

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

        // id 為空字串（proto3 string 缺席時的預設值）＝新增，不是「查無的既有 SKU」
        invoke(List.of(ProductSkuDTO.builder().id("").build()));

        verify(productSkuRepository).insert(any());
    }
}
