package io.github.will413028.hawkr.common.webhook;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Locale;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * Uber webhook 驗簽（66 §6.3.1、API 總覽 §6.1）。
 *
 * <h2>🔴 期望值是<b>硬寫的字面 HMAC</b>，不是用被測物算出來的</h2>
 * 用 {@code UberWebhookVerifier} 自己（或它內部那段 {@code Mac}）算出期望值，
 * 等於「期望值跟著被測物一起變」——把演算法改成 HmacSHA1、把金鑰換成別的，
 * 兩邊會一起變而測試照樣綠（{@code 03-delegation-templates.md} §0.3 第二種形態）。
 *
 * <p>下面那串 hex 由<b>獨立於本 repo 的工具</b>算出：
 * {@code python3 -c "import hmac,hashlib;print(hmac.new(key,body,hashlib.sha256).hexdigest())"}。
 */
@DisplayName("Uber webhook 驗簽")
class UberWebhookVerifierTest {

    // 🔴 金鑰硬寫字面值，不從別的模組引用：本檔的 VALID_SIG 是對「這一把金鑰 × 這一份 body」
    //    算好的 hex 常數，兩者必須成對。從外部常數取金鑰會讓那一端一改，
    //    這裡就變成「拿被測物自己算出來的東西驗被測物」。
    //    ⚠️ shipping-service 的 TestUberAdapters.TEST_SIGNING_KEY 是同一個字面值，
    //    兩邊各自硬寫是刻意的。
    private static final String KEY = "test-uber-webhook-signing-key";
    private static final String BODY =
            "{\"id\":\"evt_1\",\"kind\":\"event.delivery_status\",\"created\":1789000000,"
                    + "\"delivery_id\":\"del_1\",\"status\":\"delivered\"}";
    /** HMAC-SHA256(KEY, BODY) 的 hex —— 硬寫字面值，不由被測物算。 */
    private static final String VALID_SIG =
            "a16cb92d084f62b4be271f7245cfd36f60ecb4451a073d8c0c67faa5f6cf0b66";

    @Test
    @DisplayName("x-uber-signature 正確 ⇒ 通過")
    void validSignaturePasses() {
        assertThat(verifier(KEY).verify(raw(BODY), Map.of(UberWebhookVerifier.SIGNATURE_HEADER, VALID_SIG)))
                .isTrue();
    }

    @Test
    @DisplayName("🔴 body 改一個位元組 ⇒ 不通過（簽的是 raw bytes，不是 re-serialize 後的 JSON）")
    void tamperedBodyFails() {
        String tampered = BODY.replace("\"status\":\"delivered\"", "\"status\":\"canceled\"");
        assertThat(tampered).isNotEqualTo(BODY);
        assertThat(verifier(KEY).verify(raw(tampered), Map.of(UberWebhookVerifier.SIGNATURE_HEADER, VALID_SIG)))
                .isFalse();
    }

    @Test
    @DisplayName("🔴 換一把金鑰 ⇒ 不通過（簽章不是「有帶就算數」）")
    void wrongKeyFails() {
        assertThat(verifier("another-key").verify(raw(BODY), Map.of(UberWebhookVerifier.SIGNATURE_HEADER, VALID_SIG)))
                .isFalse();
    }

    @Test
    @DisplayName("🔴 零把金鑰 ⇒ 一律不通過（fail-closed，不得「沒設定就放行」）")
    void noKeysFailClosed() {
        assertThat(new UberWebhookVerifier(List.of())
                .verify(raw(BODY), Map.of(UberWebhookVerifier.SIGNATURE_HEADER, VALID_SIG)))
                .isFalse();
    }

    @Test
    @DisplayName("🔴 signing key 是一組不是一個：第二把命中也要通過（陷阱 C1）")
    void anyRegisteredKeyPasses() {
        // 66 §8.5：per-webhook signing key ⇒ 只放一把，多註冊一個 webhook 就靜默驗簽失敗，
        // 而我方回 401 ⇒ Uber 不重試 4xx ⇒ 該 webhook 的事件全丟。
        UberWebhookVerifier multi = new UberWebhookVerifier(List.of("first-webhook-key", KEY));
        assertThat(multi.verify(raw(BODY), Map.of(UberWebhookVerifier.SIGNATURE_HEADER, VALID_SIG))).isTrue();
    }

    @Test
    @DisplayName("ofKeyMap：{webhookId: key} 的 map 形狀（66 §8.5 的 UBER_WEBHOOK_SIGNING_KEYS）")
    void keyMapFormIsSupported() {
        UberWebhookVerifier fromMap = UberWebhookVerifier.ofKeyMap(Map.of("wh_1", "other-key", "wh_2", KEY));
        assertThat(fromMap.verify(raw(BODY), Map.of(UberWebhookVerifier.SIGNATURE_HEADER, VALID_SIG))).isTrue();
    }

    @Test
    @DisplayName("舊版 x-postmates-signature 也接受（Delivery Status／Courier Update）")
    void legacyHeaderAccepted() {
        assertThat(verifier(KEY).verify(raw(BODY), Map.of(UberWebhookVerifier.LEGACY_SIGNATURE_HEADER, VALID_SIG)))
                .isTrue();
    }

    @Test
    @DisplayName("header 名大小寫不敏感（不同 ingress 正規化成不同大小寫）")
    void headerNameIsCaseInsensitive() {
        assertThat(verifier(KEY).verify(raw(BODY), Map.of("X-Uber-Signature", VALID_SIG))).isTrue();
    }

    @Test
    @DisplayName("簽章 hex 大小寫不敏感（先解碼再比 bytes，而不是比字串）")
    void signatureHexIsCaseInsensitive() {
        assertThat(verifier(KEY).verify(raw(BODY), Map.of(UberWebhookVerifier.SIGNATURE_HEADER, VALID_SIG.toUpperCase(Locale.ROOT)))).isTrue();
    }

    @Test
    @DisplayName("缺 header／缺 raw body ⇒ 不通過")
    void missingSignatureOrBodyFails() {
        assertThat(verifier(KEY).verify(raw(BODY), Map.of("x-other", VALID_SIG))).isFalse();
        assertThat(verifier(KEY).verify(null,
                Map.of(UberWebhookVerifier.SIGNATURE_HEADER, VALID_SIG))).isFalse();
        assertThat(verifier(KEY).verify(new byte[0],
                Map.of(UberWebhookVerifier.SIGNATURE_HEADER, VALID_SIG))).isFalse();
        assertThat(verifier(KEY).verify(raw(BODY), null)).isFalse();
    }

    @Test
    @DisplayName("🔴 非 hex 的簽章回 false，不拋例外（否則「格式錯」會變成 500 而不是驗簽失敗）")
    void malformedSignatureIsRejectedNotThrown() {
        assertThat(verifier(KEY).verify(raw(BODY), Map.of(UberWebhookVerifier.SIGNATURE_HEADER, "zz!!")))
                .isFalse();
    }

    /**
     * 🔴 <b>結構斷言，不是行為斷言</b>——而且本檔刻意把這件事寫清楚。
     *
     * <h2>為什麼行為測不到</h2>
     * constant-time 要防的是<b>時間側通道</b>：{@code String.equals}／{@code Arrays.equals}
     * 在第一個不同的位元組就返回，讓攻擊者能以回應時間逐位元組還原簽章。
     * 那個差異是奈秒等級、且被 JIT 與 GC 淹沒，<b>單元測試量不到</b>。
     *
     * <h2>變異實測（2026-09-12）</h2>
     * <ul>
     *   <li>把比較換成 {@code String.equals}(hex) ⇒ {@link #signatureHexIsCaseInsensitive}
     *       轉紅——<b>但抓到它的是大小寫，不是 timing</b></li>
     *   <li>換成 {@code String.equalsIgnoreCase}(hex) ⇒ 上面那支<b>照樣綠</b>
     *       ⇒ 沒有本條的話，那個變異完全無人看守</li>
     * </ul>
     * ⇒ 本條補的正是第二種變異的缺口。它證明的是「用了對的 API」，
     * 不是「時間常數成立」——後者只能靠 review 與 API 選擇保證。
     */
    @Test
    @DisplayName("🔴 結構斷言：簽章比較必須走 MessageDigest.isEqual（timing 本身測不到）")
    void comparisonUsesConstantTimeApi() throws java.io.IOException {
        java.nio.file.Path source = repoRoot().resolve(
                "services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/"
                        + "webhook/UberWebhookVerifier.java");
        if (!java.nio.file.Files.isRegularFile(source)) {
            throw new AssertionError("找不到被測原始碼：" + source
                    + "——檔案搬家了，本斷言的錨要跟著改，不能當成通過");
        }
        String code = java.nio.file.Files.readString(source, java.nio.charset.StandardCharsets.UTF_8);
        // 只看 verify() 這個方法體，避免 javadoc 裡提到的 API 名把斷言變成恆真。
        int start = code.indexOf("public boolean verify(");
        assertThat(start).as("verify() 方法名改了 ⇒ 本斷言的錨失效").isGreaterThan(0);
        // 🔴 範圍**只到 verify() 自己**，用「下一個 private 方法」當終點而不是某個具名方法。
        //
        // ⚠️ 2026-09-12 實測踩到：原本錨在 hmacSha256()，而中間新增了一支
        //    header()（大小寫不敏感的 header 查找，內含 equalsIgnoreCase）
        //    ⇒ 掃描範圍多吃了一支與簽章比較無關的方法 ⇒ 否定斷言誤報。
        //    那**不是誤報該被放寬**，是錨該收窄：本條要守的是「簽章怎麼比」，
        //    不是「這個檔案裡有沒有出現過 equalsIgnoreCase」。
        int end = code.indexOf("\n    private static ", start);
        assertThat(end).as("verify() 之後沒有 private 方法了 ⇒ 本斷言的範圍失效").isGreaterThan(start);
        String verifyBody = code.substring(start, end);

        assertThat(verifyBody)
                .as("constant-time 比較是這支方法唯一的安全性質，換成字串比較不會有任何行為差異被測到")
                .contains("MessageDigest.isEqual");
        assertThat(verifyBody)
                .as("🔴 禁止 String.equals／equalsIgnoreCase：前者提早返回，後者連大小寫斷言都繞得過")
                .doesNotContain(".equals(")
                .doesNotContain(".equalsIgnoreCase(");
    }

    /** 由 module 目錄往上找到 repo root（存在 {@code .git} 的那一層）。 */
    private static java.nio.file.Path repoRoot() {
        java.nio.file.Path p = java.nio.file.Path.of("").toAbsolutePath();
        // backend 根目錄以 go.work 為錨：monorepo 裡 .git 在上一層，不是 backend/。
        while (p != null && !java.nio.file.Files.exists(p.resolve("go.work"))) {
            p = p.getParent();
        }
        if (p == null) {
            throw new AssertionError("找不到 backend 根目錄（沒有 go.work）");
        }
        return p;
    }

    private static UberWebhookVerifier verifier(String key) {
        return new UberWebhookVerifier(List.of(key));
    }

    private static byte[] raw(String body) {
        return body.getBytes(StandardCharsets.UTF_8);
    }
}
