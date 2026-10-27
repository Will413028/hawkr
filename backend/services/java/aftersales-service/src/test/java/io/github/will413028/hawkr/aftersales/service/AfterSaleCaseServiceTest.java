package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseItemMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleEvidenceMapper;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.when;

/**
 * Plan C Phase 3：{@link AfterSaleCaseService#getCaseForSeller} 的 account_id
 * vs store_id 判別測試 — 這支方法在 rename 前後方法本體邏輯完全不變（一律比對
 * 「傳入的 id」against「entity 的身分欄位」），真正的 bug 出在呼叫端過去傳入的
 * 是未解析的登入 account_id；本測試釘住「呼叫端必須傳入已解析的 store_id」這個
 * 契約——account_id != store_id 時必須被拒絕，而非巧合放行。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class AfterSaleCaseServiceTest {

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private AfterSaleCaseItemMapper caseItemMapper;
    @Mock private AfterSaleEvidenceMapper evidenceMapper;
    @Mock private OrderGrpcClient orderGrpcClient;
    @Mock private EvidenceService evidenceService;
    @Mock private AfterSaleAuditService auditService;
    @Mock private AfterSalesProperties properties;
    @Mock private AfterSalesEventPublisher eventPublisher;

    @InjectMocks private AfterSaleCaseService service;

    private static final long CURRENT_STORE_ID = 7001L;
    private static final long STALE_LOGIN_ACCOUNT_ID = 42L;

    private AfterSaleCaseEntity caseWithStoreId(long storeId) {
        AfterSaleCaseEntity e = new AfterSaleCaseEntity();
        e.setCaseNo("A001");
        e.setStoreId(storeId);
        return e;
    }

    @BeforeEach
    void setup() {
        when(caseMapper.selectOne(any())).thenReturn(caseWithStoreId(CURRENT_STORE_ID));
    }

    @Test
    void getCaseForSeller_matchingStoreId_returnsCase() {
        AfterSaleCaseEntity result = service.getCaseForSeller(CURRENT_STORE_ID, "A001");
        assertEquals("A001", result.getCaseNo());
    }

    @Test
    void getCaseForSeller_staleAccountIdDiffersFromStoreId_throwsNotOwner() {
        AfterSalesBusinessException ex = assertThrows(
                AfterSalesBusinessException.class,
                () -> service.getCaseForSeller(STALE_LOGIN_ACCOUNT_ID, "A001"));
        assertEquals(GrpcResponseCode.AFTERSALE_NOT_OWNER.code(), ex.code());
    }
}
