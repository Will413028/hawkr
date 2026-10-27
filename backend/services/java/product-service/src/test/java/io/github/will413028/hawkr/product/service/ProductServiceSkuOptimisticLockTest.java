package io.github.will413028.hawkr.product.service;

import com.baomidou.mybatisplus.annotation.Version;
import io.github.will413028.hawkr.common.grpc.DelegationDeniedException;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.product.converter.ProductSkuConverter;
import io.github.will413028.hawkr.product.dto.request.ProductSkuDTO;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.lang.reflect.Field;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * SKU 層樂觀鎖（#2545）——兩條必須**同時**成立的不變式。
 *
 * <p><b>① 賣家帶舊 version 寫回 SKU 必須被擋。</b> 扣庫存走
 * {@code ProductRepository.reaggregateStockFromSku}，它刻意不 bump {@code product.version}
 * （56 §5.4 F41 第 11 列：那四條是 MQ 驅動的反正規化計數欄，bump 會讓代行編輯在高流量商品上
 * 幾乎必然 409）。於是 product 層的 {@code expectedVersion} 對「表單開著期間發生的扣庫存」
 * 完全失明，而賣家的 PUT 會把 client 送來的 {@code stockQty} 逐 SKU 原樣寫回 ⇒
 * <b>只改個商品名稱存檔就把中間賣掉的庫存復活</b>，HTTP 200、稽核記 SUCCEEDED、DB 真的錯，
 * 沒有任何訊號。窗口 ＝ 表單開著多久。
 *
 * <p><b>② 同一情境下只改 product 其他欄位不得 409。</b> 這條與 ① 一樣重要：只驗 ① 的話，
 * 「把 {@code product.version} 也一起 bump」會是一個看起來能過測試的修法——而那正是設計書
 * 明令避開的 409 阻斷器。SKU 層的鎖之所以不會製造 409 風暴，就是因為它只在請求**確實帶了
 * 那筆 SKU** 時才比對；沒帶 SKU 的商品編輯一條 CAS 都不會發生。
 *
 * <p><b>為什麼 mock 的 {@code updateById} 要按 entity.version 決定回 0 還是 1</b>：
 * 真實的 CAS 由 MyBatis-Plus 的 {@code OptimisticLockerInnerInterceptor} 加上
 * {@code AND version = #{et.version}} 述詞、由 DB 判定，版本不符只回 0 列**不拋例外**。
 * 這裡把那個契約寫成 stub，本測要守的是 service 層的責任：把 <b>client 的</b>
 * expectedVersion 放進送去 CAS 的 entity（不放就是拿本次交易剛讀到的新版本比對自己，
 * 永遠命中，加了 {@code @Version} 也擋不住任何東西），以及 {@code rows == 0} 必須轉成 409。
 *
 * <p>SQL 層那一半（扣庫存／歸還庫存的 {@code @Update} 必須手寫 {@code version = version + 1}）
 * 由 {@code scripts/check-version-bump-coverage.py} 這支 repo 級 gate 守，不在此重複。
 */
@ExtendWith(MockitoExtension.class)
@org.mockito.junit.jupiter.MockitoSettings(strictness = org.mockito.quality.Strictness.LENIENT)
class ProductServiceSkuOptimisticLockTest {

    /** 賣家開啟編輯表單那一刻讀到的 SKU 版本。 */
    private static final long VERSION_WHEN_FORM_OPENED = 5L;
    /** 表單開著的期間有買家下單扣了庫存 ⇒ deductStock 把 SKU 版本推到這裡。 */
    private static final long VERSION_AFTER_STOCK_DEDUCTED = 6L;

    @Mock ProductSkuRepository productSkuRepository;
    @Mock ProductSkuConverter productSkuConverter;
    @InjectMocks ProductService productService;

    @SuppressWarnings("unchecked")
    private List<ProductSkuEntity> invoke(List<ProductSkuDTO> skuDTOs) throws Exception {
        Method m = ProductService.class.getDeclaredMethod("updateSkus", Long.class, List.class);
        m.setAccessible(true);
        return (List<ProductSkuEntity>) m.invoke(productService, 1L, skuDTOs);
    }

    private Throwable causeOf(List<ProductSkuDTO> dtos) {
        InvocationTargetException wrapped =
                assertThrows(InvocationTargetException.class, () -> invoke(dtos));
        return wrapped.getCause();
    }

    private ProductSkuEntity skuAtVersion(long id, long version) {
        ProductSkuEntity e = new ProductSkuEntity();
        e.setId(id);
        e.setProductId(1L);
        e.setSkuName("sku-" + id);
        e.setStockQty(30);
        e.setVersion(version);
        return e;
    }

    /**
     * CAS 當下 entity 帶的版本。
     *
     * <p>不能用 {@code ArgumentCaptor}：它捕捉的是**參考**，而 updateSkus 在 CAS 成功之後會把
     * 同一個 entity 的 version +1 ⇒ 事後讀 captor 讀到的是寫入後的值，斷言會對著錯的東西成立
     * 或失敗（本測第一版即如此）。要驗「送去 CAS 的是哪個版本」，只能在 stub 內當場記下來。
     */
    private final List<Long> versionsSeenByCas = new ArrayList<>();

    /** DB 現在停在 dbVersion；entity 帶的版本對得上才寫得進去（＝ MyBatis-Plus 樂觀鎖的契約）。 */
    private void dbIsAtVersion(long dbVersion) {
        when(productSkuRepository.updateById(any())).thenAnswer(inv -> {
            ProductSkuEntity e = inv.getArgument(0);
            versionsSeenByCas.add(e.getVersion());
            return e.getVersion() != null && e.getVersion() == dbVersion ? 1 : 0;
        });
    }

    /** 與 {@link #dbIsAtVersion} 同樣會記錄版本，但一律回寫入成功（用於只看「送出什麼」的測）。 */
    private void casAlwaysHits() {
        when(productSkuRepository.updateById(any())).thenAnswer(inv -> {
            versionsSeenByCas.add(((ProductSkuEntity) inv.getArgument(0)).getVersion());
            return 1;
        });
    }

    private void skuInDbIs(ProductSkuEntity entity) {
        when(productSkuRepository.findByProductId(1L)).thenReturn(new ArrayList<>(List.of(entity)));
    }

    // ── ⓪ 攔截器開關 ─────────────────────────────────────────────────────────

    @Test
    @DisplayName("⓪ ProductSkuEntity.version 必須帶 @Version —— 少了它，本檔其餘每一測都是裝飾品")
    void skuVersionFieldCarriesVersionAnnotation() throws Exception {
        Field version = ProductSkuEntity.class.getDeclaredField("version");

        assertNotNull(version.getAnnotation(Version.class),
                "ProductSkuEntity.version 缺少 @Version。OptimisticLockerInnerInterceptor 只認這個註解，"
                        + "沒有它 updateById 不會補上 AND version = #{et.version} 述詞 ⇒ 每次都寫得進去、"
                        + "rows 恆為 1 ⇒ requireSkuCasHit 永不觸發 ⇒ #2545 的 SKU 樂觀鎖整個歸零。"
                        + "本檔其餘測試的 rows=0 全部由 mock stub 自己算出來（見類別 javadoc），"
                        + "分辨不出真實述詞在不在 ⇒ 拿掉註解後那些測試照樣全綠。");
    }

    // ── ① 賣家帶舊 version 寫回 ────────────────────────────────────────────────

    @Test
    @DisplayName("① 表單開著期間被扣了庫存 → 賣家帶舊 version 寫回，必須 409 而不是把庫存復活")
    void staleSkuVersionIsRejected() {
        skuInDbIs(skuAtVersion(101L, VERSION_AFTER_STOCK_DEDUCTED));
        dbIsAtVersion(VERSION_AFTER_STOCK_DEDUCTED);

        // 賣家表單裡的 stockQty 還是開表單那一刻的 30（實際已經賣掉一部分）
        Throwable cause = causeOf(List.of(ProductSkuDTO.builder()
                .id("101")
                .stockQty(30)
                .expectedVersion(String.valueOf(VERSION_WHEN_FORM_OPENED))
                .build()));

        assertInstanceOf(DelegationDeniedException.class, cause,
                "必須是 UpdateProduct RPC 認得的例外型別，否則會落進 catch-all 變成 500");
        assertEquals(GrpcResponseCode.DELEGATION_RESOURCE_VERSION_CONFLICT.code(),
                ((DelegationDeniedException) cause).code(),
                "9266「資料已被更新，請重新載入後再試」——賣家自助端與代行端共用同一個語意");
    }

    @Test
    @DisplayName("① 核心機制：送去 CAS 的 entity 必須帶 client 的 expectedVersion，不是剛讀到的新版本")
    void clientExpectedVersionReachesTheCas() throws Exception {
        skuInDbIs(skuAtVersion(101L, VERSION_AFTER_STOCK_DEDUCTED));
        casAlwaysHits();   // 這一測不看 CAS 結果，只看送去 CAS 的是哪個版本

        invoke(List.of(ProductSkuDTO.builder()
                .id("101")
                .expectedVersion(String.valueOf(VERSION_WHEN_FORM_OPENED))
                .build()));

        assertEquals(List.of(VERSION_WHEN_FORM_OPENED), versionsSeenByCas,
                "沒把 client 的版本蓋進去，CAS 就是拿本交易剛讀到的版本比對自己 ⇒ 永遠命中 ⇒ 鎖是裝飾品");
    }

    @Test
    @DisplayName("① rows=0 不得被當成成功：靜默不寫比報錯更糟（HTTP 200 但資料沒進去）")
    void casMissIsNotSilentSuccess() {
        skuInDbIs(skuAtVersion(101L, VERSION_AFTER_STOCK_DEDUCTED));
        dbIsAtVersion(VERSION_AFTER_STOCK_DEDUCTED);

        Throwable cause = causeOf(List.of(ProductSkuDTO.builder()
                .id("101")
                .expectedVersion("999")     // 任何對不上的值
                .build()));

        assertInstanceOf(DelegationDeniedException.class, cause);
    }

    // ── ② 不得製造新的 409 風暴 ──────────────────────────────────────────────

    @Test
    @DisplayName("② 只改 product 其他欄位（skus 未帶）→ 一條 SKU CAS 都不做，扣過庫存也不會 409")
    void productOnlyEditNeverTouchesSkuVersion() {
        // 前提：庫存剛被扣過，SKU 版本已經前進；product.version 依設計不動
        skuInDbIs(skuAtVersion(101L, VERSION_AFTER_STOCK_DEDUCTED));

        assertDoesNotThrow(() -> invoke(new ArrayList<>()),
                "SKU 層的鎖只在請求確實帶了那筆 SKU 時才比對；否則就變成設計書要避開的 409 阻斷器");

        verify(productSkuRepository, never()).updateById(any());
    }

    @Test
    @DisplayName("② 沒送 expectedVersion 的舊 client → 沿用既有語意（不比對），不得平白 409")
    void absentExpectedVersionKeepsLegacyBehaviour() throws Exception {
        skuInDbIs(skuAtVersion(101L, VERSION_AFTER_STOCK_DEDUCTED));
        dbIsAtVersion(VERSION_AFTER_STOCK_DEDUCTED);

        List<ProductSkuEntity> result = assertDoesNotThrow(() -> invoke(List.of(
                ProductSkuDTO.builder().id("101").stockQty(30).build())));

        assertEquals(1, result.size());
        assertEquals(List.of(VERSION_AFTER_STOCK_DEDUCTED), versionsSeenByCas,
                "未提供時 entity 保持剛讀到的版本 ⇒ CAS 必命中 ⇒ 行為與本次修改前完全相同");
    }

    // ── 邊界 ────────────────────────────────────────────────────────────────

    @Test
    @DisplayName("CAS 成功後回傳的 entity 版本要 +1，才不會把舊版本再下發給 client")
    void versionAdvancesAfterSuccessfulCas() throws Exception {
        skuInDbIs(skuAtVersion(101L, VERSION_AFTER_STOCK_DEDUCTED));
        dbIsAtVersion(VERSION_AFTER_STOCK_DEDUCTED);

        List<ProductSkuEntity> result = invoke(List.of(ProductSkuDTO.builder()
                .id("101")
                .expectedVersion(String.valueOf(VERSION_AFTER_STOCK_DEDUCTED))
                .build()));

        assertEquals(VERSION_AFTER_STOCK_DEDUCTED + 1, result.get(0).getVersion(),
                "下發舊版本 = client 下一次寫回必定 409，且它自己完全看不出原因");
    }

    @Test
    @DisplayName("expectedVersion 不是數字 → 400 參數錯誤，不得靜默當成「沒帶」而放行")
    void nonNumericExpectedVersionIsRejected() {
        skuInDbIs(skuAtVersion(101L, VERSION_AFTER_STOCK_DEDUCTED));

        Throwable cause = causeOf(List.of(ProductSkuDTO.builder()
                .id("101")
                .expectedVersion("abc")
                .build()));

        assertInstanceOf(IllegalArgumentException.class, cause,
                "靜默放行的話，client 送壞了 token 反而拿到「寫入成功」——比 409 危險");
        verify(productSkuRepository, never()).updateById(any());
    }
}
