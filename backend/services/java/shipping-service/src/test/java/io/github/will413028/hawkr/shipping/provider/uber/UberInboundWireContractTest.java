package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.common.webhook.UberWebhookVerifier;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.provider.FulfillmentContext;
import io.github.will413028.hawkr.shipping.provider.capability.InboundCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundPayload;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * Uber DaaS webhook 的 <b>wire 契約</b>：adapter 認得的狀態值域必須就是官方的值域。
 *
 * <h2>fixture 從哪裡來（刻意不手寫）</h2>
 * 狀態值域<b>解析自 repo 內的權威文件</b>
 * {@code docs/第三方整合/Uber Direct API 總覽.md} §7.1 的 DaaS 狀態表。
 * 手寫一份「我覺得 Uber 會回這幾個值」的清單 ＝ 自己跟自己對，必然全綠。
 * ⚠️ 文件搬走或表格改名 ⇒ 本測試 <b>fail-loud</b>，不是靜默 skip。
 *
 * <h2>這支測試要防的 bug class</h2>
 * <ol>
 *   <li>🔴 <b>抄錯家族</b>：官方文件站有兩組 API 家族、各自一套 status，名字完全不同。
 *       總覽 §7 自己就記著撰寫當時一度抓成 Eats deliveries 那一組
 *       （{@code SCHEDULED}／{@code EN_ROUTE_TO_PICKUP}…）並據此判定 66 的映射表「對不上官方」。
 *       映射表若寫成那一組，<b>每一則 webhook 都會落進未知值分支</b>，而症狀是
 *       「Uber 從來沒有推進過狀態」——看起來像 Uber 沒發 webhook</li>
 *   <li>🔴 <b>把 {@code courier_imminent} 當第 8 個狀態</b>（U-I）：會同時造成兩個錯——
 *       多一個永不出現的狀態，且漏讀「即將抵達」，而那正是買家最需要的推播</li>
 *   <li>🔴 <b>把正向的 {@code returned} 讀成「已退貨完成」</b>：它在正向 leg 是<b>失敗</b>
 *       （→33→41→退款給買家），在逆向 leg 才是「退貨包裹送不回賣家」（→33→{@code RETURN_FAILED}）。
 *       66 §7.3.3 逐字：<b>兩者的錢往完全相反的方向走</b></li>
 * </ol>
 */
@DisplayName("Uber inbound wire 契約")
class UberInboundWireContractTest {

    private static final Path API_DOC = repoRoot()
            .resolve("docs/第三方整合/Uber Direct API 總覽.md");

    /** 66 §7.3.1：Pick &amp; Pack 專屬，我方非代購情境不訂閱 ⇒ 容忍值，不是我們要映射的狀態。 */
    private static final String TOLERATED = "shopping_completed";

    // ==================== ① 值域 ====================

    @Test
    @DisplayName("① DaaS 狀態值域（官方 8 值 − 容忍值 1）＝ 映射表的 7 個 key")
    void mappedStatusesAreExactlyTheOfficialDaasValues() throws IOException {
        Set<String> official = daasStatusesFromApiDoc();

        assertThat(official)
                .as("官方 §7.1 表共 8 列（7 值 ＋ Pick & Pack 專屬的容忍值）")
                .hasSize(8)
                .contains(TOLERATED);

        Set<String> expectedMapped = new LinkedHashSet<>(official);
        expectedMapped.remove(TOLERATED);

        assertThat(UberStatusMapper.knownRawStatuses())
                .as("🔴 少一個 ⇒ 那個狀態的每一則 webhook 都落進未知值分支；"
                        + "多一個 ⇒ 我們在猜一個官方不會送的值")
                .containsExactlyInAnyOrderElementsOf(expectedMapped);
    }

    @Test
    @DisplayName("① 容忍值：shopping_completed 不映射任何狀態，但也不是未知值（不建 case）")
    void toleratedValueIsNeitherMappedNorUnknown() {
        assertThat(UberStatusMapper.isTolerated(TOLERATED))
                .as("它是我方沒訂閱的功能的正常值——建 case 只會產生永遠關不完的噪音")
                .isTrue();
        assertThat(UberStatusMapper.normalize(TOLERATED)).isEmpty();
        assertThat(UberStatusMapper.isTolerated("teleported"))
                .as("真未知值必須與容忍值分得開：前者要 counter ＋ 建 case")
                .isFalse();
    }

    // ==================== ② 未知值 fail-closed ====================

    @Test
    @DisplayName("② 未知 raw status ⇒ normalized empty ＋ raw 照存（fail-closed，不得靜默忽略）")
    void unknownStatusIsFailClosedWithRawPreserved() {
        InboundCapability.InboundStatusEvent event = parse(
                "{\"id\":\"evt_2\",\"kind\":\"event.delivery_status\",\"created\":1789000001,"
                        + "\"delivery_id\":\"del_2\",\"status\":\"teleported\"}");

        assertThat(event.isKnown()).isFalse();
        assertThat(event.normalizedStatus()).isEmpty();
        assertThat(event.providerRawStatus())
                .as("🔴 raw 必須保存：靜默忽略會讓「Uber 新增了一個狀態」在系統裡完全沒有痕跡")
                .isEqualTo("teleported");
    }

    @Test
    @DisplayName("② 已知值走正常路徑（陽性對照：沒有它，把 normalize 寫成恆 empty 也會綠）")
    void knownStatusIsNormalized() {
        InboundCapability.InboundStatusEvent event = parse(
                "{\"id\":\"evt_1\",\"kind\":\"event.delivery_status\",\"created\":1789000000,"
                        + "\"delivery_id\":\"del_1\",\"status\":\"delivered\"}");

        assertThat(event.isKnown()).isTrue();
        // 🔴 硬寫 "11"：期望值跟著 ShippingStatus 一起變的話，這條就什麼都驗不到
        assertThat(event.normalizedStatus()).contains("11");
        assertThat(event.providerRawStatus()).isEqualTo("delivered");
    }

    @Test
    @DisplayName("② 驗簽失敗 ⇒ 拋 UberWebhookSignatureException（端點回 401；其餘一律 200／5xx）")
    void badSignatureThrows() {
        UberDirectAdapter adapter = TestUberAdapters.builder()
                .signingKeys(java.util.List.of("a-different-key")).build();
        InboundCapability inbound = adapter.inbound(ctx()).orElseThrow();

        assertThatThrownBy(() -> inbound.parseStatusEvent(signed(
                "{\"id\":\"evt_1\",\"kind\":\"event.delivery_status\",\"status\":\"delivered\"}")))
                .isInstanceOf(UberWebhookSignatureException.class);
    }

    // ==================== ③ courier_imminent 是欄位不是狀態 ====================

    @Test
    @DisplayName("③ courier_imminent 是 boolean 欄位，不是第 8 個狀態")
    void courierImminentIsAFieldNotAStatus() throws IOException {
        // 🔴 語意斷言，不釘完整句子（BL1）：文件原本把「不是狀態」與「是 boolean 欄位」寫在
        // 同一句，Stage 8B Step 5P-0 的文件重寫把它拆成相鄰兩行 ＋ 粗體強調——語意一字未改，
        // 而釘連續字串的斷言當場假紅。改成分別驗兩個語意，並要求兩者相鄰：
        // 🔴 相鄰是鑑別力的來源。只驗「全文某處有 boolean」會被文件裡任何一個 boolean 欄位滿足，
        // 只驗「全文某處有『不是狀態』」同理 —— 那樣的斷言看起來還在守，實際上已經空了。
        List<String> docLines = Files.readAllLines(API_DOC, StandardCharsets.UTF_8);
        int declLine = -1;
        for (int i = 0; i < docLines.size(); i++) {
            String line = docLines.get(i);
            if (line.contains("courier_imminent") && line.contains("boolean")) {
                declLine = i;
                break;
            }
        }
        assertThat(declLine)
                .as("文件必須有一處把 courier_imminent 宣告為 boolean 欄位（措辭可改，型別名不可改）")
                .isNotNegative();

        StringBuilder declarationBlock = new StringBuilder();
        for (int i = Math.max(0, declLine - 2); i < Math.min(docLines.size(), declLine + 3); i++) {
            declarationBlock.append(docLines.get(i)).append('\n');
        }
        assertThat(declarationBlock.toString())
                .as("宣告處附近必須明確否定它是狀態；這個否定不在了代表事實可能變了，要回頭查")
                .containsPattern("不是(第\\s*\\d+\\s*個)?狀態");

        assertThat(daasStatusesFromApiDoc())
                .as("它不該出現在狀態表裡")
                .doesNotContain("courier_imminent");
        assertThat(UberStatusMapper.knownRawStatuses()).doesNotContain("courier_imminent");
        assertThat(UberStatusMapper.normalize("courier_imminent"))
                .as("🔴 當成狀態會多出一條永不出現的邊")
                .isEmpty();

        // 帶著 courier_imminent=true 的事件，狀態仍由 status 欄決定（不被旗標帶偏）。
        InboundCapability.InboundStatusEvent event = parse(
                "{\"kind\":\"event.courier_update\",\"created\":1789000003,\"delivery_id\":\"del_4\","
                        + "\"courier_imminent\":true,\"status\":\"dropoff\"}");
        assertThat(event.normalizedStatus()).contains("31");
        assertThat(UberStatusMapper.courierImminent(Boolean.TRUE)).isTrue();
        assertThat(UberStatusMapper.courierImminent(null))
                .as("缺欄位不得當成 true——那會讓每一則事件都發一次「即將抵達」推播")
                .isFalse();
    }

    // ==================== ④ 正向 vs 逆向的 returned ====================

    @Test
    @DisplayName("④ 🔴 正向 returned 與逆向 returned 的下游語意不同（錢往相反方向走）")
    void returnedMeansOppositeThingsOnEachLeg() {
        // 兩條 leg 的 normalized 值相同（leg 是維度不是狀態，66 §7.3.2）……
        assertThat(UberStatusMapper.normalize("returned")).contains(ShippingStatus.ABNORMAL);

        // ……分歧在售後案件軸：逆向 returned ＝ 退貨包裹送不回賣家 ⇒ RETURN_FAILED(28)
        assertThat(UberStatusMapper.returnLegCaseStatus("returned"))
                .contains(AfterSaleCaseStatus.RETURN_FAILED);
        // 而逆向 delivered ＝ 退貨已送達賣家 ⇒ RETURN_DELIVERED(27)：兩者不可互換
        assertThat(UberStatusMapper.returnLegCaseStatus("delivered"))
                .contains(AfterSaleCaseStatus.RETURN_DELIVERED);
        assertThat(UberStatusMapper.returnLegCaseStatus("returned"))
                .as("🔴 逆向的 returned 與 delivered 映到不同案件狀態——"
                        + "一個是退款失敗要人介入，一個是可以退錢了")
                .isNotEqualTo(UberStatusMapper.returnLegCaseStatus("delivered"));
    }

    @Test
    @DisplayName("④ 逆向 leg 的 U5／U6：pickup_complete 進退貨運送中；pending／pickup 是「明確不處理」")
    void returnLegExplicitNoOpIsNotTheSameAsNoMapping() {
        // U5：不決定的代價是案件永遠停在 RETURN_PENDING(20)，出口是 7 天逾期的「不退款」終態
        // ⇒ 買家明明已交件卻拿不到退款（66 §7.3.2 的四問推導）。
        assertThat(UberStatusMapper.returnLegCaseStatus("pickup_complete"))
                .contains(AfterSaleCaseStatus.RETURN_SHIPPING);

        // U6：「明確不處理」要與「未識別」分得開，否則兩者混在一起沒有人分得出漏了什麼。
        assertThat(UberStatusMapper.isReturnLegExplicitNoOp("pending")).isTrue();
        assertThat(UberStatusMapper.isReturnLegExplicitNoOp("pickup")).isTrue();
        assertThat(UberStatusMapper.returnLegCaseStatus("pending")).isEmpty();

        // UP-8：逆向的 canceled **無對應** AfterSaleCaseStatus ⇒ 建 case，不猜。
        assertThat(UberStatusMapper.isReturnLegExplicitNoOp("canceled"))
                .as("🔴 它不是「明確不處理」，是 open item UP-8——兩者的處置完全不同")
                .isFalse();
        assertThat(UberStatusMapper.returnLegCaseStatus("canceled")).isEmpty();
    }

    // ==================== 去重鍵（66 §6.3.3） ====================

    @Test
    @DisplayName("去重鍵：級 1 用 provider event id；沒有時退級 2 且同內容 → 同鍵、不同內容 → 不同鍵")
    void dedupKeyPrefersEventId() {
        UberDirectAdapter adapter = TestUberAdapters.mockedAdapter();
        InboundCapability inbound = adapter.inbound(ctx()).orElseThrow();

        String withId = "{\"id\":\"evt_1\",\"kind\":\"event.delivery_status\",\"created\":1789000000,"
                + "\"delivery_id\":\"del_1\",\"status\":\"delivered\"}";
        assertThat(inbound.dedupKey(signed(withId))).isEqualTo("evt_1");

        String noId1 = "{\"kind\":\"event.delivery_status\",\"created\":1789000004,"
                + "\"delivery_id\":\"del_5\",\"status\":\"returned\"}";
        String noId2 = "{\"kind\":\"event.delivery_status\",\"created\":1789000004,"
                + "\"delivery_id\":\"del_5\",\"status\":\"canceled\"}";
        assertThat(inbound.dedupKey(signed(noId1)))
                .as("級 2 的 sha256(raw_body) 讓內容不同的兩則不會撞鍵")
                .isNotEqualTo(inbound.dedupKey(signed(noId2)));
        assertThat(inbound.dedupKey(signed(noId1))).isEqualTo(inbound.dedupKey(signed(noId1)));
    }

    @Test
    @DisplayName("Uber 沒有標籤概念：isLabelEvent 恆 false、parseLabelEvent 恆 empty")
    void uberHasNoLabelEvents() {
        InboundCapability inbound = TestUberAdapters.mockedAdapter().inbound(ctx()).orElseThrow();
        InboundPayload p = signed("{\"id\":\"evt_1\",\"kind\":\"event.delivery_status\"}");
        assertThat(inbound.isLabelEvent(p)).isFalse();
        assertThat(inbound.parseLabelEvent(p)).isEmpty();
    }

    // ==================== helpers ====================

    private static InboundCapability.InboundStatusEvent parse(String body) {
        return TestUberAdapters.mockedAdapter().inbound(ctx()).orElseThrow()
                .parseStatusEvent(signed(body));
    }

    /** 以測試金鑰簽好的 payload。⚠️ 驗簽本身的鑑別力在 {@code UberWebhookVerifierTest}（硬寫字面 HMAC）。 */
    private static InboundPayload signed(String body) {
        byte[] raw = body.getBytes(StandardCharsets.UTF_8);
        return InboundPayload.ofJson(raw,
                Map.of(UberWebhookVerifier.SIGNATURE_HEADER, hmacHex(raw)));
    }

    private static String hmacHex(byte[] raw) {
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(
                    TestUberAdapters.TEST_SIGNING_KEY.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
            return java.util.HexFormat.of().formatHex(mac.doFinal(raw));
        } catch (Exception e) {
            throw new AssertionError(e);
        }
    }

    /**
     * 從官方文件 §7.1 的 DaaS 狀態表取值域。
     * 文件不存在或表格找不到 ⇒ 直接失敗（fail-closed，不 skip）。
     */
    private static Set<String> daasStatusesFromApiDoc() throws IOException {
        if (!Files.isRegularFile(API_DOC)) {
            throw new AssertionError("找不到 Uber API 文件：" + API_DOC
                    + "——本測試的 fixture 來源消失了，不能當成通過");
        }
        String doc = Files.readString(API_DOC, StandardCharsets.UTF_8);
        int start = doc.indexOf("### 7.1 DaaS");
        if (start < 0) {
            throw new AssertionError("文件裡找不到 §7.1 DaaS 狀態表——章節可能已改名，"
                    + "adapter 認得的值域要跟著查（§7.3 的 Eats deliveries 家族是另一組，不可混用）");
        }
        int end = doc.indexOf("###", start + 1);
        String section = end < 0 ? doc.substring(start) : doc.substring(start, end);

        Set<String> values = new LinkedHashSet<>();
        Matcher m = Pattern.compile("(?m)^\\|\\s*`([a-z_]+)`\\s*\\|").matcher(section);
        while (m.find()) {
            values.add(m.group(1));
        }
        if (values.isEmpty()) {
            throw new AssertionError("§7.1 的狀態表解析不到任何值——表格格式可能已改");
        }
        return values;
    }

    private static FulfillmentContext ctx() {
        ShipmentEntity e = new ShipmentEntity();
        e.setId(1L);
        e.setOrderNo("O-UBER-WIRE-1");
        e.setShipmentNo("S-UBER-WIRE-1");
        e.setStoreId(2685914682L);
        e.setShippingMethod(ShippingMethod.UBER_INSTANT.value());
        e.setStatus(ShippingStatus.UNPROCESSED.value());
        return FulfillmentContext.forShipment(e);
    }

    /**
     * 由 module 目錄往上找到 repo root（存在 {@code .git} 的那一層）。
     *
     * <p>🔴 用 {@link Files#exists} 而非 {@code isDirectory}：<b>git worktree 的 {@code .git}
     * 是一個檔案</b>（內容是 {@code gitdir: ...}），不是目錄。用 {@code isDirectory} 會在任何
     * worktree 內一路往上找不到 root ⇒ 在 static field initializer 拋 {@code AssertionError}
     * ⇒ 整個 class 的 {@code <clinit>} 連坐 ⇒ 本檔全部 test method 變成
     * {@code NoClassDefFoundError}，而真正要驗的斷言一條都沒跑到。
     * 它在 CI 是綠的（CI 是一般 clone，{@code .git} 為目錄），所以這個形狀只會在本機 worktree
     * 現形；而實作 WP 依規定一律從 {@code origin/dev} 另開 clean worktree 施工。
     */
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
