package io.github.will413028.hawkr.payment.adapter.grpc;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.github.will413028.hawkr.payment.grpc.ProposeResponsibilityResolutionForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.ReviewResponsibilityResolutionForAdminRequest;
import com.google.protobuf.Descriptors.Descriptor;
import com.google.protobuf.Descriptors.FieldDescriptor;
import java.util.List;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

/**
 * WP2B 的 <b>adapter 邊界</b>契約：ADM 能送進來的欄位有哪些。
 *
 * <p>🔴 <b>它與 {@code ctx3_finalizeCommandHasNoEventFacts} 是<u>兩道不同的門</u>，缺一不可。</b>
 * 那一支釘的是 <b>domain</b>（{@code FinalizeCommand} 的 record component）；本檔釘的是
 * <b>wire</b>（proto request 的 field）。
 *
 * <p>兩者為什麼都要有：proto 多長一個 {@code responsibility_cause} 欄位<b>不會</b>讓
 * {@code FinalizeCommand} 多一個元件——adapter 可以把它讀出來、拿去做別的事
 * （例如「先驗一下 client 說的成因對不對」，而那一驗就會有人把它改成「以 client 的為準」）。
 * 反過來也一樣。⇒ 只釘一邊，另一邊就是無人看守的入口。
 *
 * <p>⚠️ <b>本檔不依賴 server 實作</b>：它讀的是 protobuf descriptor，也就是**契約本身**。
 * 就算有人把 adapter 整支改寫，只要 proto 沒有長出那些欄位，這道保證仍然成立。
 */
@DisplayName("WP2B Admin 責任判定端點的 wire 契約")
class FulfillmentResponsibilityGrpcContractTest {

    private static List<String> fieldNames(Descriptor d) {
        return d.getFields().stream().map(FieldDescriptor::getName).toList();
    }

    // =========================================================================
    // A-4 maker
    // =========================================================================

    @Test
    @DisplayName("🔴 A-4 request 不得出現任何 client-authored 的金額或身分欄位")
    void proposeRequestHasNoClientAuthoredFinancialsOrIdentity() {
        List<String> fields = fieldNames(
                ProposeResponsibilityResolutionForAdminRequest.getDescriptor());

        // 對照組：真的讀到了欄位（descriptor 抓錯類別時本行會先炸）。
        assertFalse(fields.isEmpty(), "量測本身失效：讀到 0 個欄位");
        assertTrue(fields.contains("responsibility_cause"),
                "量測本身失效：連 responsibility_cause 都讀不到 ⇒ descriptor 抓錯類別。實際：" + fields);

        for (String banned : List.of(
                // 影響預覽由 server 依成因 ＋ orders.total_actual_amount 算。
                // client 給金額 ＝ 讓被覆核的數字由被覆核者自己決定。
                "impact_preview", "impact_preview_refund_amount", "refund_amount",
                // 65 原列的兩個維度：在現行商流下沒有 canonical 算法且語意不成立
                // （訂單層沒有平台代收）⇒ 它們不是 0，而是不屬於本 contract。
                // 填 0 會讓覆核者看到「平台與賣家都不受影響」的假結論，比不顯示更糟。
                "platform_fee_delta", "seller_net_delta",
                // 操作者身分取自 GrpcAuthContext。由 request 讀 ＝ 呼叫端可宣稱自己是誰
                // ⇒ chk_fcr_two_person 形同虛設，一個人用兩個 id 就能自問自答。
                "proposed_by_account_id", "actor_account_id", "account_id",
                // 事件 context 由建案當下凍結（V14），提案階段不得重寫。
                "order_no", "shipment_id", "shipping_method", "provider_delivery_ref",
                // 來源服務是 backend internal metadata，不是 ADM 輸入。
                "source_service")) {
            assertFalse(fields.contains(banned),
                    "🔴 A-4 request 出現了 " + banned + "——它必須由 server 決定。實際欄位：" + fields);
        }

        assertEquals(4, fields.size(),
                "A-4 request 應只有 case_no／resolution_type／responsibility_cause／reason，實際："
                        + fields);
    }

    // =========================================================================
    // A-5 checker
    // =========================================================================

    @Test
    @DisplayName("🔴 A-5 request 不得出現責任成因——那是 maker 提案的標的物")
    void reviewRequestCannotCarryACause() {
        List<String> fields = fieldNames(
                ReviewResponsibilityResolutionForAdminRequest.getDescriptor());

        assertFalse(fields.isEmpty(), "量測本身失效：讀到 0 個欄位");
        assertTrue(fields.contains("decision"),
                "量測本身失效：連 decision 都讀不到 ⇒ descriptor 抓錯類別。實際：" + fields);

        for (String banned : List.of(
                // 🔴 本檔最重要的一欄。成因若能由 checker 提供，雙人制保證的就只剩
                //    「有兩個人碰過這張單」，而不是「第二個人批准了第一個人提的那一件事」
                //    ——覆核者可以獨自決定 BUYER_CAUSED（＝買家拿不到退款），
                //    而 chk_fcr_two_person 在帳面上完全成立。
                "responsibility_cause", "resolution_code", "cause",
                // 事件 context 取自建案當下凍結的 case row（V14）。由呼叫端再傳一次
                // ⇒ N17 與 N16 可以帶不同的 order_no，而那是 order-service 35→54 CAS 的錨，
                //   對不上時那道 CAS 永遠 rows=0 而**全鏈路沒有任何錯誤訊息**。
                "order_no", "shipment_id", "shipping_method", "provider_delivery_ref",
                // 金額同 A-4。
                "refund_amount", "impact_preview", "platform_fee_delta", "seller_net_delta",
                // 覆核者身分取自 GrpcAuthContext。
                "reviewed_by_account_id", "actor_account_id", "account_id")) {
            assertFalse(fields.contains(banned),
                    "🔴 A-5 request 出現了 " + banned + "——覆核者能表達的只有「批准或駁回這一筆提案」。"
                            + "實際欄位：" + fields);
        }

        assertEquals(4, fields.size(),
                "A-5 request 應只有 case_no／resolution_id／decision／review_comment，實際：" + fields);
    }

    // =========================================================================
    // 回應面：N16 是否真的排出去了，必須分辨得出來
    // =========================================================================

    @Test
    @DisplayName("A-5 response 必須帶 n16_enqueued —— 重送與首次定案要分得出來")
    void reviewResponseExposesWhetherN16WasEnqueued() {
        List<String> fields = fieldNames(
                io.github.will413028.hawkr.payment.grpc.ReviewResponsibilityResolutionForAdminData.getDescriptor());
        assertTrue(fields.contains("n16_enqueued"),
                "🔴 少了這一欄，ADM 分不出「這次真的定案並發了事件」與「重送、事件早就發過了」。"
                        + "兩者都回成功，但後者若被當成前者，就會有人以為自己剛剛做了一個財務決定。"
                        + "實際欄位：" + fields);
        assertTrue(fields.contains("responsibility_cause"),
                "🔴 定案結果必須回傳**實際寫進 case.resolution_code 的那個成因**——"
                        + "它取自 maker 的提案，覆核者需要看到自己批准的到底是什麼。實際：" + fields);
    }
}
