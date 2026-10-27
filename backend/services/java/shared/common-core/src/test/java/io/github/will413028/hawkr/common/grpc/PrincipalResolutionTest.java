package io.github.will413028.hawkr.common.grpc;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.assertj.core.api.Assertions.assertThatCode;

/**
 * {@link PrincipalResolution} 的語意契約（issue #2789 階段 B1）。
 *
 * <p>這個型別存在的唯一理由是：{@code ResolvePrincipals} 對查無在架列的 id 會把輸入
 * 原樣回吐進 {@code principals}，那個值與真實解析結果逐位元無法區分。所以每一支測試
 * 都刻意用「map 有 entry ＋ 該 id 同時在 unresolvedIds」這個 server 真實會回的形狀。
 */
class PrincipalResolutionTest {

    private static final long ID = 2683382637L;

    @Test
    @DisplayName("map 有 echo-back entry 但 id 在 unresolvedIds ⇒ 視為未解析")
    void echoBackEntryIsStillUnresolved() {
        PrincipalResolution r = PrincipalResolution.of(Map.of(ID, ID), List.of(ID));

        assertThat(r.principals()).containsEntry(ID, ID); // 舊呼叫端看到的東西完全沒變
        assertThat(r.isUnresolved(ID)).isTrue();          // 新呼叫端看得出來那是回吐
    }

    @Test
    @DisplayName("真正解析到 ⇒ requireResolved 回 principal")
    void resolvedIdReturnsPrincipal() {
        PrincipalResolution r = PrincipalResolution.of(Map.of(ID, 55L), Set.of());

        assertThat(r.isUnresolved(ID)).isFalse();
        assertThat(r.requireResolved(ID, "store_id")).isEqualTo(55L);
    }

    @Test
    @DisplayName("未解析 ⇒ requireResolved 拋，例外帶得出 id 與語意")
    void unresolvedIdThrows() {
        PrincipalResolution r = PrincipalResolution.of(Map.of(ID, ID), List.of(ID));

        assertThatThrownBy(() -> r.requireResolved(ID, "store_id"))
                .isInstanceOf(UnresolvedPrincipalException.class)
                .satisfies(e -> {
                    UnresolvedPrincipalException ex = (UnresolvedPrincipalException) e;
                    assertThat(ex.idKind()).isEqualTo("store_id");
                    assertThat(ex.unresolvedIds()).containsExactly(ID);
                });
    }

    @Test
    @DisplayName("map 缺 entry、值為 0 或負數 ⇒ 一律視為未解析（防禦性，正常回應不會出現）")
    void missingOrNonPositivePrincipalIsUnresolved() {
        PrincipalResolution missing = PrincipalResolution.of(Map.of(), Set.of());
        PrincipalResolution zero = PrincipalResolution.of(Map.of(ID, 0L), Set.of());

        assertThat(missing.isUnresolved(ID)).isTrue();
        assertThat(zero.isUnresolved(ID)).isTrue();
        assertThat(missing.isUnresolved(null)).isTrue();
    }

    @Test
    @DisplayName("requireAllResolved：任一未解析即拋，且例外列出全部未解析的 id")
    void requireAllResolvedReportsEveryUnresolvedId() {
        PrincipalResolution r = PrincipalResolution.of(
                Map.of(1L, 1L, 2L, 22L, 3L, 3L), List.of(1L, 3L));

        assertThatThrownBy(() -> r.requireAllResolved(List.of(1L, 2L, 3L), "store_id"))
                .isInstanceOf(UnresolvedPrincipalException.class)
                .satisfies(e -> assertThat(((UnresolvedPrincipalException) e).unresolvedIds())
                        .containsExactlyInAnyOrder(1L, 3L));
    }

    @Test
    @DisplayName("requireAllResolved 只回傳被檢查過的 id，避免呼叫端誤查未驗證的 entry")
    void requireAllResolvedReturnsOnlyCheckedIds() {
        PrincipalResolution r = PrincipalResolution.of(
                Map.of(1L, 11L, 2L, 22L, 99L, 99L), List.of(99L));

        Map<Long, Long> checked = r.requireAllResolved(List.of(1L, 2L), "store_id");

        assertThat(checked).containsOnlyKeys(1L, 2L);
        assertThat(checked).doesNotContainKey(99L);
    }

    @Test
    @DisplayName("null id 進 requireAllResolved 不會炸成 NPE，而是走未解析路徑")
    void nullIdInBatchThrowsUnresolvedNotNpe() {
        PrincipalResolution r = PrincipalResolution.of(Map.of(1L, 11L), Set.of());
        java.util.List<Long> ids = new java.util.ArrayList<>();
        ids.add(1L);
        ids.add(null);

        assertThatThrownBy(() -> r.requireAllResolved(ids, "store_id"))
                .isInstanceOf(UnresolvedPrincipalException.class);
    }

    @Test
    @DisplayName("空輸入：empty() 與 requireAllResolved(空) 都不拋")
    void emptyInputsAreNotFailures() {
        assertThat(PrincipalResolution.empty().principals()).isEmpty();
        assertThatCode(() -> PrincipalResolution.empty().requireAllResolved(List.of(), "store_id"))
                .doesNotThrowAnyException();
    }

    @Test
    @DisplayName("建構後外部改動原 map 不會影響已建立的 resolution（防禦性複製）")
    void defensivelyCopiesInputs() {
        Map<Long, Long> mutable = new HashMap<>();
        mutable.put(ID, 55L);
        PrincipalResolution r = PrincipalResolution.of(mutable, Set.of());

        mutable.put(ID, 66L);

        assertThat(r.requireResolved(ID, "store_id")).isEqualTo(55L);
    }

    @Test
    @DisplayName("principalOrZero：解析到回 principal；unresolved_ids、map 沒有、非正值一律回 0，不回吐 id")
    void principalOrZeroNeverEchoesTheId() {
        assertThat(PrincipalResolution.of(Map.of(ID, 42L), List.of()).principalOrZero(ID, "buyer_id")).isEqualTo(42L);
        assertThat(PrincipalResolution.of(Map.of(ID, ID), List.of(ID)).principalOrZero(ID, "buyer_id")).isZero();
        assertThat(PrincipalResolution.of(Map.of(), List.of()).principalOrZero(ID, "store_id")).isZero();
        assertThat(PrincipalResolution.of(Map.of(ID, 0L), List.of()).principalOrZero(ID, "store_id")).isZero();
        assertThat(PrincipalResolution.empty().principalOrZero(null, "store_id")).isZero();
    }
}
