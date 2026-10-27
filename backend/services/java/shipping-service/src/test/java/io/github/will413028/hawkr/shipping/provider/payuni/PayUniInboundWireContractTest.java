package io.github.will413028.hawkr.shipping.provider.payuni;

import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.provider.FulfillmentContext;
import io.github.will413028.hawkr.shipping.provider.TestFulfillmentProviders;
import io.github.will413028.hawkr.shipping.provider.capability.InboundCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundPayload;
import io.github.will413028.hawkr.shipping.service.OnboardingService;
import io.github.will413028.hawkr.shipping.service.ShipmentStatusMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;

/**
 * PayUni 貨態 Notify 的 <b>wire 契約</b>：adapter 讀的欄名必須就是 provider 真的會送的欄名。
 *
 * <h2>這支測試要防的 bug class</h2>
 * {@code InboundCapability.parseStatusEvent} 原本讀 {@code "Message"} 與 {@code "StatusTime"}
 * ——<b>PayUni 沒有這兩個欄位</b>（實際是 {@code ShipStatusDesc} 與 {@code ShipStatusTime}）。
 * 在 {@code inbound()} 還是零呼叫端死碼的期間，這個錯誤沒有任何東西看得見：
 * 不會編譯錯、不會測試紅、也沒有任何測試拿真實形狀的 payload 餵過它。
 * 接上 consumer 的那一刻，症狀會是 {@code provider_message} 與貨態時間<b>靜默全空</b>。
 *
 * <h2>fixture 從哪裡來（刻意不手寫）</h2>
 * 欄名<b>解析自 repo 內的權威文件</b>
 * {@code docs/第三方整合/PayUni 物流官方 API 總覽.md} 的貨態欄位表。
 * 手寫一份「我覺得 PayUni 會送成這樣」的 fixture ＝ 自己跟自己對，必然全綠
 * ——那正是上面那個 bug 活下來的原因。
 * ⚠️ 文件搬走或欄位表改名 ⇒ 本測試 <b>fail-loud</b>，不是靜默 skip。
 */
class PayUniInboundWireContractTest {

    private static final Path API_DOC = repoRoot()
            .resolve("docs/第三方整合/PayUni 物流官方 API 總覽.md");

    @Test
    @DisplayName("貨態欄名取自官方文件：ShipStatusDesc／ShipStatusTime 必須真的被 adapter 讀到")
    void statusEventUsesTheFieldNamesPayUniActuallySends() throws IOException {
        Map<String, String> payload = payloadFromApiDoc();

        InboundCapability.InboundStatusEvent event = inbound().parseStatusEvent(fp(payload));

        assertThat(event.providerRawStatus())
                .as("raw 值一律保存，即使正規化失敗")
                .isEqualTo(payload.get("ShipStatus"));
        assertThat(event.providerMessage())
                .as("讀錯欄名時這裡會是 null——那正是修復前的行為")
                .isEqualTo(payload.get("ShipStatusDesc"));
        assertThat(event.providerStatusAt())
                .as("貨態時間讀錯欄名時會是 null，而 DB 上那一欄會靜默留空")
                .isNotNull();
    }

    @Test
    @DisplayName("未知貨態：raw 值保留、normalized 為 empty（fail-closed 的可查詢形式）")
    void unknownStatusIsReportedAsQueryableUnknown() throws IOException {
        Map<String, String> payload = payloadFromApiDoc();
        payload.put("ShipStatus", "9999");   // 值域外

        InboundCapability.InboundStatusEvent event = inbound().parseStatusEvent(fp(payload));

        assertThat(event.isKnown()).isFalse();
        assertThat(event.normalizedStatus()).isEmpty();
        assertThat(event.providerRawStatus()).isEqualTo("9999");
    }

    @Test
    @DisplayName("去重鍵：Print 與貨態必須不同，且 Print 以 Odno 區分（修復前兩者都塌成同一個鍵）")
    void dedupKeySeparatesPrintFromStatus() throws IOException {
        InboundCapability inbound = inbound();

        Map<String, String> status = payloadFromApiDoc();
        Map<String, String> print1 = new LinkedHashMap<>();
        print1.put("MerID", "HAWK1");
        print1.put("ShipTradeNo", "SC1");
        print1.put("ApiType", "Print");
        print1.put("Odno", "S001");
        Map<String, String> print2 = new LinkedHashMap<>(print1);
        print2.put("Odno", "S002");

        assertThat(inbound.dedupKey(fp(print1)))
                .as("Print 與貨態撞鍵 ⇒ 其中一則會被當成重複而丟棄")
                .isNotEqualTo(inbound.dedupKey(fp(status)));
        assertThat(inbound.dedupKey(fp(print1)))
                .as("修復前 Print 的鍵讀不存在的 ShipStatus ⇒ 兩則不同的 Print 產出同一個鍵")
                .isNotEqualTo(inbound.dedupKey(fp(print2)));
    }

    @Test
    @DisplayName("建單結果回報（只有 TradeStatus、沒有 ShipStatus）不得被當成未知貨態")
    void createResultEventIsNotAnUnknownStatus() {
        // 🔴 上游 EventKeyBuilder 的註解逐字寫著：
        //    「建單回傳無 ShipStatus（有 TradeStatus），貨態回傳有 ShipStatus」
        //    ⇒ 這是**另一種事件**，不是「看不懂的貨態」。
        //    2026-09-10 dev 實測：把它當成未知貨態，呼叫端會把 provider_status 寫成 null
        //    ——用「保存證據」的名義銷毀證據。
        Map<String, String> createResult = new LinkedHashMap<>();
        createResult.put("MerID", "HAWK2675237784");
        createResult.put("ShipTradeNo", "S0011789042138064014");
        createResult.put("MerTradeNo", "O940402546");
        createResult.put("TradeStatus", "0");
        createResult.put("TradeNo", "-");

        InboundCapability.InboundStatusEvent event = inbound().parseStatusEvent(fp(createResult));

        assertThat(event.isKnown()).isFalse();
        assertThat(event.providerRawStatus())
                .as("沒有 ShipStatus ⇒ raw 必須是 null，呼叫端才分辨得出「沒有東西可保存」")
                .isNull();
    }

    @Test
    @DisplayName("去重鍵對建單結果事件要退到 TradeStatus——與上游 EventKeyBuilder 對齊")
    void dedupKeyFallsBackToTradeStatusLikeUpstream() {
        Map<String, String> createResult = new LinkedHashMap<>();
        createResult.put("MerID", "HAWK2675237784");
        createResult.put("ShipTradeNo", "S0011789042138064014");
        createResult.put("TradeStatus", "0");

        // 上游實際算出的鍵（dev Loki 逐字）：HAWK2675237784:S0011789042138064014:STATUS:0
        assertThat(inbound().dedupKey(fp(createResult)))
                .isEqualTo("HAWK2675237784:S0011789042138064014:STATUS:0");
    }

    // ==================== helpers ====================

    /** PayUni 是 form-encoded provider ⇒ 只有 formParams，rawBody 為 null（ADR-012）。 */
    private static InboundPayload fp(Map<String, String> formParams) {
        return InboundPayload.ofFormParams(formParams);
    }

    private static InboundCapability inbound() {
        return TestFulfillmentProviders
                .realRegistry(mock(ShippingClient.class), new ShipmentStatusMapper(),
                        mock(OnboardingService.class))
                .require(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value())
                .inbound(ctx())
                .orElseThrow(() -> new AssertionError("PayUni 超商 adapter 沒有 inbound capability"));
    }

    /**
     * 從官方文件的貨態欄位表取欄名，組出一份真實形狀的 payload。
     * 文件不存在或表格裡找不到必要欄名 ⇒ 直接失敗（fail-closed，不 skip）。
     */
    private static Map<String, String> payloadFromApiDoc() throws IOException {
        if (!Files.isRegularFile(API_DOC)) {
            throw new AssertionError("找不到 PayUni API 文件：" + API_DOC
                    + "——本測試的 fixture 來源消失了，不能當成通過");
        }
        String doc = Files.readString(API_DOC, StandardCharsets.UTF_8);
        Map<String, String> sample = new LinkedHashMap<>();
        sample.put("MerID", "HAWK2675237784");
        sample.put("ShipTradeNo", "S0011789021584194041");
        sample.put(requireField(doc, "ShipStatus"), "300");
        sample.put(requireField(doc, "ShipStatusDesc"), "配送中");
        sample.put(requireField(doc, "ShipStatusTime"), "2026-09-10 14:32:41");
        return sample;
    }

    /** 欄名必須真的出現在文件的欄位表列中（`| 欄名 | 型別 | …`），否則 fail-loud。 */
    private static String requireField(String doc, String field) {
        Matcher m = Pattern.compile("(?m)^\\|\\s*" + Pattern.quote(field) + "\\s*\\|").matcher(doc);
        if (!m.find()) {
            throw new AssertionError("官方文件的欄位表裡找不到 " + field
                    + "——欄名可能已改，adapter 讀的名字要跟著查");
        }
        return field;
    }

    private static FulfillmentContext ctx() {
        ShipmentEntity e = new ShipmentEntity();
        e.setId(1L);
        e.setOrderNo("O-WIRE-0001");
        e.setShipmentNo("S-WIRE-0001");
        e.setStoreId(2675237784L);
        e.setShippingMethod(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());
        e.setStatus(ShippingStatus.UNPROCESSED.value());
        return FulfillmentContext.forShipment(e);
    }

    /** 由 module 目錄往上找到 repo root（存在 `.git` 的那一層）。 */
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
