package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleAppealEventMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSalePlatformDecisionMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAppealState;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * backend#3262 鏈路第 7 格的冪等性：<b>Admin 裁決不得重複建立退款</b>。
 *
 * <p>本票的資損鏈路是「撥款 → 之後仍可成立退款」。閘門把撥款擋在退款可能性之後，
 * 而這一支守的是另一半——退款側本身不得被重複觸發（否則擋住撥款也只是換一種賠法）。
 *
 * <p>唯一的保護是 {@code markAppealResolved} 的樂觀鎖 CAS（{@code WHERE appeal_state = '10'}）：
 * 它必須<b>在建立 refund_task 之前</b>執行且 affected=1 才繼續。順序寫反、或把 CAS 的回傳值
 * 當成可忽略的，都會讓兩次併發裁決各建一張退款單。
 */
@ExtendWith(MockitoExtension.class)
class PlatformDecisionRefundIdempotencyTest {

    private static final String CASE_NO = "AS3262001";
    private static final long ADMIN_ID = 26000000L;

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private AfterSalePlatformDecisionMapper platformDecisionMapper;
    @Mock private AfterSaleResolutionMapper resolutionMapper;
    @Mock private AfterSaleAppealEventMapper appealEventMapper;
    @Mock private RefundTaskMapper refundTaskMapper;
    @Mock private AfterSalesEventPublisher eventPublisher;

    private PlatformDecisionService service;

    @BeforeEach
    void setUp() {
        // refund_task 的 id 走 TSID（node-id-base 700 = aftersales，§7.2）
        try { io.github.will413028.hawkr.common.utils.TsidUtil.init(700, 0); } catch (IllegalStateException ignored) { /* 已初始化 */ }
        service = new PlatformDecisionService(caseMapper, platformDecisionMapper, resolutionMapper,
                appealEventMapper, refundTaskMapper, new AfterSalesProperties(), eventPublisher);
    }

    private AfterSaleCaseEntity appealedCase() {
        AfterSaleCaseEntity c = new AfterSaleCaseEntity();
        c.setCaseNo(CASE_NO);
        c.setOrderNo("O3262");
        c.setStatus(AfterSaleCaseStatus.REJECTED.value());
        c.setAppealState(AfterSaleAppealState.APPEALED.value());
        c.setRefundableAmount(1000L);
        c.setPaymentMethod(PaymentMethod.CREDIT_CARD.value());
        c.setBuyerId(2623900520L);
        c.setStoreId(2685914682L);
        return c;
    }

    @Test
    @DisplayName("第二次裁決（CAS affected=0）⇒ 拋例外且一張退款單都不建（併發 / 重送皆同）")
    void secondDecisionCreatesNoRefundTask() {
        when(caseMapper.selectOne(any())).thenReturn(appealedCase());
        // 併發的第一筆已把 appeal_state 推進 RESOLVED ⇒ 本次 CAS 命中 0 列
        when(caseMapper.markAppealResolved(eq(CASE_NO), anyString(), any())).thenReturn(0);

        assertThrows(AfterSalesBusinessException.class, () -> service.decide(
                ADMIN_ID, CASE_NO, AfterSalePlatformDecisionOutcome.FULL_REFUND.value(), null, "重複裁決"));

        verify(refundTaskMapper, never()).insert(any(RefundTaskEntity.class));
        verifyNoInteractions(platformDecisionMapper, resolutionMapper, appealEventMapper, eventPublisher);
    }

    @Test
    @DisplayName("case 已非 APPEALED（例如已裁決過）⇒ 前置守衛就擋下，不進 CAS 也不建退款")
    void nonAppealedCaseRejectedBeforeCas() {
        AfterSaleCaseEntity resolved = appealedCase();
        resolved.setAppealState(AfterSaleAppealState.RESOLVED.value());
        when(caseMapper.selectOne(any())).thenReturn(resolved);

        assertThrows(AfterSalesBusinessException.class, () -> service.decide(
                ADMIN_ID, CASE_NO, AfterSalePlatformDecisionOutcome.FULL_REFUND.value(), null, "重送"));

        verify(caseMapper, never()).markAppealResolved(anyString(), anyString(), any());
        verify(refundTaskMapper, never()).insert(any(RefundTaskEntity.class));
    }

    @Test
    @DisplayName("CAS 成功 ⇒ 才建退款單（證明前一個案例不是因為別的原因沒建）")
    void firstDecisionCreatesExactlyOneRefundTask() {
        when(caseMapper.selectOne(any())).thenReturn(appealedCase());
        when(caseMapper.markAppealResolved(eq(CASE_NO), anyString(), any())).thenReturn(1);

        service.decide(ADMIN_ID, CASE_NO, AfterSalePlatformDecisionOutcome.FULL_REFUND.value(), null, "全額退款");

        verify(refundTaskMapper).insert(any(RefundTaskEntity.class));
    }

    @Test
    @DisplayName("REJECT_APPEAL ⇒ 不建退款單（裁決駁回不得留下一張待執行的退款）")
    void rejectAppealCreatesNoRefundTask() {
        when(caseMapper.selectOne(any())).thenReturn(appealedCase());
        when(caseMapper.markAppealResolved(eq(CASE_NO), anyString(), any())).thenReturn(1);

        service.decide(ADMIN_ID, CASE_NO, AfterSalePlatformDecisionOutcome.REJECT_APPEAL.value(), null, "駁回");

        verify(refundTaskMapper, never()).insert(any(RefundTaskEntity.class));
    }
}
