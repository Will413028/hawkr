package io.github.will413028.hawkr.payment.infrastructure.mq;

import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.NullSource;
import org.junit.jupiter.params.provider.ValueSource;

import java.time.LocalDateTime;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * 36 §3.3.1 的 writer／factory 側 fail-fast（與 publisher 側的 fail-closed 是兩層，都要有）。
 *
 * <p><b>兩層的分工</b>：factory 擋「新寫入的壞列」——讓它連 insert 都到不了，
 * 失敗點就在寫入的那個交易裡、堆疊直接指向罪魁 caller；
 * publisher 擋「已經在表上的壞列」（corrupt／legacy／手寫 SQL 造出來的），
 * factory 對那些完全無能為力。
 *
 * <p><b>為什麼 exchange 是必填參數而不是 setter</b>：必填參數讓漏傳變成<b>編譯錯</b>。
 * 若做成 setter，漏呼叫就會靜默落 DB 的 {@code DEFAULT 'payment.events'}
 * ——而那個 DEFAULT 的存在理由是滾動更新相容（見 V11 檔頭），不是給新 code 當後路。
 */
class OutboxEventFactoryExchangeTest {

    private static final LocalDateTime NOW = LocalDateTime.of(2026, 9, 16, 12, 0, 0);

    @Test
    @DisplayName("合法 exchange → 原樣寫進 entity（不改寫、不正規化）")
    void validExchange_isPersistedVerbatim() {
        OutboxEventEntity e = OutboxEventFactory.newPendingEntity(
                "hawkr.shipping", "fulfillment.responsibility.finalized",
                "fulfillment.responsibility.finalized", "{}", NOW);

        assertEquals("hawkr.shipping", e.getExchange());
        assertEquals("fulfillment.responsibility.finalized", e.getEventType());
        assertEquals(OutboxEventStatus.PENDING.value(), e.getStatus());
        assertEquals(0, e.getRetryCount());
        assertEquals(NOW, e.getCreatedAt());
    }

    @Test
    @DisplayName("payment 事件走同一支 factory，exchange 一樣是 caller 顯式給的")
    void paymentExchange_isAlsoExplicit() {
        OutboxEventEntity e = OutboxEventFactory.newPendingEntity(
                "payment.events", "payment.success", "payment.success", "{}", NOW);
        assertEquals("payment.events", e.getExchange());
    }

    /**
     * 空白的四種形態。{@code "\t\n"} 這一組對應 V11 的 CHECK 從
     * {@code btrim(exchange) <> ''} 改成 {@code exchange ~ '\S'} 的理由
     * ——{@code btrim} 預設只剝空格，tab／換行會穿過去。
     * Java 的 {@code isBlank()} 以 {@code Character.isWhitespace} 判定，涵蓋全部，兩層語意一致。
     */
    @ParameterizedTest(name = "exchange={0} → IllegalArgumentException")
    @NullSource
    @ValueSource(strings = {"", " ", "   ", "\t", "\n", "\t\n", "\r\n  "})
    @DisplayName("exchange 為 null／空／任何 whitespace-only → fail-fast，entity 根本產不出來")
    void blankExchange_failsFast(String invalid) {
        IllegalArgumentException ex = assertThrows(IllegalArgumentException.class,
                () -> OutboxEventFactory.newPendingEntity(
                        invalid, "payment.success", "payment.success", "{}", NOW));

        assertTrue(ex.getMessage().contains("exchange"),
                "訊息要指得出是 exchange 的問題，實際值=" + ex.getMessage());
        assertTrue(ex.getMessage().contains("payment.success"),
                "訊息要帶得出是哪個事件，否則四個 caller 出錯時分不出是誰，實際值=" + ex.getMessage());
    }

    /**
     * 對照組：證明上一支不是「只要有參數就丟例外」。
     * 沒有這一支，把 factory 寫成無條件 throw 也會讓 blankExchange_failsFast 全綠。
     */
    @Test
    @DisplayName("對照組：exchange 只要有一個非空白字元就放行（fail-fast 不是恆擋）")
    void singleNonBlankCharacter_isAccepted() {
        assertEquals("x", OutboxEventFactory.newPendingEntity("x", "e", "r", "{}", NOW).getExchange());
        assertEquals(" x ", OutboxEventFactory.newPendingEntity(" x ", "e", "r", "{}", NOW).getExchange(),
                "前後空白不是 blank，且 factory 不得偷偷 trim（exchange 是對外契約值，改寫它會讓契約與列上的值不一致）");
    }
}
