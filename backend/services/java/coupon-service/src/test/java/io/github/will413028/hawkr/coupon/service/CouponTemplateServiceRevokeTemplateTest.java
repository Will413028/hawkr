package io.github.will413028.hawkr.coupon.service;

import io.github.will413028.hawkr.common.utils.code.IssuerType;
import io.github.will413028.hawkr.coupon.mapper.CouponTemplateMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * Plan C Phase 1 Task 4（附帶修正）— {@code CouponTemplateMapper#revokeTemplate} 的
 * WHERE 子句同時比對 {@code issuer_type='SELLER'} 與 {@code issuer_store_id}，比照既有
 * {@code disableActiveTemplatesBySuspension}/{@code restoreTemplatesFromSuspension} 的
 * 作法補上 issuerType 參數。{@code issuer_store_id} 現為 SELLER-only 欄位（由 V6
 * migration 之後的新 code 寫入路徑保證：ADMIN 建立路徑寫 issuer_account_id、SELLER
 * 建立路徑寫 issuer_store_id），但 V6 這次只做 expand，DB 層的
 * {@code chk_tpl_issuer_identity} CHECK constraint 依 expand-contract 規範
 * （docs/開發規範/DB Migration 相容性規範（expand-contract）.md §4）延後到 contract
 * release 才加，目前尚未真的存在——巧合數值碰撞在「新 code 均已上線」前提下不會發生，
 * 但那是應用層寫入紀律撐住的，不是 DB 保證。保留 issuer_type 過濾是為了與同批方法呼叫
 * 慣例一致，並讓 WHERE 子句本身清楚表達「只收回本店家的模板」——在 CHECK constraint
 * 缺席的這個 release，這層過濾的重要性不降反增。這是 Task 1 review 發現、V3 migration
 * comment 記錄、progress ledger 明確指派給 Task 4 的 pre-existing gap，非 brief Step 3
 * 逐字要求的範圍，此測試/修正屬本任務判斷後主動補上。
 */
@ExtendWith(MockitoExtension.class)
class CouponTemplateServiceRevokeTemplateTest {

    @Mock private CouponTemplateMapper templateMapper;

    private static final Long TEMPLATE_ID = 999L;
    private static final Long STORE_ID = 555L;
    private static final String REASON = "測試收回";
    private static final String SELLER = IssuerType.SELLER.value();

    @Test
    @DisplayName("revokeTemplate 呼叫 mapper 時帶上 issuer_type=SELLER 過濾條件")
    void revokeTemplate_passesSellerIssuerTypeToMapper() {
        when(templateMapper.revokeTemplate(TEMPLATE_ID, STORE_ID, SELLER, REASON)).thenReturn(1);

        CouponTemplateService service = new CouponTemplateService(templateMapper);
        boolean revoked = service.revokeTemplate(TEMPLATE_ID, STORE_ID, REASON);

        assertTrue(revoked);
        verify(templateMapper).revokeTemplate(TEMPLATE_ID, STORE_ID, SELLER, REASON);
    }

    @Test
    @DisplayName("mapper 因 issuer_type 不符（例如模板其實是 ADMIN 建立）回 0 列受影響 → revokeTemplate 回傳 false")
    void revokeTemplate_returnsFalse_whenMapperAffectsZeroRows() {
        when(templateMapper.revokeTemplate(TEMPLATE_ID, STORE_ID, SELLER, REASON)).thenReturn(0);

        CouponTemplateService service = new CouponTemplateService(templateMapper);
        boolean revoked = service.revokeTemplate(TEMPLATE_ID, STORE_ID, REASON);

        assertFalse(revoked);
    }
}
