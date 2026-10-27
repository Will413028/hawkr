package io.github.will413028.hawkr.webhook.uber;

import io.github.will413028.hawkr.webhook.adapter.http.UberWebhookController;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;

/**
 * 🔴 <b>ingress 的 HTTP 回應只有 200 與 5xx 兩種，永遠不得回 4xx。</b>
 *
 * <p>Uber 只對 {@code 500/502/503/504} 與 timeout 重試（API 總覽 §6.3）
 * ⇒ <b>回 4xx ＝ 該事件永久丟失</b>。這是一個「測起來很無聊、壞起來是靜默資料遺失」的契約，
 * 所以它需要自己的測試而不是靠 code review。
 *
 * <h2>變異對照</h2>
 * 把 {@code UberWebhookController} 的例外處理改成回 {@code 400}／加上
 * {@code consumes = APPLICATION_JSON_VALUE}（content-type 不符時回 415）／
 * 加 {@code @Valid}（驗證失敗回 400）—— 本類別任一測試都會轉紅。
 */
class UberWebhookResponseContractTest {

    private UberWebhookReceiveService receiveService;
    private MockMvc mockMvc;

    @BeforeEach
    void setUp() {
        receiveService = mock(UberWebhookReceiveService.class);
        mockMvc = MockMvcBuilders.standaloneSetup(new UberWebhookController(receiveService)).build();
    }

    @Test
    @DisplayName("正常落地 → 200")
    void landedReturns200() throws Exception {
        mockMvc.perform(post("/webhooks/uber/delivery")
                        .contentType(MediaType.APPLICATION_JSON)
                        .header("X-Uber-Signature", "sig")
                        .content("{\"kind\":\"event.delivery_status\",\"id\":\"evt_1\"}"))
                .andExpect(r -> assertEquals(200, r.getResponse().getStatus()));
    }

    @Test
    @DisplayName("🔴 落地失敗 → 5xx（不是 4xx）：Uber 才會重送")
    void landingFailureReturns5xx() throws Exception {
        doThrow(new RuntimeException("DB down")).when(receiveService).receive(any(), any());

        // standaloneSetup 沒有 servlet container 的錯誤轉譯層：例外會直接從 perform() 冒出來。
        // 那本身就是「沒有任何東西把它轉成回應」的證據 —— 真實部署中它會變成 500。
        // 兩種表現都算通過（例外冒出、或 status >= 500），但「被轉成 4xx」一定不算。
        try {
            MvcResult result = mockMvc.perform(post("/webhooks/uber/delivery")
                            .contentType(MediaType.APPLICATION_JSON)
                            .content("{\"kind\":\"event.delivery_status\",\"id\":\"evt_1\"}"))
                    .andReturn();
            int status = result.getResponse().getStatus();
            assertTrue(status >= 500,
                    "🔴 落地失敗卻回了 " + status + "：Uber 只重試 5xx／timeout，"
                            + "回 4xx ⇒ 該事件永久丟失（API 總覽 §6.3）");
        } catch (Exception propagated) {
            assertTrue(rootMessage(propagated).contains("DB down"),
                    "冒出來的必須是原始落地失敗，實得：" + propagated);
        }
    }

    @Test
    @DisplayName("🔴 content-type 不是 JSON → 仍然 200（限定 consumes 會回 415，那是 4xx）")
    void wrongContentTypeStillReturns200() throws Exception {
        mockMvc.perform(post("/webhooks/uber/delivery")
                        .contentType(MediaType.TEXT_PLAIN)
                        .content("not json at all"))
                .andExpect(r -> assertEquals(200, r.getResponse().getStatus(),
                        "限定 consumes 會讓 content-type 不符回 415 ⇒ 事件永久丟失"));
    }

    @Test
    @DisplayName("🔴 body 是垃圾 → 仍然 200（任何入參驗證的失敗模式都是 4xx）")
    void garbageBodyStillReturns200() throws Exception {
        mockMvc.perform(post("/webhooks/uber/delivery")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("<html>502 Bad Gateway</html>"))
                .andExpect(r -> assertEquals(200, r.getResponse().getStatus()));
    }

    @Test
    @DisplayName("🔴 body 為空 → 仍然 200（required=false，不得因缺 body 回 400）")
    void emptyBodyStillReturns200() throws Exception {
        mockMvc.perform(post("/webhooks/uber/delivery")
                        .contentType(MediaType.APPLICATION_JSON))
                .andExpect(r -> assertEquals(200, r.getResponse().getStatus()));
    }

    private static String rootMessage(Throwable t) {
        Throwable cur = t;
        StringBuilder sb = new StringBuilder();
        while (cur != null) {
            sb.append(cur.getMessage()).append(" | ");
            cur = cur.getCause();
        }
        return sb.toString();
    }

    @Test
    @DisplayName("controller 不得宣告任何 @ExceptionHandler —— 有的話 5xx 會被吃掉")
    void controllerDeclaresNoExceptionHandler() {
        for (var m : UberWebhookController.class.getDeclaredMethods()) {
            assertTrue(m.getAnnotation(org.springframework.web.bind.annotation.ExceptionHandler.class) == null,
                    "UberWebhookController." + m.getName() + " 宣告了 @ExceptionHandler："
                            + "把例外轉成回應等於讓 Uber 不再重送");
        }
    }
}
