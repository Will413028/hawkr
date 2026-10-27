package io.github.will413028.hawkr.payment.agentfee;

import io.github.will413028.hawkr.payment.config.PayUniConfig;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.nio.file.Files;
import java.nio.file.Path;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 🔴 送出前 gross 必須已知（架構裁決 DEC-20260912-184025 第 7 條）。
 *
 * <h2>這份測試釘住的不變式</h2>
 * {@code agent_fee_query} 的查詢鍵是 <b>{@code (MerID, TradeNo, Amt)} 三元組</b>，
 * 而 {@code Amt} 是 gross 不是 target net。存在一個具體的失效窗口：
 * <b>送出成功、回應遺失</b>——PayUni 可能已受理，我方卻不知道自己送了多少
 * ⇒ 後續 query 帶不出正確的 {@code Amt} ⇒ 查無此筆 ⇒ 一筆已經在對方系統裡的收款
 * 在我方永遠停在不確定狀態。
 *
 * <p>⇒ 裁決逐字：「不得靠 response 才取得查詢鍵」。
 * 本檔用<b>行為</b>證明這件事成立，而不是靠註解宣稱。
 */
@DisplayName("agent_fee：送出前 gross 必須已知")
class AgentFeeSubmitPreCommitTest {

    private static final String MER_ID = "HAWK123";
    private static final String TRADE_NO = "LGS20260912001";

    private AgentFeeService service(PaymentClient client, AgentFeeGrossUp grossUp) {
        PayUniConfig cfg = mock(PayUniConfig.class);
        when(cfg.getAgentId()).thenReturn("AGENT01");
        return new AgentFeeService(client, cfg, grossUp, "AGENTMER");
    }

    @Test
    @DisplayName("🔴 呼叫端可以先拿到 gross，再用它送出——這是 pre-commit 成立的前提")
    void callerCanObtainGrossBeforeSubmitting() {
        AgentFeeGrossUp grossUp = mock(AgentFeeGrossUp.class);
        AgentFeeGrossUp.Result solved = new AgentFeeGrossUp.Result(1012L, 12L, "v1");
        when(grossUp.solve(1000L)).thenReturn(solved);

        PaymentClient client = mock(PaymentClient.class);
        when(client.agentFee(anyString(), anyString(), anyString(), anyLong(), anyInt(), anyInt(), anyString()))
                .thenThrow(new RuntimeException("connection reset"));   // 送出成功但回應遺失

        AgentFeeService svc = service(client, grossUp);

        // 呼叫端的正確順序：先 solve（拿到 gross）→ 此時就可以落地 → 才送出
        AgentFeeGrossUp.Result preCommitted = grossUp.solve(1000L);
        assertThat(preCommitted.grossAmount())
                .as("gross 在任何 HTTP 呼叫發生之前就已經是已知的")
                .isEqualTo(1012L);
        verify(client, never())
                .agentFee(anyString(), anyString(), anyString(), anyLong(), anyInt(), anyInt(), anyString());

        AgentFeeService.SubmitOutcome outcome = svc.submit(MER_ID, TRADE_NO, preCommitted, 4, 1);

        assertThat(outcome.providerResponded())
                .as("逾時／連線失敗不是失敗——PayUni 可能已受理")
                .isFalse();
        assertThat(outcome.grossAmount())
                .as("🔴 即使回應遺失，我方仍然知道送出的 Amt 是多少——否則 query 查不到那一筆")
                .isEqualTo(1012L);
    }

    @Test
    @DisplayName("🔴 回應遺失時，query 的三個鍵全部拿得到（MerID／TradeNo／Amt）")
    void queryKeysSurviveLostResponse() {
        AgentFeeGrossUp grossUp = mock(AgentFeeGrossUp.class);
        when(grossUp.solve(anyLong())).thenReturn(new AgentFeeGrossUp.Result(2024L, 24L, "v1"));

        PaymentClient client = mock(PaymentClient.class);
        when(client.agentFee(anyString(), anyString(), anyString(), anyLong(), anyInt(), anyInt(), anyString()))
                .thenThrow(new RuntimeException("read timeout"));

        AgentFeeService.SubmitOutcome outcome =
                service(client, grossUp).submit(MER_ID, TRADE_NO, grossUp.solve(2000L), 4, 1);

        // 三個鍵：MerID 與 TradeNo 是呼叫端自己給的，Amt 來自 outcome。
        // 🔴 三者缺一，agent_fee_query 就查不到那一筆。
        assertThat(MER_ID).isNotBlank();
        assertThat(TRADE_NO).isNotBlank();
        assertThat(outcome.grossAmount()).isEqualTo(2024L);
    }

    @Test
    @DisplayName("🔴 送出用的金額是 gross 不是 target net（送錯的話對方收到的金額就是錯的）")
    void submittedAmountIsGrossNotTargetNet() {
        AgentFeeGrossUp grossUp = mock(AgentFeeGrossUp.class);
        AgentFeeGrossUp.Result solved = new AgentFeeGrossUp.Result(1012L, 12L, "v1");

        PaymentClient client = mock(PaymentClient.class);
        when(client.agentFee(anyString(), anyString(), anyString(), anyLong(), anyInt(), anyInt(), anyString()))
                .thenReturn(null);

        service(client, grossUp).submit(MER_ID, TRADE_NO, solved, 4, 1);

        // 1012 是 gross；1000 是 target net。送出的必須是前者。
        verify(client).agentFee("AGENT01", MER_ID, TRADE_NO, 1012L, 4, 1, "AGENTMER");
        verify(client, never()).agentFee("AGENT01", MER_ID, TRADE_NO, 1000L, 4, 1, "AGENTMER");
    }

    @Test
    @DisplayName("🔴 結構斷言：舊多載必須委派給新多載，不得各自 solve 一次")
    void legacyOverloadDelegates() throws Exception {
        // 兩個多載各自 solve 的話，兩次之間沒有任何保證相等——目前會相等只是因為
        // AgentFeeRatePolicy 是 @ConfigurationProperties 啟動後不變，那是巧合不是保證。
        Path src = repoRoot().resolve(
                "services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/agentfee/AgentFeeService.java");
        if (!Files.isRegularFile(src)) {
            throw new AssertionError("找不到被測原始碼：" + src + "——檔案搬家了，本斷言的錨要跟著改");
        }
        String code = Files.readString(src);
        int start = code.indexOf("public SubmitOutcome submit(String merId, String tradeNo, long targetNet");
        assertThat(start).as("舊多載的簽章改了 ⇒ 本斷言的錨失效").isGreaterThan(0);
        int end = code.indexOf("public SubmitOutcome submit(String merId, String tradeNo, AgentFeeGrossUp.Result");
        assertThat(end).as("新多載不見了").isGreaterThan(start);

        String legacyBody = code.substring(start, end);
        assertThat(legacyBody)
                .as("舊多載必須委派，不得自己組 PayUni 呼叫")
                .contains("return submit(merId, tradeNo, grossUp.solve(targetNet), feeType, balType);")
                .doesNotContain("paymentClient.agentFee");
    }

    private static Path repoRoot() {
        Path p = Path.of("").toAbsolutePath();
        // backend 根目錄以 go.work 為錨：monorepo 裡 .git 在上一層，不是 backend/。
        while (p != null && !Files.exists(p.resolve("go.work"))) {
            p = p.getParent();
        }
        if (p == null) {
            throw new AssertionError("找不到 backend 根目錄（沒有 go.work）");
        }
        return p;
    }
}
