package io.github.will413028.hawkr.media.util;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;

/**
 * {@code AssetFilename}（issue #3149，設計書 08 §4.9.3／§4.9.3a／§4.9.4）。
 *
 * <p>🔴 <b>每一個期望值都是硬寫的字面值</b>，不呼叫被測函式、也不引用被測類別的任何常數
 * （255 的上限、{@code .attempt-} 的 pattern 都在這裡重新寫死）。期望值若跟著被測物一起變，
 * 這些測試就什麼都驗不到。
 */
class AssetFilenameTest {

    @Nested
    @DisplayName("sanitize：寫入側消毒（§4.9.3）")
    class Sanitize {

        @Test
        @DisplayName("乾淨的檔名一個字都不動")
        void keepsCleanNameUnchanged() {
            assertThat(AssetFilename.sanitize("報表 2026-09.pdf")).isEqualTo("報表 2026-09.pdf");
        }

        @Test
        @DisplayName("含路徑取 basename（POSIX 與 Windows 反斜線都要）")
        void takesBasename() {
            // 瀏覽器在少數情況會送出完整路徑；路徑成分不該進 DB 也不該進 header
            assertThat(AssetFilename.sanitize("a/b/c.pdf")).isEqualTo("c.pdf");
            assertThat(AssetFilename.sanitize("C:\\Users\\user\\Desktop\\稅籍證明.pdf"))
                    .isEqualTo("稅籍證明.pdf");
            // 混用時取最後一個分隔符之後
            assertThat(AssetFilename.sanitize("a\\b/c\\d.png")).isEqualTo("d.png");
        }

        @Test
        @DisplayName("CR／LF 被移除——header injection 的直接防線")
        void stripsCrLf() {
            assertThat(AssetFilename.sanitize("re\r\nport.pdf")).isEqualTo("report.pdf");
            // 注入整個假 header 的形態
            assertThat(AssetFilename.sanitize("x.pdf\r\nSet-Cookie: a=b"))
                    .isEqualTo("x.pdfSet-Cookie: a=b");
        }

        @Test
        @DisplayName("NUL（0x00）被移除——PostgreSQL 的 VARCHAR 直接拒收它")
        void stripsNul() {
            // %00 經 URL path 解碼後是真的 0x00；點名清除 " / CR / LF 會漏掉它，
            // 而漏掉的後果是一條新引入的 INSERT 失敗路徑（§4.9.3 的白名單語意）
            assertThat(AssetFilename.sanitize("re\u0000port.pdf")).isEqualTo("report.pdf");
            // C0 其他成員與 C1 區段（0x7F–0x9F）一併涵蓋
            assertThat(AssetFilename.sanitize("a\u0001b\u001Fc\u007Fd\u0085e.pdf")).isEqualTo("abcde.pdf");
        }

        @Test
        @DisplayName("超長截斷且保留副檔名（總長 255）")
        void truncatesKeepingExtension() {
            String raw = "a".repeat(300) + ".pdf";

            String got = AssetFilename.sanitize(raw);

            // 硬寫：251 個 a ＋ ".pdf" ＝ 255 字元
            assertThat(got).isEqualTo("a".repeat(251) + ".pdf");
            assertThat(got).hasSize(255);
            assertThat(got).endsWith(".pdf");
        }

        @Test
        @DisplayName("PostgreSQL 的 VARCHAR(255) 計字元不計位元組：CJK 檔名不得被砍到顯示寬度")
        void truncatesByCharactersNotDisplayWidth() {
            String raw = "中".repeat(300) + ".pdf";

            String got = AssetFilename.sanitize(raw);

            // 若誤用「顯示寬度」（CJK 記 2 格）截斷，這裡會變成 127 個字
            assertThat(got).hasSize(255);
            assertThat(got).isEqualTo("中".repeat(251) + ".pdf");
        }

        @Test
        @DisplayName("清洗後為空 → 回 null（空字串與 NULL 在這裡語意相同，只留一種表示法）")
        void returnsNullWhenNothingLeft() {
            assertThat(AssetFilename.sanitize("   ")).isNull();
            assertThat(AssetFilename.sanitize("\r\n")).isNull();
            assertThat(AssetFilename.sanitize("\u0000")).isNull();
            // 只有路徑、沒有檔名
            assertThat(AssetFilename.sanitize("a/b/")).isNull();
            assertThat(AssetFilename.sanitize("")).isNull();
        }

        @Test
        @DisplayName("輸入 null 回 null")
        void returnsNullForNullInput() {
            assertThat(AssetFilename.sanitize(null)).isNull();
        }

        @Test
        @DisplayName("雙引號必須被移除——builder 的 charset 多載並不逃逸它（設計書原本的假設是假的）")
        void stripsDoubleQuote() {
            // 🔴 設計書 §4.9.3 原本寫「雙引號刻意不移除，安全性由 ContentDisposition builder
            // 負責」。**那個前提在我們實際使用的 spring-web 6.1.2 上不成立**，實測：
            //   ContentDisposition.attachment().filename("a\"b.pdf", UTF_8).build()
            //     → attachment; filename="=?UTF-8?Q?a"b.pdf?="; filename*=UTF-8''a%22b.pdf
            //                                          ^ 裸引號把 quoted-string 提前收尾
            // 只有**不帶 charset** 的多載走 encodeQuotedPairs 才會逃逸；我們用的
            // filename(String, Charset) 走 encodeQuotedPrintableFilename，其 PRINTABLE 位元集
            // 只排除 = ? _，" (34) 與 \\ (92) 原樣輸出。
            // ⚠️ 這是**版本相依**的：spring-web 7.0.6 已修（輸出 =22）。所以防線放在寫入側消毒，
            // 不放在 builder——升級 Spring 不該是這條防線成立與否的前提。
            assertThat(AssetFilename.sanitize("my\"file\".pdf")).isEqualTo("myfile.pdf");
            assertThat(AssetFilename.sanitize("a\"b.pdf")).isEqualTo("ab.pdf");
        }

        @Test
        @DisplayName("消毒後的輸出恆不含 \" 與 \\ ——進 header 前的最後防線")
        void outputNeverContainsQuoteOrBackslash() {
            // 反斜線在 basename 那一步就被當成路徑分隔符取走，引號則由字元類別清除；
            // 兩者都不得殘留，否則 quoted-string 會被提前收尾。
            assertThat(AssetFilename.sanitize("a/b/../evil\".pdf")).isEqualTo("evil.pdf");
            assertThat(AssetFilename.sanitize("back\\slash.pdf")).isEqualTo("slash.pdf");
            assertThat(AssetFilename.sanitize("x.pdf\"; filename=\"evil.exe"))
                    .isEqualTo("x.pdf; filename=evil.exe");

            for (String raw : new String[]{
                    "my\"file\".pdf", "a\"b.pdf", "a/b/../evil\".pdf", "back\\slash.pdf",
                    "x.pdf\"; filename=\"evil.exe", "\"\"\".pdf"}) {
                assertThat(AssetFilename.sanitize(raw))
                        .as("消毒輸出不得含裸引號或反斜線，輸入=%s", raw)
                        .doesNotContain("\"")
                        .doesNotContain("\\");
            }
        }

        @Test
        @DisplayName("前後空白 trim 掉")
        void trimsSurroundingWhitespace() {
            assertThat(AssetFilename.sanitize("  報表.pdf  ")).isEqualTo("報表.pdf");
        }
    }

    @Nested
    @DisplayName("resolveDisplayName：讀取側單一入口（§4.9.4）")
    class ResolveDisplayName {

        @Test
        @DisplayName("原始檔名存在時直接回，不看 objectKey")
        void originalFilenameWins() {
            // 兩個值刻意完全不同：相同的話「原始優先」壞掉也不會被發現
            assertThat(AssetFilename.resolveDisplayName(
                    "稅籍證明.pdf", "secure/certification/886/20260901/9f1c2b3a.pdf"))
                    .isEqualTo("稅籍證明.pdf");
        }

        @Test
        @DisplayName("原始檔名為 null → 回退到 objectKey 尾段")
        void fallsBackToObjectKeyTail() {
            assertThat(AssetFilename.resolveDisplayName(
                    null, "public/product/12345/20260129/3f2504e0.jpg"))
                    .isEqualTo("3f2504e0.jpg");
        }

        @Test
        @DisplayName("回退時剝掉 .attempt-{uuid} 後綴（#2759 的 per-attempt key）")
        void stripsAttemptSuffixOnFallback() {
            assertThat(AssetFilename.resolveDisplayName(
                    null,
                    "secure/certification/886/20260901/3f2504e0-4f89-11d3-9a0c-0305e82c3301.pdf"
                            + ".attempt-6ba7b810-9dad-11d1-80b4-00c04fd430c8"))
                    .isEqualTo("3f2504e0-4f89-11d3-9a0c-0305e82c3301.pdf");
        }

        @Test
        @DisplayName("長得像但不是 uuid 的 .attempt- 後綴不剝（避免誤傷真的檔名）")
        void doesNotStripNonUuidAttemptSuffix() {
            assertThat(AssetFilename.resolveDisplayName(null, "secure/x/report.pdf.attempt-1"))
                    .isEqualTo("report.pdf.attempt-1");
        }

        @Test
        @DisplayName("objectKey 不含 '/' 時整串就是尾段")
        void objectKeyWithoutSlash() {
            assertThat(AssetFilename.resolveDisplayName(null, "plain.jpg")).isEqualTo("plain.jpg");
        }

        @Test
        @DisplayName("原始檔名是空白字串 → 視同無，走回退")
        void blankOriginalIsTreatedAsAbsent() {
            assertThat(AssetFilename.resolveDisplayName("   ", "public/a/b/c.jpg")).isEqualTo("c.jpg");
            assertThat(AssetFilename.resolveDisplayName("", "public/a/b/c.jpg")).isEqualTo("c.jpg");
        }

        @Test
        @DisplayName("回退值也必須消毒——object_key 尾段不是乾淨的（sanitizedExtension 放行引號）")
        void fallbackValueIsSanitized() {
            // 🔴 這一格守的是一條真實可達的路徑，不是理論：
            // AbstractStorageService.sanitizedExtension 只拒絕 / \\ ..，**引號原樣通過**
            // ⇒ 使用者送 `a.pd"f` 就足以讓 object_key 尾段帶引號；而 V9 之前的列沒有
            // original_filename 可用 ⇒ 一定走回退 ⇒ 那個引號會直接進 Content-Disposition，
            // 而 spring-web 6.1.2 的 builder 不會逃逸它。
            assertThat(AssetFilename.resolveDisplayName(null, "public/product/1/20260101/a.pd\"f"))
                    .isEqualTo("a.pdf");
            // 剝 .attempt- 後綴與消毒兩件事要能疊加
            assertThat(AssetFilename.resolveDisplayName(null,
                    "secure/certification/886/20260901/u.pd\"f"
                            + ".attempt-91d7608d-9dad-11d1-80b4-00c04fd430c8"))
                    .isEqualTo("u.pdf");
            // 控制字元同理（回退值一樣會進 header）
            assertThat(AssetFilename.resolveDisplayName(null, "public/p/1/2/re\u0000port.pdf"))
                    .isEqualTo("report.pdf");
        }

        @Test
        @DisplayName("原始檔名那條分支也過消毒（冪等，且不依賴寫入端紀律）")
        void originalFilenameBranchIsSanitizedToo() {
            // 已消毒的值再跑一次沒有代價
            assertThat(AssetFilename.resolveDisplayName("已消毒的檔名.pdf", null))
                    .isEqualTo("已消毒的檔名.pdf");
            // 萬一有任何寫入路徑漏了消毒（或是 V9 之前手動塞進去的值），這裡仍是安全的
            assertThat(AssetFilename.resolveDisplayName("a\"b.pdf", "x/y.pdf")).isEqualTo("ab.pdf");
        }

        @Test
        @DisplayName("回傳值恆不含 \" 與 \\ ——這是本函式對呼叫端的保證")
        void returnValueNeverContainsQuoteOrBackslash() {
            // 四個消費點（batchGetAssetMetadata／serverReadSecureAsset／兩個 controller 的
            // preview 與 download）都直接把這個回傳值餵進 header 或 JSON，不再自行防護。
            String[][] cases = {
                    {null, "public/p/1/2/a.pd\"f"},
                    {null, "secure/c/1/2/x\"y\".pdf"},
                    {"q\"uoted.pdf", "public/p/1/2/clean.pdf"},
                    {null, "public/p/1/2/back\\slash.pdf"},
            };
            for (String[] c : cases) {
                assertThat(AssetFilename.resolveDisplayName(c[0], c[1]))
                        .as("original=%s objectKey=%s", c[0], c[1])
                        .doesNotContain("\"")
                        .doesNotContain("\\");
            }
        }

        @Test
        @DisplayName("回退值消毒後為空 → 回空字串（不得回 null）")
        void returnsEmptyWhenFallbackSanitizesToNothing() {
            assertThat(AssetFilename.resolveDisplayName(null, "public/p/1/2/\u0000")).isEqualTo("");
        }

        @Test
        @DisplayName("兩者皆不可用 → 回空字串，不回 null（契約釘死）")
        void returnsEmptyStringWhenNothingAvailable() {
            // proto 的 string 欄位與 Content-Disposition 都不接受 null；
            // 改成回 null 會讓四個消費點各自補一次 null 判斷（那正是本函式要消滅的東西）
            assertThat(AssetFilename.resolveDisplayName(null, null)).isEqualTo("");
            assertThat(AssetFilename.resolveDisplayName("  ", null)).isEqualTo("");
        }
    }

    @Nested
    @DisplayName("fromSourceUrl：匯入路徑的 URL → 檔名（§4.9.3a）")
    class FromSourceUrl {

        @Test
        @DisplayName("query 與 fragment 被剝掉——簽名 token 不得流進 DB 與 header")
        void stripsQueryAndFragment() {
            String got = AssetFilename.fromSourceUrl(
                    "https://cdn.example.com/a/img.jpg?token=SECRET&exp=1&sig=SECRET#frag");

            assertThat(got).isEqualTo("img.jpg");
            // 直接對整串 URL 取 basename（最直覺的寫法）會把 token 一路寫進 DB
            assertThat(got).doesNotContain("SECRET");
            assertThat(got).doesNotContain("?");
            assertThat(got).doesNotContain("#");
        }

        @Test
        @DisplayName("不得再 URL-decode 一次之一：100%25_off.jpg 不拋例外")
        void doesNotDoubleDecode_percent() {
            // getPath() 回的已是解碼後的路徑；再解一次 "%_o" 不是合法的 %XX ⇒ 拋例外
            assertThatCode(() -> AssetFilename.fromSourceUrl("https://cdn.example.com/100%25_off.jpg"))
                    .doesNotThrowAnyException();
            assertThat(AssetFilename.fromSourceUrl("https://cdn.example.com/100%25_off.jpg"))
                    .isEqualTo("100%_off.jpg");
        }

        @Test
        @DisplayName("不得再 URL-decode 一次之二：my+file.jpg 的 '+' 不得變成空格")
        void doesNotDoubleDecode_plus() {
            assertThat(AssetFilename.fromSourceUrl("https://cdn.example.com/my+file.jpg"))
                    .isEqualTo("my+file.jpg");
        }

        @Test
        @DisplayName("不得再 URL-decode 一次之三：a%252F..%252F.. 不得還原出真的路徑分隔符")
        void doesNotDoubleDecode_pathTraversal() {
            String got = AssetFilename.fromSourceUrl("https://cdn.example.com/a%252F..%252F..jpg");

            assertThat(got).isEqualTo("a%2F..%2F..jpg");
            assertThat(got).doesNotContain("/");
            assertThat(got).doesNotContain("\\");
        }

        @Test
        @DisplayName("trailing slash（.../dir/）→ null")
        void trailingSlashYieldsNull() {
            assertThat(AssetFilename.fromSourceUrl("https://cdn.example.com/dir/")).isNull();
            assertThat(AssetFilename.fromSourceUrl("https://cdn.example.com/")).isNull();
        }

        @Test
        @DisplayName("opaque URI（data:）→ null，不拋例外")
        void opaqueUriYieldsNull() {
            assertThat(AssetFilename.fromSourceUrl("data:image/png;base64,AAAA")).isNull();
        }

        @Test
        @DisplayName("未編碼空格的非法 URL → 回 null，不得讓整支拋出")
        void illegalUriYieldsNullNotThrow() {
            // 匯入圖片拿不到檔名不是錯誤——顯示值壞掉不該讓上傳／轉正失敗
            assertThatCode(() -> AssetFilename.fromSourceUrl("https://cdn.example.com/a b.jpg"))
                    .doesNotThrowAnyException();
            assertThat(AssetFilename.fromSourceUrl("https://cdn.example.com/a b.jpg")).isNull();
        }

        @Test
        @DisplayName("null／空白輸入 → null（URI.create(null) 拋的是 NPE，必須在解析前擋掉）")
        void nullInputYieldsNull() {
            assertThatCode(() -> AssetFilename.fromSourceUrl(null)).doesNotThrowAnyException();
            assertThat(AssetFilename.fromSourceUrl(null)).isNull();
            assertThat(AssetFilename.fromSourceUrl("   ")).isNull();
        }

        @Test
        @DisplayName("%00 解出的 NUL 被消毒掉（否則 INSERT 會炸在 VARCHAR）")
        void stripsDecodedNul() {
            assertThat(AssetFilename.fromSourceUrl("https://cdn.example.com/re%00port.pdf"))
                    .isEqualTo("report.pdf");
        }

        @Test
        @DisplayName("路徑深層時只取最後一段")
        void takesLastPathSegment() {
            assertThat(AssetFilename.fromSourceUrl("https://cdn.example.com/a/b/c/櫻桃.jpg"))
                    .isEqualTo("櫻桃.jpg");
        }
    }
}
