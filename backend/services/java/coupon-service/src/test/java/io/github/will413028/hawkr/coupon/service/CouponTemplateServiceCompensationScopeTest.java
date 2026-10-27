package io.github.will413028.hawkr.coupon.service;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.CouponTargetType;
import io.github.will413028.hawkr.coupon.entity.CouponTemplateEntity;
import io.github.will413028.hawkr.coupon.exception.CouponBusinessException;
import io.github.will413028.hawkr.coupon.mapper.CouponTemplateMapper;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;

/**
 * 補償券適用範圍必須是「顯式指定」的。
 *
 * <p>先前 {@code applicable_store_id} 缺值會被 {@code parseLongOrNull} 吞成 null，
 * 而 null 在 Scope 檢查（{@code CouponInstanceService} 的 template.getApplicableStoreId()
 * != null 短路）等同「任何商店都能用」——也就是一個打錯的欄位名（例如仍送已正名掉的
 * {@code applicableSellerAccountId}）就會把限店補償券靜默放大成全平台券，API 還回 200。
 * 影響金額範圍的欄位不該用「沒填 = 最大範圍」當預設。
 *
 * <p>本測試把該行為釘成紅燈：缺值／0／負值一律 INVALID_PARAM 且不得寫入 DB。
 */
@ExtendWith(MockitoExtension.class)
class CouponTemplateServiceCompensationScopeTest {

    @Mock private CouponTemplateMapper templateMapper;

    private static final Long ADMIN_ACCOUNT_ID = 1L;
    private static final Long AMOUNT = 100L;
    private static final Long STORE_ID = 2685914682L;

    @BeforeAll
    static void initTsid() {
        // coupon-service node-id-base = 500（10 檔 §7.2）
        try {
            TsidUtil.init(500, 0);
        } catch (IllegalStateException alreadyInitialised) {
            // 同一 JVM 內其他測試已初始化，忽略
        }
    }

    @Test
    @DisplayName("applicableStoreId 為 null → INVALID_PARAM，且不得 insert 全平台補償券")
    void createCompensationTemplate_nullStore_rejectedAndNotPersisted() {
        CouponTemplateService service = new CouponTemplateService(templateMapper);

        assertThatThrownBy(() -> service.createCompensationTemplate(ADMIN_ACCOUNT_ID, AMOUNT, null))
                .isInstanceOf(CouponBusinessException.class)
                .hasMessageContaining("applicable_store_id");

        verify(templateMapper, never()).insert(org.mockito.ArgumentMatchers.any());
    }

    @Test
    @DisplayName("applicableStoreId 為 0 / 負值 → INVALID_PARAM（sentinel 不得被當成合法商店）")
    void createCompensationTemplate_nonPositiveStore_rejected() {
        CouponTemplateService service = new CouponTemplateService(templateMapper);

        assertThatThrownBy(() -> service.createCompensationTemplate(ADMIN_ACCOUNT_ID, AMOUNT, 0L))
                .isInstanceOf(CouponBusinessException.class);
        assertThatThrownBy(() -> service.createCompensationTemplate(ADMIN_ACCOUNT_ID, AMOUNT, -1L))
                .isInstanceOf(CouponBusinessException.class);

        verify(templateMapper, never()).insert(org.mockito.ArgumentMatchers.any());
    }

    @Test
    @DisplayName("帶合法 store_id → 建立 COMPENSATION 模板且 scope 落在該商店")
    void createCompensationTemplate_validStore_persistsScopedTemplate() {
        CouponTemplateService service = new CouponTemplateService(templateMapper);

        assertThatCode(() -> service.createCompensationTemplate(ADMIN_ACCOUNT_ID, AMOUNT, STORE_ID))
                .doesNotThrowAnyException();

        ArgumentCaptor<CouponTemplateEntity> captor = ArgumentCaptor.forClass(CouponTemplateEntity.class);
        verify(templateMapper).insert(captor.capture());
        CouponTemplateEntity persisted = captor.getValue();
        assertThat(persisted.getApplicableStoreId()).isEqualTo(STORE_ID);
        assertThat(persisted.getTargetType()).isEqualTo(CouponTargetType.COMPENSATION.value());
    }
}
