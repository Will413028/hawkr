package io.github.will413028.hawkr.common.grpc;

import java.util.List;

/**
 * 身分反解（{@code store_id} / {@code buyer_id} → 登入 {@code account_id}）查無在架資料。
 *
 * <p>與「RPC 失敗」是兩回事：RPC 成功、{@code code=0}，只是這些 id 在 seller/buyer
 * 服務沒有對應的在架列（軟刪除的 stint 或根本不存在），所以 {@code principals} 裡的值是
 * 未經驗證的 echo-back。閘門類呼叫端拿到本例外一律 fail-closed：拒絕該操作、記 log、
 * 轉成自己服務的業務錯誤碼回覆，<b>不得</b>拿 echo-back 值繼續做身分比對。
 *
 * <p>本例外刻意不綁任何 {@code GrpcResponseCode}：同一個原因在不同 surface 上要對使用者
 * 說的話不同（購物車說「商品不存在或已下架」、賣家端說「找不到您的商家資料」），
 * 碼的選擇留在各服務的呼叫端。
 */
public class UnresolvedPrincipalException extends RuntimeException {

    private final String idKind;
    private final List<Long> unresolvedIds;

    public UnresolvedPrincipalException(String idKind, List<Long> unresolvedIds) {
        super("principal 反解失敗：" + idKind + "=" + unresolvedIds
                + " 查無在架資料（軟刪除或不存在），其解析結果是未經驗證的 echo-back，不可用於身分比對");
        this.idKind = idKind;
        this.unresolvedIds = List.copyOf(unresolvedIds);
    }

    /** 這批 id 的語意（{@code "store_id"} / {@code "buyer_id"}）。 */
    public String idKind() {
        return idKind;
    }

    /** 未解析的 id（全部，不只第一個）。 */
    public List<Long> unresolvedIds() {
        return unresolvedIds;
    }
}
