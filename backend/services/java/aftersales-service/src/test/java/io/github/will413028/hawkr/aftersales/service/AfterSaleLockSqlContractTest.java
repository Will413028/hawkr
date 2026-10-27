package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleAppealEventMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSalePlatformDecisionMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSalePayoutHoldService.PayoutHold;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAppealState;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import org.apache.ibatis.annotations.Select;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.invocation.Invocation;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.lang.reflect.Method;
import java.time.LocalDateTime;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.clearInvocations;
import static org.mockito.Mockito.mockingDetails;
import static org.mockito.Mockito.when;

/**
 * backend#3290：backend#3262 兩道持鎖保證的 <b>CI 可見</b> 結構性守衛。
 *
 * <p><b>為什麼要有本檔</b>：#3262 的順序保證由兩句 SQL 承載——{@code submitAppeal} 的
 * {@code FOR UPDATE} 與撥款閘門探測的 {@code FOR UPDATE SKIP LOCKED}。把任一句拿掉，
 * P0 立刻復活，而<b>當時只有需要真 PostgreSQL 的 Testcontainers IT 抓得到</b>
 * （獨立 QA 的 M5 / M8 變異：正常 CI 全綠）。CI 預設不跑 Docker ⇒ 那道保護在 PR 上等於不存在
 * （同 {@code 10-engineering-contracts.md} §17.5）。本檔補上不依賴 Docker 的那一半。
 *
 * <p><b>為什麼不是「grep 全 repo 有沒有 FOR UPDATE」</b>：那種寫法對「SQL 還在、但生產路徑
 * 已經改呼叫別的方法」完全失明。本檔改成<b>跑真正的生產方法</b>（{@code submitAppeal} /
 * {@code getPayoutHold}），從 Mockito 的 invocation 記錄取出<b>它實際呼叫的那一支 mapper 方法</b>，
 * 再讀那一支的 {@code @Select} 字面值。SQL 與呼叫點兩者只要有一邊被換掉，本檔就轉紅。
 *
 * <p>真 PostgreSQL 的行為層驗證（鎖真的鎖得住、SKIP LOCKED 真的不等待）仍由
 * {@code AfterSalePayoutHoldLockProtocolIT} 承擔，本檔不取代它。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class AfterSaleLockSqlContractTest {

    private static final Long BUYER_ID = 1001L;
    private static final String CASE_NO = "AS-3290";
    private static final String ORDER_NO = "O20260909-3290";
    private static final int DEADLINE_HOURS = 72;
    private static final int GRACE_MINUTES = 15;

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private AfterSaleCaseService caseService;
    @Mock private AfterSaleAppealEventMapper appealEventMapper;
    @Mock private AfterSalePlatformDecisionMapper platformDecisionMapper;
    @Mock private AfterSaleResolutionMapper resolutionMapper;
    @Mock private EvidenceService evidenceService;
    @Mock private AfterSalesEventPublisher eventPublisher;

    // ==================== 共用：從生產路徑取出「它實際呼叫的那一支 mapper 方法」 ====================

    /**
     * 取本次執行中，{@code caseMapper} 上第一個<b>由 {@link AfterSaleCaseMapper} 自己宣告</b>的
     * 被呼叫方法。
     *
     * <p>刻意排除繼承自 MyBatis-Plus {@code BaseMapper} 的方法（{@code selectList} 等）：
     * 那些是非持鎖的一般查詢，不可能承載 {@code FOR UPDATE}。若生產路徑改成只用 wrapper 查詢，
     * 這裡會找不到候選而轉紅——那正是我們要的（wrapper 查不出持鎖讀）。
     */
    private Method firstDeclaredMapperCall(String what) {
        List<Invocation> invocations = List.copyOf(mockingDetails(caseMapper).getInvocations());
        return invocations.stream()
                .map(Invocation::getMethod)
                .filter(m -> m.getDeclaringClass() == AfterSaleCaseMapper.class)
                .findFirst()
                .orElseThrow(() -> new AssertionError(
                        what + " 沒有呼叫任何 AfterSaleCaseMapper 自宣告的方法 ⇒ 持鎖讀已不在生產路徑上"));
    }

    /** 讀該 mapper 方法的 {@code @Select} 字面值（壓平空白）。沒有 {@code @Select} 即為不合格。 */
    private static String selectSqlOf(Method method, String what) {
        Select select = method.getAnnotation(Select.class);
        assertThat(select)
                .as("%s 實際呼叫的是 %s()，而它沒有 @Select 註解 ⇒ 不是本檔要守的持鎖讀",
                        what, method.getName())
                .isNotNull();
        return String.join(" ", select.value()).replaceAll("\\s+", " ").trim();
    }

    // ==================== Guard M5：submitAppeal 的持鎖讀 ====================

    @Test
    @DisplayName("M5 guard：submitAppeal 實際使用的鎖查詢必須帶 FOR UPDATE（拿掉＝#3262 P0 復活）")
    void submitAppealLockQueryUsesForUpdate() {
        when(caseService.getCaseForBuyer(eq(BUYER_ID), eq(CASE_NO)))
                .thenReturn(appealableCase(DateTimeUtils.nowUtc().minusHours(1)));
        when(caseMapper.selectByCaseNoForUpdate(CASE_NO))
                .thenReturn(appealableCase(DateTimeUtils.nowUtc().minusHours(1)));
        when(caseMapper.markAppealed(eq(CASE_NO), any(), anyString(), anyString())).thenReturn(1);

        AfterSaleAppealService service = new AfterSaleAppealService(caseMapper, caseService,
                appealEventMapper, platformDecisionMapper, resolutionMapper, evidenceService,
                eventPublisher, new AfterSalesProperties());

        clearInvocations(caseMapper);
        service.submitAppeal(BUYER_ID, CASE_NO, "R", "D", List.of());

        Method lockRead = firstDeclaredMapperCall("submitAppeal");
        String sql = selectSqlOf(lockRead, "submitAppeal");
        assertThat(sql)
                .as("""
                        submitAppeal 的第一個 mapper 動作必須是持鎖讀：資格判定（A1/A2/A3）通過之後，\
                        交易還要跑完 media gRPC、evidence insert、CAS、event insert、MQ publish 才 commit，\
                        那段期間該申訴對撥款閘門不可見（MVCC）。沒有 FOR UPDATE，閘門的 SKIP LOCKED 探測\
                        就探不到任何東西 ⇒ 可能在「申訴其實已取得資格、只是還沒 commit」的當下放行撥款，\
                        而全 repo 沒有賣家扣回機制 ⇒ 平台資損。獨立 QA 的 M5 變異就是拿掉它\
                        ——當時只有 Testcontainers IT 轉紅，本斷言補上 CI 可見的那一半。""")
                .containsIgnoringCase("FOR UPDATE");

        assertThat(sql)
                .as("""
                        這一把鎖必須會「等」，不得是 SKIP LOCKED：申訴側要的是互斥，探測側才要非阻塞。\
                        改成 SKIP LOCKED 的話，該列被壓住時本查詢回 null ⇒ submitAppeal 把它讀成\
                        AFTERSALE_NOT_FOUND，買家的申訴被平台自己的併發打掉（方向固定為「平台多等一下」，\
                        不是「買家少一點權利」），而且順序保證所需的互斥根本沒有建立。""")
                .doesNotContainIgnoringCase("SKIP LOCKED");
    }

    // ==================== Guard M8：撥款閘門的在途申訴探測 ====================

    @Test
    @DisplayName("M8 guard：撥款放行前的探測必須帶 FOR UPDATE SKIP LOCKED（缺 SKIP LOCKED ＝ 探測會等待）")
    void payoutInFlightProbeUsesForUpdateSkipLocked() {
        AfterSalesProperties props = new AfterSalesProperties();
        props.getAppeal().setDeadlineHours(DEADLINE_HOURS);
        props.getAppeal().setPayoutHoldGraceMinutes(GRACE_MINUTES);
        AfterSalePayoutHoldService service = new AfterSalePayoutHoldService(caseMapper, props);

        // 申訴窗（含 grace）已過的 case ⇒ 走到本 service 唯一的「放行」出口，探測必被執行
        AfterSaleCaseEntity windowClosed = new AfterSaleCaseEntity();
        windowClosed.setCaseNo(CASE_NO);
        windowClosed.setOrderNo(ORDER_NO);
        windowClosed.setStatus(AfterSaleCaseStatus.REJECTED.value());
        windowClosed.setAppealState(AfterSaleAppealState.NONE.value());
        windowClosed.setLastRejectedAt(DateTimeUtils.nowUtc().minusHours(DEADLINE_HOURS + 1));

        when(caseMapper.selectList(any())).thenReturn(List.of(windowClosed));
        when(caseMapper.tryLockCaseForPayoutRelease(CASE_NO)).thenReturn(windowClosed);

        clearInvocations(caseMapper);
        PayoutHold hold = service.getPayoutHold(ORDER_NO);

        assertThat(hold.hold())
                .as("前置條件：本 case 必須真的走到放行出口，否則下面驗到的不是那支探測")
                .isFalse();

        Method probe = firstDeclaredMapperCall("撥款閘門放行前的探測");
        String sql = selectSqlOf(probe, "撥款閘門放行前的探測");

        assertThat(sql)
                .as("""
                        探測必須是持鎖讀：它要回答的是「這一列現在有沒有被別的交易壓著」，\
                        而唯一會壓著它的就是 submitAppeal 的 FOR UPDATE。非持鎖查詢永遠探得到 \
                        ⇒ 閘門形同不存在。""")
                .containsIgnoringCase("FOR UPDATE");
        assertThat(sql)
                .as("""
                        必須是 SKIP LOCKED 而非等待或 NOWAIT：本查詢跑在 GrantPaymentWorker 的租約窗口內\
                        （經 5 秒 gRPC deadline）。沒有 SKIP LOCKED，探測會阻塞等待持鎖者——\
                        把「申訴交易的耗時」灌進租約下界的算式（《多副本排程搶佔規範》§三第 5 步），\
                        而那個耗時正是 #3262 已經證明沒有上界的東西 ⇒ 租約不等式破裂、任務被別的 pod 搶走。\
                        獨立 QA 的 M8 變異就是拿掉它——當時只有 Testcontainers IT 轉紅。""")
                .containsIgnoringCase("SKIP LOCKED");
    }

    private AfterSaleCaseEntity appealableCase(LocalDateTime lastRejectedAt) {
        AfterSaleCaseEntity c = new AfterSaleCaseEntity();
        c.setCaseNo(CASE_NO);
        c.setOrderNo(ORDER_NO);
        c.setBuyerId(BUYER_ID);
        c.setStoreId(2001L);
        c.setStatus(AfterSaleCaseStatus.REJECTED.value());
        c.setAppealState(AfterSaleAppealState.NONE.value());
        c.setLastRejectedAt(lastRejectedAt);
        return c;
    }
}
