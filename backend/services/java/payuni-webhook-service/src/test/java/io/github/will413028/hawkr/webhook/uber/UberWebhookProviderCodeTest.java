package io.github.will413028.hawkr.webhook.uber;

import io.github.will413028.hawkr.common.utils.code.WebhookProvider;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * {@link UberWebhookProviderCode} 與 CodeMeta 的綁定。
 *
 * <p>這支測試守的是一個<b>靜默</b>的失效：{@code WebhookProvider.UBER} 被拿掉或改號時，
 * 症狀不是編譯錯（本類別用 {@code fromCode(String)} 查表），而是每一筆 Uber webhook 回 5xx，
 * 或（更糟）寫進一個 consumer 比不出來的值。
 */
class UberWebhookProviderCodeTest {

    @Test
    @DisplayName("CodeMeta 有 UBER，且 value 是 8242（webhook_inbox.provider 存的是 Value 不是 Name）")
    void codeMetaHasUber() {
        assertTrue(UberWebhookProviderCode.isAvailable(),
                "WebhookProvider CodeMeta 少了 UBER ⇒ Uber webhook 全部寫不進 webhook_inbox");
        // 🔴 期望值硬寫字面值：引用 WebhookProvider.UBER.value() 會讓改號時測試跟著改，等於沒驗
        assertEquals("8242", new UberWebhookProviderCode().value());
        assertEquals("UBER", UberWebhookProviderCode.CODE);
    }

    @Test
    @DisplayName("寫進 DB 的是 Value 不是 code name（66 §6.3.3：寫 'UBER' 會讓 consumer 拿字串比數字碼）")
    void valueIsNotTheCodeName() {
        String written = new UberWebhookProviderCode().value();
        assertEquals(WebhookProvider.UBER.value(), written);
        assertTrue(!written.equals(WebhookProvider.UBER.code()),
                "provider 欄位不得存 code name");
    }
}
