package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;

import java.time.Duration;
import java.util.Optional;
import java.util.UUID;

/**
 * 付款中轉頁 canonical 表單暫存（opaque redirect_id）。
 * <p>
 * 付款建立後 WebView 導向 {@code /payment/redirect?id={id}}；Gateway 以 id 透過 gRPC 取回
 * 表單欄位並以 html/template 受控 render。表單以不可猜 id 存 Redis、短效、一次性取用，
 * 避免 client 可 round-trip 的資料進入 render 造成反射型 XSS（#1394）。
 */
@Component
public class PaymentRedirectFormStore {

    private static final ObsLogger log = ObsLogger.forClass(PaymentRedirectFormStore.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    /** Redis key 前綴（集中常數，禁散落硬編碼） */
    static final String KEY_PREFIX = "payment:redirect:";
    /** 一次性中轉表單存活時間，對齊 PayUni MerTradeNo 10 分鐘窗 */
    static final Duration TTL = Duration.ofMinutes(10);

    private final StringRedisTemplate redisTemplate;

    public PaymentRedirectFormStore(StringRedisTemplate redisTemplate) {
        this.redisTemplate = redisTemplate;
    }

    /**
     * 存入 canonical 表單，回傳不可猜的 opaque id。
     */
    public String store(RedirectForm form) throws JsonProcessingException {
        String id = UUID.randomUUID().toString();
        String json = MAPPER.writeValueAsString(form);
        redisTemplate.opsForValue().set(KEY_PREFIX + id, json, TTL);
        return id;
    }

    /**
     * 依 id 原子取出並刪除（GETDEL，一次性 consume）。查無 / 過期 / 已用 → empty。
     */
    public Optional<RedirectForm> consume(String id) {
        if (id == null || id.isEmpty()) {
            return Optional.empty();
        }
        String json = redisTemplate.opsForValue().getAndDelete(KEY_PREFIX + id);
        if (json == null) {
            return Optional.empty();
        }
        try {
            return Optional.of(MAPPER.readValue(json, RedirectForm.class));
        } catch (JsonProcessingException e) {
            log.error("PaymentRedirectFormStore.consume.error", "反序列化失敗: " + e.getMessage(), e);
            return Optional.empty();
        }
    }

    /**
     * canonical 付款中轉表單欄位（camelCase，對齊 {@code UppFormData}）。
     */
    public record RedirectForm(
            String actionUrl,
            String merId,
            String version,
            String encryptInfo,
            String hashInfo,
            String isPlatform
    ) {}
}
