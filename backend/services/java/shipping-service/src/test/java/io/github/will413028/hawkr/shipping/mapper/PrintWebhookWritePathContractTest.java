package io.github.will413028.hawkr.shipping.mapper;

import io.github.will413028.hawkr.shipping.service.ShippingService;
import org.apache.ibatis.annotations.Update;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.transaction.annotation.Transactional;

import java.lang.reflect.Method;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * Print webhook 這條 write path 的兩個保證——兩個都是<b>拿掉之後完全無聲</b>的那種。
 *
 * <h2>保證一：Print 回填不得復活終態</h2>
 * {@code markLabelPrintSucceededByWebhook} 的 {@code WHERE} 只守 {@code label_print_status}，
 * 而它的 {@code SET} 寫的是 {@code status}。一張「取消（41）之後才收到 Print 回調」的單子
 * 會被推回 {@code 920 已取得代碼}——正是 BLOCKER 3 要消滅的
 * 「較後階段被舊 action 推回較前階段」，而這條路徑<b>沒有</b>接 canonical rule。
 *
 * <h2>保證二：{@code @Transactional} 必須掛在 consumer 真正呼叫的那個多載</h2>
 * canonical ingress 落地後，consumer 呼叫的是 {@code processWebhookPrint(String, Map)}；
 * 它內部呼叫四參數多載屬 <b>self-invocation</b> ⇒ Spring proxy 不攔截 ⇒
 * 四參數上的 {@code @Transactional} <b>靜默失效</b>（CLAUDE.md §11.1 明列的陷阱）。
 * 症狀不是報錯，是三段碼與狀態的寫入不再原子。
 *
 * <h2>它不證明什麼（誠實標註）</h2>
 * 保證一是對 <b>SQL 字面</b>的斷言，不是對 PostgreSQL 執行結果的斷言——
 * shipping-service 沒有 DB 測試基礎設施。它擋得住「有人把終態守衛拿掉」，
 * 擋不住「CASE 的語意其實不是我以為的那樣」。
 */
class PrintWebhookWritePathContractTest {

    private static String printWebhookSql() throws NoSuchMethodException {
        return sqlOf("markLabelPrintSucceededByWebhook",
                Long.class, String.class, String.class, String.class, String.class);
    }

    /** 取一支 mapper 方法的 {@code @Update} SQL 字面；找不到目標一律 FAIL，不 skip。 */
    private static String sqlOf(String method, Class<?>... params) throws NoSuchMethodException {
        Method m = ShipmentMapper.class.getDeclaredMethod(method, params);
        Update ann = m.getAnnotation(Update.class);
        assertThat(ann)
                .as("找不到 " + method + " 的 @Update——方法被改名／改簽章／換成 XML mapper 了？"
                        + "本測試對『目標不存在』一律 FAIL，不 skip")
                .isNotNull();
        return String.join(" ", ann.value()).replaceAll("\\s+", " ");
    }

    @Test
    @DisplayName("Print 回填不得把終態（11 已取貨／41 已取消）推回 920")
    void printWebhookMustNotRevivePickedUpOrCancelled() throws Exception {
        String sql = printWebhookSql();

        assertThat(sql)
                .as("status 的 SET 必須帶終態守衛；沒有它，取消後才到的 Print 回調會復活單子")
                .contains("status = CASE WHEN status IN ('11', '41') THEN status ELSE");
        assertThat(sql)
                .as("status_entered_at 也要一起守——只守 status 會留下一個時間戳說謊的列")
                .contains("WHEN status IN ('11', '41') THEN status_entered_at");
    }

    @Test
    @DisplayName("三段碼仍要落庫：終態守衛不得寫成把整列跳過")
    void printWebhookStillPersistsSegmentCodesOnTerminalShipments() throws Exception {
        String sql = printWebhookSql();

        // 🔴 這一支是上一支的反面：正解是「狀態不動、證據照留」，
        //    而不是把終態排除在 WHERE 之外（那會連 provider 證據一起丟）。
        assertThat(sql)
                .as("WHERE 只該守 label_print_status；把 status 加進 WHERE 等於連三段碼一起不寫")
                .contains("WHERE id = #{id} AND label_print_status IN ('4102', '4101', '4104')");
        assertThat(sql).contains("partner_id = #{partnerId}");
        assertThat(sql).contains("odno = #{odno}");
        assertThat(sql).contains("validation_no = #{validationNo}");
    }

    /**
     * 🔴 <b>兩支姊妹 SQL 必須有同一道守衛</b>——只補一支比不補更危險。
     *
     * <p>本波先為 {@code markLabelPrintSucceededByWebhook} 補了終態守衛，而
     * {@code markTcatCallSucceeded} 在**同一個檔案裡**仍是無條件 {@code status = #&#123;status&#125;}
     * ⇒ QA 實測的 {@code 11（已取貨）→ 210（已叫件）} 倒退沒有關閉，
     * 只是被 Java 層的 {@code require()} 收窄成 TOCTOU 窗口：
     * {@code require} 用的是呼叫當下的 in-memory entity，而這一句在<b>打完 PayUni</b>
     * （事務外、跨網路、數秒）之後才執行。
     *
     * <p>「同一份設計裡兩個相鄰決策方向相反」本身就是訊號
     * （{@code .claude/rules/06-no-small-tails.md} §3.10）。
     */
    @Test
    @DisplayName("兩支寫 status 的 provider 回填 SQL 都要有終態守衛，不能只補一支")
    void bothProviderCallbackWritesGuardTerminalStates() throws Exception {
        String tcat = sqlOf("markTcatCallSucceeded",
                Long.class, java.time.LocalDateTime.class, String.class);
        assertThat(tcat)
                .as("叫件成功回填也會寫 status；WHERE 只守 tcat_call_status ⇒ 終態會被復活成 210")
                .contains("status = CASE WHEN status IN ('11', '41') THEN status ELSE");
        assertThat(tcat)
                .as("status_entered_at 也要一起守")
                .contains("WHEN status IN ('11', '41') THEN status_entered_at");
    }

    /**
     * 🔴 我方的狀態轉換不得抹掉 provider 說過的話。
     *
     * <p>{@code confirmCvsShipment} 走 {@code updateShipmentStatus} 時傳
     * {@code (null, null, null)}——它對 provider 沒有新資訊。原本的無條件覆寫
     * 會把三欄一起清空 ⇒ 本波剛開始保存的未知貨態 raw 值
     * （{@code persistUnknownProviderStatus}）會被下一次「賣家確認出貨」整組清掉，
     * 而 ADM 看到空欄會以為 provider 從沒回報過。**修一半比不修更容易造成誤信。**
     */
    @Test
    @DisplayName("狀態轉換不得抹掉 provider 證據（null ＝ 沒有新資訊，不是清空）")
    void statusTransitionMustNotEraseProviderEvidence() throws Exception {
        String sql = sqlOf("updateShipmentStatus",
                Long.class, String.class, String.class, String.class, String.class,
                java.time.LocalDateTime.class);
        assertThat(sql).contains("provider_status = COALESCE(#{providerStatus}, provider_status)");
        assertThat(sql).contains("provider_message = COALESCE(#{providerMessage}, provider_message)");
        assertThat(sql)
                .contains("provider_status_time = COALESCE(#{providerStatusTime}, provider_status_time)");
    }

    @Test
    @DisplayName("@Transactional 必須掛在 consumer 實際呼叫的那個多載（self-invocation 會讓它靜默失效）")
    void transactionalIsOnTheOverloadTheConsumerActuallyCalls() throws Exception {
        Method neutral = ShippingService.class.getDeclaredMethod(
                "processWebhookPrint", String.class, Map.class);
        assertThat(neutral.getAnnotation(Transactional.class))
                .as("consumer 呼叫的是這個中性多載；它沒有 @Transactional 的話，"
                        + "內部呼叫四參數多載屬 self-invocation ⇒ Spring proxy 不攔截 ⇒ 交易靜默消失")
                .isNotNull();
    }
}
