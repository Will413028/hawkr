package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleAppealEventMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSalePlatformDecisionMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.NullAndEmptySource;
import org.junit.jupiter.params.provider.ValueSource;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.ArgumentMatchers.isNull;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code AfterSaleAppealService#listAppealsForAdmin} 的 appeal_state filter 值域測試。
 *
 * <p>appeal_state 全鏈路只有一種表示法 = AfterSaleAppealState 的 Value：DB 存 Value、
 * 回應欄位吐 Value、Gateway 也只放行 Value（非法值直接 400）。本測試釘住微服務側
 * 同樣只收 Value —— CodeMeta 的 Name（APPEALED / INVESTIGATING / RESOLVED）必須被拒，
 * 否則就是把 #2104 修掉的那條雙軌又接回來。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class AfterSaleAppealServiceFilterTest {

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private AfterSaleCaseService caseService;
    @Mock private AfterSaleAppealEventMapper appealEventMapper;
    @Mock private AfterSalePlatformDecisionMapper platformDecisionMapper;
    @Mock private AfterSaleResolutionMapper resolutionMapper;
    @Mock private EvidenceService evidenceService;
    @Mock private AfterSalesEventPublisher eventPublisher;
    @Mock private AfterSalesProperties properties;

    @InjectMocks private AfterSaleAppealService service;

    @BeforeEach
    void setUp() {
        when(properties.getAppeal()).thenReturn(new AfterSalesProperties.AppealConfig());
        when(caseMapper.countAppealsForAdmin(org.mockito.ArgumentMatchers.any())).thenReturn(0L);
        when(caseMapper.selectAppealsForAdmin(org.mockito.ArgumentMatchers.any(), anyInt(), anyInt()))
                .thenReturn(List.of());
    }

    @DisplayName("CodeMeta Name 一律被拒（Name 分支已物理移除，不再靜默轉成 Value）")
    @ParameterizedTest(name = "appeal_state=\"{0}\" → IllegalArgumentException")
    @ValueSource(strings = {
            "APPEALED", "INVESTIGATING", "RESOLVED", "NONE",
            "appealed", "resolved",   // 舊實作用 equalsIgnoreCase，大小寫變體同樣不得放行
            "ALL"                     // 'all' 只收精確小寫（Gateway 的 allowlist 亦然）
    })
    void rejectsCodeMetaName(String filter) {
        assertThrows(IllegalArgumentException.class,
                () -> service.listAppealsForAdmin(filter, 1, 20));

        verify(caseMapper, never()).countAppealsForAdmin(org.mockito.ArgumentMatchers.any());
        verify(caseMapper, never())
                .selectAppealsForAdmin(org.mockito.ArgumentMatchers.any(), anyInt(), anyInt());
    }

    @DisplayName("NONE 的 Value（'0'）也不合法：申訴列表以 appeal_state <> '0' 為前提")
    @Test
    void rejectsNoneValue() {
        assertThrows(IllegalArgumentException.class,
                () -> service.listAppealsForAdmin("0", 1, 20));
    }

    @DisplayName("非 CodeMeta 的任意字串一律被拒")
    @ParameterizedTest(name = "appeal_state=\"{0}\" → IllegalArgumentException")
    @ValueSource(strings = {"99", "10 ", "APPEALED,RESOLVED", "'10'"})
    void rejectsUnknownFilter(String filter) {
        assertThrows(IllegalArgumentException.class,
                () -> service.listAppealsForAdmin(filter, 1, 20));
    }

    @DisplayName("Value 原樣下傳給 mapper")
    @ParameterizedTest(name = "appeal_state=\"{0}\" → mapper 收到 \"{0}\"")
    @ValueSource(strings = {"10", "20", "30"})
    void passesValueThroughToMapper(String value) {
        service.listAppealsForAdmin(value, 1, 20);

        verify(caseMapper).countAppealsForAdmin(eq(value));
        verify(caseMapper).selectAppealsForAdmin(eq(value), eq(20), eq(0));
    }

    @DisplayName("未帶 appeal_state（null / 空字串）→ 預設 APPEALED 的 Value '10'")
    @ParameterizedTest(name = "appeal_state={0} → \"10\"")
    @NullAndEmptySource
    @ValueSource(strings = {"   "})
    void defaultsToAppealedValue(String filter) {
        service.listAppealsForAdmin(filter, 1, 20);

        verify(caseMapper).countAppealsForAdmin(eq("10"));
        verify(caseMapper).selectAppealsForAdmin(eq("10"), eq(20), eq(0));
    }

    @DisplayName("'all' → 傳 null 給 mapper（不加 appeal_state 條件）")
    @Test
    void allMapsToNullFilter() {
        service.listAppealsForAdmin("all", 2, 50);

        verify(caseMapper).countAppealsForAdmin(isNull());
        verify(caseMapper).selectAppealsForAdmin(isNull(), eq(50), eq(50));
    }
}
