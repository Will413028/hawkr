package io.github.will413028.hawkr.common.grpc;

import java.util.Collection;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * {@code ResolvePrincipals} 的解析結果（seller / buyer 兩軸同形）。
 *
 * <p><b>為什麼需要這個型別</b>：seller-service / buyer-service 的
 * {@code ResolvePrincipals} 對「查無在架資料」的 id（軟刪除的 stint、或根本不存在的 id）
 * 會以 {@code code=0} 把<b>輸入值原樣回吐</b>（{@code COALESCE(x.account_id, input.id)}）。
 * 呼叫端若只拿 {@code principals} 這張 map，那個回吐值與真實解析結果<b>逐位元無法區分</b>：
 * 它是一個合法非零的 id，會被當成真的 account_id 用下去。
 * 兩支 proto 因此都補了 {@code unresolved_ids}（註解逐字寫明「UNVERIFIED echo-back,
 * not a real resolution」），本型別就是把「map ＋ 未解析清單」綁在一起，讓呼叫端
 * <b>拿不到只有 map 的版本</b>——這是這個 class 的結構性修法，不是又一個要記得檢查的欄位。
 *
 * <p><b>閘門類（防詐 / 授權 / 擁有權比對）一律用 {@link #requireResolved} 或
 * {@link #requireAllResolved}</b>：未解析即拋 {@link UnresolvedPrincipalException}，
 * 由呼叫端轉成自己服務的業務錯誤碼並記 log。理由是回吐值會把「兩邊都未解析」變成
 * 「兩邊相等」，而每個等價比較的 equal 分支都是拒絕／放行／刪除的觸發條件
 * （media 擁有權比對 ⇒ 越權放行；cart mergeCart ⇒ 刪光訪客車）。
 *
 * <p>通知類呼叫端（值只當推播收件人）取捨相反、不適用本型別的 require 方法，
 * 直接讀 {@link #principals()} 即可，行為與補 {@code unresolved_ids} 之前完全一致。
 *
 * <p>本型別是 seller / buyer 兩軸<b>唯一</b>的解析結果型別。aftersales-service 的
 * {@code BuyerGrpcClient.PrincipalResolution}（買家軸前身，同樣的
 * {@code (principals, unresolvedIds)} 形狀）已於 issue #2789 階段 B2 收斂進來 ——
 * 留兩份的實害不是重複本身，而是「閘門必須用 {@link #requireResolved}」這條規則
 * 在那一份上<b>選不到</b>，日後的閘門作者只要注入到前身型別就繞過了整條保證。
 * 新增服務端 client 時一律回傳本型別，不要再定義同形的 nested record。
 *
 * @param principals    只含真正解析到的 id；未解析的 id 沒有 entry（單體不再回吐 echo-back；舊的 seller-service、buyer-service 仍會把
 *                      輸入原樣回填，所以未解析一律以 {@link #isUnresolved} 判斷，不直接讀 map）
 * @param unresolvedIds 真正查無在架列的 id 子集（已關閉、軟刪除或不存在）
 */
public record PrincipalResolution(Map<Long, Long> principals, Set<Long> unresolvedIds) {

    private static final Logger LOG = LoggerFactory.getLogger(PrincipalResolution.class);

    private static final PrincipalResolution EMPTY =
            new PrincipalResolution(Map.of(), Set.of());

    public PrincipalResolution {
        principals = (principals == null) ? Map.of() : Map.copyOf(principals);
        unresolvedIds = (unresolvedIds == null) ? Set.of() : Set.copyOf(unresolvedIds);
    }

    /** 空輸入的解析結果（呼叫端 id 清單為空時的短路回傳，不代表「全部未解析」）。 */
    public static PrincipalResolution empty() {
        return EMPTY;
    }

    public static PrincipalResolution of(Map<Long, Long> principals, Collection<Long> unresolvedIds) {
        return new PrincipalResolution(
                principals,
                unresolvedIds == null ? Set.of() : new LinkedHashSet<>(unresolvedIds));
    }

    /**
     * 全部視為已解析。
     *
     * <p>⚠️ 僅供測試 fixture 與「輸入本身就不經過 RPC」的場合使用。
     * <b>禁止</b>拿它包裝真實 {@code ResolvePrincipalsResponse}——那等於把
     * {@code unresolved_ids} 丟掉，本型別要防的東西就全部失效。
     */
    public static PrincipalResolution allResolved(Map<Long, Long> principals) {
        return new PrincipalResolution(principals, Set.of());
    }

    /**
     * 通知類用：真正解析到就回 principal，未解析回 {@code 0L}（沒有收件人，notification 端略過）並記 warn（含 id 與原因）。
     * <b>不回吐 id 本身</b>：單體的 store_id、buyer_id 與 account_id 是互不相干的序列，回吐值是另一個帳號。閘門類用
     * {@link #requireResolved}，不用這個。
     *
     * @param idKind 這個 id 的語意（{@code "store_id"} / {@code "buyer_id"}），只進 log
     */
    public long principalOrZero(Long id, String idKind) {
        if (isUnresolved(id)) {
            LOG.warn("PrincipalResolution.unresolved：{}={} 沒有解析到存活的帳號（已關閉、不存在或回應沒有它），收件人帶 0（略過該收件人）",
                    idKind, id);
            return 0L;
        }
        return principals.get(id);
    }

    /**
     * 這個 id 是否<b>沒有</b>真正解析到 principal。
     *
     * <p>三種都算未解析：{@code null}、落在 {@link #unresolvedIds}（server 明說查無在架列）、
     * 或 map 裡根本沒有 entry／值非正（防禦性，正常回應不會發生）。
     */
    public boolean isUnresolved(Long id) {
        if (id == null || unresolvedIds.contains(id)) {
            return true;
        }
        Long principal = principals.get(id);
        return principal == null || principal <= 0L;
    }

    /**
     * 閘門用：取出真正解析到的 principal，未解析即拋。
     *
     * @param idKind 這個 id 的語意（{@code "store_id"} / {@code "buyer_id"}），只進錯誤訊息與 log
     * @throws UnresolvedPrincipalException 未解析
     */
    public long requireResolved(Long id, String idKind) {
        if (isUnresolved(id)) {
            throw new UnresolvedPrincipalException(idKind, List.of(id == null ? 0L : id));
        }
        return principals.get(id);
    }

    /**
     * 閘門用：整批都必須真正解析到，任一個未解析即拋（<b>拋之前不做任何副作用</b>，
     * 呼叫端據此把整個操作擋在副作用之前）。
     *
     * @return 僅含 {@code ids} 的 principal map（不回吐整張 map，避免呼叫端誤查未驗證的 id）
     * @throws UnresolvedPrincipalException 任一 id 未解析，例外帶著全部未解析的 id
     */
    public Map<Long, Long> requireAllResolved(Collection<Long> ids, String idKind) {
        if (ids == null || ids.isEmpty()) {
            return Map.of();
        }
        List<Long> unresolved = ids.stream()
                .filter(this::isUnresolved)
                .map(id -> id == null ? 0L : id) // null 也算未解析，但不能進 List.copyOf
                .distinct()
                .toList();
        if (!unresolved.isEmpty()) {
            throw new UnresolvedPrincipalException(idKind, unresolved);
        }
        Map<Long, Long> resolved = new LinkedHashMap<>();
        for (Long id : ids) {
            resolved.put(id, principals.get(id));
        }
        return Collections.unmodifiableMap(resolved);
    }
}
