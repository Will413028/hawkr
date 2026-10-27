package io.github.will413028.hawkr.shipping.service;

import com.baomidou.mybatisplus.core.MybatisConfiguration;
import com.baomidou.mybatisplus.core.conditions.AbstractWrapper;
import com.baomidou.mybatisplus.core.conditions.Wrapper;
import com.baomidou.mybatisplus.core.metadata.TableInfoHelper;
import io.github.will413028.hawkr.common.providercall.ProviderCallExecutor;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.shipping.client.SellerGrpcClient;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.mapper.OnboardingLogMapper;
import io.github.will413028.hawkr.shipping.mapper.SellerShipmentSettingMapper;
import io.github.will413028.hawkr.shipping.provider.FulfillmentProviderRegistry;
import io.github.will413028.hawkr.shipping.provider.TestFulfillmentProviders;
import org.apache.ibatis.builder.MapperBuilderAssistant;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code OnboardingService} 的賣家寄件設定查詢<b>選列確定化</b>釘樁。
 *
 * <h2>它證明什麼</h2>
 * {@link OnboardingService#findActiveSetting} 與 {@link OnboardingService#findSettingByMethod}
 * 生成的 SQL 帶著 {@code ORDER BY id ASC}，且它出現在 {@code LIMIT} <b>之前</b>
 * ⇒ 同一個 {@code (store_id, shipping_type)} 有多列時，{@code LIMIT 1} 選到哪一列
 * 不再取決於 PostgreSQL 的查詢計畫與物理順序。
 *
 * <h2>為什麼這條非釘不可</h2>
 * {@code shipping_type} <b>不是</b>唯一鍵——唯一索引是
 * {@code (store_id, shipping_type, COALESCE(goods_type, ''))}
 * （{@code V1__init_shipping_tables.sql} 的 {@code idx_settings_store_type}）
 * ⇒ 超商（{@code 3651}）底下常溫／冷凍<b>合法地</b>各一列。
 * 無 {@code ORDER BY} 的 {@code LIMIT 1} 因此是一個<b>不會報錯</b>的不確定性：
 * 它會隨 {@code VACUUM}、索引變化、計畫變化而換答案，而選到的那一列決定了
 * 寄件人聯絡與地址、{@code return_store_code}，以及寫進
 * {@code shipments.sender_snapshot} 的 {@code onboardingId}。
 *
 * <h2>它不證明什麼（誠實標註）</h2>
 * <ul>
 *   <li>這裡斷言的是 <b>MyBatis-Plus 真的生成出來的 SQL 片段</b>（不是我手寫的字串），
 *       但<b>不是</b> PostgreSQL 的執行結果——shipping-service 沒有 DB 測試基礎設施
 *       （testcontainers 只在 cart / aftersales / coupon）。
 *       「{@code ORDER BY <PK> ASC} ＋ {@code LIMIT 1} 在 PostgreSQL 上是確定的」
 *       由 PK 的全序性保證，不由本測試保證。</li>
 *   <li>它擋得住「有人把 {@code ORDER BY} 拿掉」「有人把方向反轉」「有人把它寫到
 *       {@code LIMIT} 之後」；擋不住「排序鍵選得對不對」——那是設計判斷，理由寫在
 *       被測方法自己的 javadoc 裡。</li>
 *   <li>它<b>不宣稱</b>語意問題被解決了：「該用哪一個溫層的設定」在設計上仍然沒有答案，
 *       本改動只讓答案固定。</li>
 * </ul>
 */
class OnboardingSettingSelectionDeterminismTest {

    /**
     * MyBatis-Plus 的 lambda 欄名解析是<b>延遲到生成 SQL 那一刻</b>才做的
     * （{@code .eq(Entity::getX, v)} 本身不會碰 TableInfo，所以既有的純 mock 測試
     * 從來沒撞到這件事）。要拿到真的 SQL 就得先把這個 entity 的 TableInfo 建起來，
     * 否則 {@code getSqlSegment()} 會拋
     * {@code can not find lambda cache for this entity}。
     */
    @BeforeAll
    static void initMybatisPlusTableInfo() {
        TableInfoHelper.initTableInfo(
                new MapperBuilderAssistant(new MybatisConfiguration(), ""),
                SellerShipmentSettingEntity.class);
    }

    // ---- 硬寫的期望值：一律字面值，不從被測物或它引用的常數推導 ----
    private static final long STORE_ID = 2685914682L;
    /** 超商 = {@code ShippingType.CVS_C2C}；硬寫 {@code "3651"}，跟著 enum 一起變就驗不到東西。 */
    private static final String CVS_SHIPPING_TYPE = "3651";
    /** 宅配 = {@code ShippingType.HOME_DELIVERY}。 */
    private static final String HOME_SHIPPING_TYPE = "3652";
    /** {@code OnboardingStatus.ACTIVE}。 */
    private static final String ACTIVE = "3602";

    /** 三列候選，{@code id} 刻意<b>不按清單順序</b>——否則「完全不排序」也會通過。 */
    private static final long ID_OLDEST = 2900000000000000001L;
    private static final long ID_MIDDLE = 2900000000000000002L;
    private static final long ID_NEWEST = 2900000000000000003L;
    private static final String MER_OLDEST = "MER-OLDEST";
    private static final String MER_MIDDLE = "MER-MIDDLE";
    private static final String MER_NEWEST = "MER-NEWEST";

    private SellerShipmentSettingMapper settingMapper;
    private OnboardingService service;

    @BeforeEach
    void setUp() {
        settingMapper = mock(SellerShipmentSettingMapper.class);
        FulfillmentProviderRegistry registry = TestFulfillmentProviders.realRegistry(
                mock(ShippingClient.class),
                mock(ShipmentStatusMapper.class),
                mock(OnboardingService.class));
        service = new OnboardingService(
                settingMapper,
                mock(OnboardingLogMapper.class),
                mock(SellerGrpcClient.class),
                mock(ShippingClient.class),
                mock(ProviderCallExecutor.class),
                mock(ShippingFeeService.class),
                registry);
    }

    /**
     * 同一個 {@code (store_id, shipping_type=3651)} 下的三列候選。
     *
     * <p>清單順序是 newest → oldest → middle，而 {@code id} 順序是 oldest &lt; middle &lt; newest
     * ⇒ 「回清單第一筆」「按 id ASC」「按 id DESC」三種行為得到<b>三個不同</b>的答案，
     * 斷言才分辨得出來。
     */
    private static List<SellerShipmentSettingEntity> threeCandidates() {
        return List.of(
                row(ID_NEWEST, "3202", MER_NEWEST),
                row(ID_OLDEST, "3201", MER_OLDEST),
                row(ID_MIDDLE, "3203", MER_MIDDLE));
    }

    private static SellerShipmentSettingEntity row(long id, String goodsType, String merId) {
        SellerShipmentSettingEntity e = new SellerShipmentSettingEntity();
        e.setId(id);
        e.setStoreId(STORE_ID);
        e.setShippingType(CVS_SHIPPING_TYPE);
        e.setGoodsType(goodsType);
        e.setMerId(merId);
        e.setOnboardingStatus(ACTIVE);
        e.setEnabled(true);
        return e;
    }

    // ==================================================================
    // fake：只實作「ORDER BY … LIMIT 1 會選到哪一列」，其餘一律拒絕
    // ==================================================================

    private static final Pattern ORDER_BY =
            Pattern.compile("(?i)\\bORDER\\s+BY\\s+([A-Za-z0-9_]+)\\s+(ASC|DESC)\\b");
    private static final Pattern LIMIT = Pattern.compile("(?i)\\bLIMIT\\b");

    /**
     * 對候選列套用 wrapper 真正生成的 {@code ORDER BY}，回第一列。
     *
     * <p>🔴 <b>鑑別力來自它拒絕什麼</b>：沒有 {@code ORDER BY}、{@code ORDER BY} 落在
     * {@code LIMIT} 之後、或排序鍵不是它認得的那一個，一律拋例外——<b>不猜、不預設</b>。
     * 一個「查不到排序就回第一筆」的 fake 在這裡零鑑別力：把 {@code ORDER BY} 拿掉也會綠。
     *
     * <p>⚠️ 它<b>只</b>模型化選列，不模型化 {@code WHERE}：候選列由測試直接給定
     * （＝「DB 已經篩出來的那幾列」）。{@code WHERE} 的正確性由
     * {@code activeSettingPinsFilterParams} 另外用生成 SQL 的參數表釘住。
     */
    private static SellerShipmentSettingEntity pickByGeneratedOrderBy(
            Wrapper<SellerShipmentSettingEntity> wrapper,
            List<SellerShipmentSettingEntity> candidates) {

        String segment = wrapper.getSqlSegment();
        Matcher order = ORDER_BY.matcher(segment);
        if (!order.find()) {
            throw new IllegalStateException(
                    "查詢沒有可辨識的 ORDER BY ⇒ LIMIT 1 選到哪一列由 PostgreSQL 的計畫決定。"
                            + "這個 fake 拒絕假裝它有答案。實際 SQL：" + segment);
        }
        Matcher limit = LIMIT.matcher(segment);
        if (limit.find() && order.start() > limit.start()) {
            throw new IllegalStateException(
                    "ORDER BY 出現在 LIMIT 之後 ⇒ PostgreSQL 會是語法錯誤。實際 SQL：" + segment);
        }

        String column = order.group(1);
        String direction = order.group(2).toUpperCase(java.util.Locale.ROOT);
        if (!"id".equals(column)) {
            throw new IllegalStateException(
                    "這個 fake 只認 id 這個排序鍵（PK ⇒ 全序 ⇒ 唯一答案）；"
                            + "換成可為 NULL 或不唯一的欄位，確定性的論證就不成立，"
                            + "要改本測試而不是讓它默默通過。實際排序鍵：" + column);
        }

        Comparator<SellerShipmentSettingEntity> cmp =
                Comparator.comparing(SellerShipmentSettingEntity::getId);
        if ("DESC".equals(direction)) {
            cmp = cmp.reversed();
        }
        return candidates.stream().sorted(cmp).findFirst().orElse(null);
    }

    private String captureGeneratedSql() {
        @SuppressWarnings("unchecked")
        ArgumentCaptor<Wrapper<SellerShipmentSettingEntity>> captor =
                ArgumentCaptor.forClass(Wrapper.class);
        verify(settingMapper).selectOne(captor.capture());
        return captor.getValue().getSqlSegment();
    }

    private Map<String, Object> captureGeneratedParams() {
        @SuppressWarnings("unchecked")
        ArgumentCaptor<Wrapper<SellerShipmentSettingEntity>> captor =
                ArgumentCaptor.forClass(Wrapper.class);
        verify(settingMapper).selectOne(captor.capture());
        Wrapper<SellerShipmentSettingEntity> wrapper = captor.getValue();
        // ⚠️ MPGENVAL 佔位名與參數值是在「渲染 SQL 那一刻」才產生的（`.eq()` 只存了 lazy
        //    ISqlSegment）⇒ 不先 render 就讀，拿到的是空 Map，而空 Map 會讓
        //    containsExactly... 失敗成「看起來像 WHERE 不見了」。先強制渲染一次。
        //    MergeSegments 會 cache 渲染結果，所以重複呼叫不會產生第二組 MPGENVAL。
        wrapper.getSqlSegment();
        // getParamNameValuePairs() 在 AbstractWrapper 上，不在 Wrapper 介面上。
        return ((AbstractWrapper<?, ?, ?>) wrapper).getParamNameValuePairs();
    }

    // ==================================================================
    // ① findActiveSetting（建單／出貨寫路徑）
    // ==================================================================

    @Nested
    @DisplayName("① findActiveSetting：ACTIVE 設定的選列是確定的")
    class ActiveSetting {

        @Test
        @DisplayName("生成 SQL 以 `ORDER BY id ASC LIMIT 1` 結尾（同時釘住欄位、方向、與 LIMIT 的相對位置）")
        void activeSettingSqlEndsWithDeterministicOrderBeforeLimit() {
            when(settingMapper.selectOne(any())).thenReturn(null);

            service.findActiveSetting(STORE_ID, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());

            String sql = captureGeneratedSql();
            // 一個 endsWith 同時擋掉三種變異：拿掉 ORDER BY、ASC↔DESC、ORDER BY 寫到 LIMIT 之後。
            assertThat(sql)
                    .as("實際生成 SQL：%s", sql)
                    .endsWith("ORDER BY id ASC LIMIT 1");
        }

        @Test
        @DisplayName("WHERE 仍然是 store_id + shipping_type + ACTIVE + enabled 四條（期望值硬寫）")
        void activeSettingPinsFilterParams() {
            when(settingMapper.selectOne(any())).thenReturn(null);

            service.findActiveSetting(STORE_ID, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());

            assertThat(captureGeneratedParams())
                    .containsExactlyInAnyOrderEntriesOf(Map.of(
                            "MPGENVAL1", 2685914682L,
                            "MPGENVAL2", "3651",
                            "MPGENVAL3", "3602",
                            "MPGENVAL4", true));
        }

        @Test
        @DisplayName("三列候選（id 刻意亂序）→ 固定回 id 最小的那一列")
        void activeSettingPicksLowestIdRow() {
            List<SellerShipmentSettingEntity> candidates = threeCandidates();
            when(settingMapper.selectOne(any()))
                    .thenAnswer(inv -> pickByGeneratedOrderBy(inv.getArgument(0), candidates));

            SellerShipmentSettingEntity picked = service.findActiveSetting(
                    STORE_ID, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());

            assertThat(picked).isNotNull();
            assertThat(picked.getId()).isEqualTo(2900000000000000001L);
            assertThat(picked.getMerId()).isEqualTo("MER-OLDEST");
            assertThat(picked.getGoodsType()).isEqualTo("3201");
        }

        /**
         * 同一組候選連查 3 次結果一致。
         *
         * <p>⚠️ 這條<b>不是</b>「跑三次比較保險」的儀式：fake 每次都重新讀一次
         * wrapper 生成的 SQL，所以它驗的是「選列只依賴 SQL、不依賴呼叫次數或
         * 清單的迭代順序」。
         */
        @Test
        @DisplayName("重複查詢回同一列（選列只依賴 SQL）")
        void activeSettingIsRepeatable() {
            List<SellerShipmentSettingEntity> candidates = threeCandidates();
            when(settingMapper.selectOne(any()))
                    .thenAnswer(inv -> pickByGeneratedOrderBy(inv.getArgument(0), candidates));

            for (int i = 0; i < 3; i++) {
                assertThat(service.findActiveSetting(
                        STORE_ID, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value()).getMerId())
                        .isEqualTo("MER-OLDEST");
            }
        }

        /** 宅配（{@code 3652}）走同一條程式路徑，排序保證不能只掛在超商上。 */
        @Test
        @DisplayName("宅配路徑同樣帶 ORDER BY id ASC")
        void homeDeliveryAlsoOrdered() {
            when(settingMapper.selectOne(any())).thenReturn(null);

            service.findActiveSetting(STORE_ID, ShippingMethod.TAQBIN.value());

            // 先確認真的解析成宅配的 shipping_type，否則這條可能在驗別的東西。
            assertThat(captureGeneratedParams()).containsEntry("MPGENVAL2", HOME_SHIPPING_TYPE);
            assertThat(captureGeneratedSql()).endsWith("ORDER BY id ASC LIMIT 1");
        }
    }

    // ==================================================================
    // ② findSettingByMethod（退件讀路徑，候選集更寬）
    // ==================================================================

    @Nested
    @DisplayName("② findSettingByMethod：historical 設定的選列同樣是確定的")
    class SettingByMethod {

        @Test
        @DisplayName("生成 SQL 以 `ORDER BY id ASC LIMIT 1` 結尾")
        void byMethodSqlEndsWithDeterministicOrderBeforeLimit() {
            when(settingMapper.selectOne(any())).thenReturn(null);

            service.findSettingByMethod(STORE_ID, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());

            String sql = captureGeneratedSql();
            assertThat(sql)
                    .as("實際生成 SQL：%s", sql)
                    .endsWith("ORDER BY id ASC LIMIT 1");
        }

        /**
         * 陰性面：這支<b>刻意不過濾</b> {@code onboarding_status} / {@code enabled}
         * （06 §2.3.1：取消 onboarding 不牽連舊單退件）⇒ WHERE 只有兩條。
         * 少了這一條，有人「順手」補上 ACTIVE 過濾會靜默通過，而那會讓退件流程壞掉。
         */
        @Test
        @DisplayName("WHERE 只有 store_id + shipping_type 兩條（不得偷偷加上 ACTIVE / enabled）")
        void byMethodDoesNotFilterStatusOrEnabled() {
            when(settingMapper.selectOne(any())).thenReturn(null);

            service.findSettingByMethod(STORE_ID, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());

            assertThat(captureGeneratedParams())
                    .containsExactlyInAnyOrderEntriesOf(Map.of(
                            "MPGENVAL1", 2685914682L,
                            "MPGENVAL2", "3651"));
        }

        @Test
        @DisplayName("三列候選（id 刻意亂序）→ 固定回 id 最小的那一列")
        void byMethodPicksLowestIdRow() {
            List<SellerShipmentSettingEntity> candidates = threeCandidates();
            when(settingMapper.selectOne(any()))
                    .thenAnswer(inv -> pickByGeneratedOrderBy(inv.getArgument(0), candidates));

            SellerShipmentSettingEntity picked = service.findSettingByMethod(
                    STORE_ID, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());

            assertThat(picked).isNotNull();
            assertThat(picked.getId()).isEqualTo(2900000000000000001L);
            assertThat(picked.getMerId()).isEqualTo("MER-OLDEST");
        }
    }

    // ==================================================================
    // ③ fake 自身的鑑別力（沒有這一組，上面那些斷言可能是空轉）
    // ==================================================================

    /**
     * 🔴 <b>這一組驗的是「測試抓得到什麼」，不是產品行為。</b>
     * {@code 06-no-small-tails.md} §3.10：一個對「看不到」回放行的判準是 fail-open。
     * 這裡明確釘住 fake 對三種壞 SQL 都<b>拋例外</b>，所以上面的選列斷言不會因為
     * 「fake 悄悄退回回第一筆」而變成空轉。
     */
    @Nested
    @DisplayName("③ fake 拒絕它看不懂的 SQL（fail-closed）")
    class FakeIsFailClosed {

        private Wrapper<SellerShipmentSettingEntity> wrapperWith(String lastSql) {
            return new com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper<
                    SellerShipmentSettingEntity>()
                    .eq(SellerShipmentSettingEntity::getStoreId, STORE_ID)
                    .last(lastSql);
        }

        @Test
        @DisplayName("沒有 ORDER BY → 拋例外，不回第一筆")
        void rejectsMissingOrderBy() {
            assertThatThrownBy(() ->
                    pickByGeneratedOrderBy(wrapperWith("LIMIT 1"), threeCandidates()))
                    .isInstanceOf(IllegalStateException.class)
                    .hasMessageContaining("沒有可辨識的 ORDER BY");
        }

        @Test
        @DisplayName("ORDER BY 落在 LIMIT 之後 → 拋例外（那在 PostgreSQL 上是語法錯誤）")
        void rejectsOrderByAfterLimit() {
            assertThatThrownBy(() ->
                    pickByGeneratedOrderBy(wrapperWith("LIMIT 1 ORDER BY id ASC"), threeCandidates()))
                    .isInstanceOf(IllegalStateException.class)
                    .hasMessageContaining("ORDER BY 出現在 LIMIT 之後");
        }

        @Test
        @DisplayName("排序鍵不是 id → 拋例外（不唯一／可為 NULL 的鍵撐不起確定性論證）")
        void rejectsNonPrimaryKeyOrdering() {
            assertThatThrownBy(() ->
                    pickByGeneratedOrderBy(wrapperWith("ORDER BY goods_type ASC LIMIT 1"),
                            threeCandidates()))
                    .isInstanceOf(IllegalStateException.class)
                    .hasMessageContaining("只認 id 這個排序鍵");
        }

        @Test
        @DisplayName("DESC 會選到不同的列 → 方向反轉的變異必然被抓到")
        void descendingPicksDifferentRow() {
            SellerShipmentSettingEntity picked = pickByGeneratedOrderBy(
                    wrapperWith("ORDER BY id DESC LIMIT 1"), threeCandidates());

            assertThat(picked).isNotNull();
            assertThat(picked.getMerId()).isEqualTo("MER-NEWEST");
        }
    }
}
