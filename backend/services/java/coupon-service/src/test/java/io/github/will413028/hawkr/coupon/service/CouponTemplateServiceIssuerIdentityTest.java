package io.github.will413028.hawkr.coupon.service;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.CouponType;
import io.github.will413028.hawkr.coupon.entity.CouponTemplateEntity;
import io.github.will413028.hawkr.coupon.mapper.CouponTemplateMapper;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.time.LocalDateTime;

import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.Mockito.verify;

/**
 * docs/superpowers/specs/2026-08-06-coupon-issuer-identity-schema-split-design.md §4 —
 * ADMIN 建立路徑必須寫 issuerAccountId、SELLER 建立路徑必須寫 issuerStoreId，
 * 兩者互斥（另一欄位維持 null）。V6 migration 這次只做 expand（見該 migration 檔頭
 * 註解），DB 層的 chk_tpl_issuer_identity CHECK constraint 依 expand-contract 規範
 * （docs/開發規範/DB Migration 相容性規範（expand-contract）.md §4）延後到 contract
 * release 才加，目前尚未存在——這裡驗證的應用層寫入路徑，是本 release 唯一把關互斥的地方。
 */
@ExtendWith(MockitoExtension.class)
class CouponTemplateServiceIssuerIdentityTest {

    @Mock private CouponTemplateMapper templateMapper;

    @BeforeAll
    static void initTsid() {
        // coupon-service node-id-base = 500（10 檔 §7.2）
        try {
            TsidUtil.init(500, 0);
        } catch (IllegalStateException alreadyInitialised) {
            // 同一 JVM 內其他測試已初始化，忽略
        }
    }

    private CouponTemplateEntity captureInsertedEntity() {
        ArgumentCaptor<CouponTemplateEntity> captor = ArgumentCaptor.forClass(CouponTemplateEntity.class);
        verify(templateMapper).insert(captor.capture());
        return captor.getValue();
    }

    @Test
    @DisplayName("createTemplate（ADMIN）寫 issuerAccountId，issuerStoreId 維持 null")
    void createTemplate_writesIssuerAccountId_leavesIssuerStoreIdNull() {
        CouponTemplateService service = new CouponTemplateService(templateMapper);

        service.createTemplate(777L, "平台活動券", CouponType.CASH.value(), "1", null,
                0L, 100L, null, null,
                LocalDateTime.now(), LocalDateTime.now().plusDays(7),
                1, -1, null, true);

        CouponTemplateEntity entity = captureInsertedEntity();
        assertEquals(777L, entity.getIssuerAccountId());
        assertNull(entity.getIssuerStoreId());
    }

    @Test
    @DisplayName("createCompensationTemplate（ADMIN）寫 issuerAccountId，issuerStoreId 維持 null")
    void createCompensationTemplate_writesIssuerAccountId_leavesIssuerStoreIdNull() {
        CouponTemplateService service = new CouponTemplateService(templateMapper);

        service.createCompensationTemplate(777L, 100L, 555L);

        CouponTemplateEntity entity = captureInsertedEntity();
        assertEquals(777L, entity.getIssuerAccountId());
        assertNull(entity.getIssuerStoreId());
    }

    @Test
    @DisplayName("createSellerTemplate（SELLER）寫 issuerStoreId，issuerAccountId 維持 null")
    void createSellerTemplate_writesIssuerStoreId_leavesIssuerAccountIdNull() {
        CouponTemplateService service = new CouponTemplateService(templateMapper);

        service.createSellerTemplate(555L, "週末優惠", CouponType.CASH.value(), "1",
                0L, 100L, null, null,
                LocalDateTime.now(), LocalDateTime.now().plusDays(7));

        CouponTemplateEntity entity = captureInsertedEntity();
        assertEquals(555L, entity.getIssuerStoreId());
        assertNull(entity.getIssuerAccountId());
    }

    @Test
    @DisplayName("createSellerCompensationTemplate（SELLER）寫 issuerStoreId，issuerAccountId 維持 null")
    void createSellerCompensationTemplate_writesIssuerStoreId_leavesIssuerAccountIdNull() {
        CouponTemplateService service = new CouponTemplateService(templateMapper);

        service.createSellerCompensationTemplate(555L, 100L, 0L, "CASE-1");

        CouponTemplateEntity entity = captureInsertedEntity();
        assertEquals(555L, entity.getIssuerStoreId());
        assertNull(entity.getIssuerAccountId());
    }
}
