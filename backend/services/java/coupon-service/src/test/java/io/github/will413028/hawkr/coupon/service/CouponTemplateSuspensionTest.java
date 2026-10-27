package io.github.will413028.hawkr.coupon.service;

import io.github.will413028.hawkr.common.utils.code.CouponTemplateStatus;
import io.github.will413028.hawkr.common.utils.code.IssuerType;
import io.github.will413028.hawkr.coupon.mapper.CouponTemplateMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 賣家停權連動模板停用／還原核心邏輯測試（07 §5.20）。
 */
@ExtendWith(MockitoExtension.class)
class CouponTemplateSuspensionTest {

    @Mock CouponTemplateMapper templateMapper;

    private static final long ACCOUNT_ID = 2619961976L;
    private static final String ACTIVE = CouponTemplateStatus.ACTIVE.value();     // 1
    private static final String INACTIVE = CouponTemplateStatus.INACTIVE.value(); // 2
    private static final String SELLER = IssuerType.SELLER.value();               // 2

    @Test
    @DisplayName("停權：有 ACTIVE 賣家自建模板 → 停用為 INACTIVE 並標記 disabled_by_suspension")
    void disableTemplatesBySuspensionWithActiveTemplates() {
        when(templateMapper.disableActiveTemplatesBySuspension(ACCOUNT_ID, SELLER, ACTIVE, INACTIVE))
                .thenReturn(2);

        CouponTemplateService service = new CouponTemplateService(templateMapper);
        int affected = service.disableTemplatesBySuspension(ACCOUNT_ID);

        assertEquals(2, affected);
        verify(templateMapper).disableActiveTemplatesBySuspension(ACCOUNT_ID, SELLER, ACTIVE, INACTIVE);
    }

    @Test
    @DisplayName("停權：無符合條件模板（皆非 ACTIVE 或已因停權停用過）→ 0 副作用，天然冪等")
    void disableTemplatesBySuspensionNoopWhenNoneEligible() {
        when(templateMapper.disableActiveTemplatesBySuspension(ACCOUNT_ID, SELLER, ACTIVE, INACTIVE))
                .thenReturn(0);

        CouponTemplateService service = new CouponTemplateService(templateMapper);
        int affected = service.disableTemplatesBySuspension(ACCOUNT_ID);

        assertEquals(0, affected);
    }

    @Test
    @DisplayName("解除停權：disabled_by_suspension=TRUE 的模板 → 精準還原 ACTIVE 並清旗標")
    void restoreTemplatesFromSuspensionRestoresFlaggedTemplates() {
        when(templateMapper.restoreTemplatesFromSuspension(ACCOUNT_ID, SELLER, ACTIVE, INACTIVE))
                .thenReturn(2);

        CouponTemplateService service = new CouponTemplateService(templateMapper);
        int affected = service.restoreTemplatesFromSuspension(ACCOUNT_ID);

        assertEquals(2, affected);
        verify(templateMapper).restoreTemplatesFromSuspension(ACCOUNT_ID, SELLER, ACTIVE, INACTIVE);
    }

    @Test
    @DisplayName("解除停權：模板停權期間被 admin 另行停用（disabled_by_suspension 仍 FALSE）→ WHERE 排除，不誤還原")
    void restoreTemplatesFromSuspensionSkipsAdminDisabledTemplates() {
        // admin 手動停用的模板 disabled_by_suspension 從未被設為 TRUE，
        // SQL WHERE disabled_by_suspension=TRUE 天然排除，mapper 回 0
        when(templateMapper.restoreTemplatesFromSuspension(ACCOUNT_ID, SELLER, ACTIVE, INACTIVE))
                .thenReturn(0);

        CouponTemplateService service = new CouponTemplateService(templateMapper);
        int affected = service.restoreTemplatesFromSuspension(ACCOUNT_ID);

        assertEquals(0, affected);
    }
}
