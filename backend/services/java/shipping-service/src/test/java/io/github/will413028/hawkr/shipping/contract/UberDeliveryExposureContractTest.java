package io.github.will413028.hawkr.shipping.contract;

import io.github.will413028.hawkr.shipping.grpc.AdminUberDeliveryDetail;
import io.github.will413028.hawkr.shipping.grpc.SellerUberDeliveryDetail;
import io.github.will413028.hawkr.shipping.grpc.UberDeliverySummary;
import io.github.will413028.hawkr.shipping.grpc.UberPodArtifactView;
import com.google.protobuf.Descriptors;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Uber 配送對外契約的<b>外露面</b>釘樁（66 §5.1.5／§5.1.7／§8.6）。
 *
 * <h2>它擋的是什麼</h2>
 * 兩個值一旦離開後端就收不回來，而兩者<b>都不是</b>「個資」那一類——它們是<b>憑證</b>
 * （§10.3 的判準：憑證問「拿到它能做什麼」）：
 * <ul>
 *   <li>外送員<b>明碼</b>電話：Uber 給的是可撥打的臨時號碼，原值只在請求生命週期內透傳，
 *       不落庫不落 log。契約上只能有 {@code courier_phone_masked}。</li>
 *   <li>POD 的 {@code pincode_value}：持有它即可領取包裹。判斷「POD 在不在」不需要它。</li>
 * </ul>
 *
 * <h2>為什麼釘在 proto descriptor 而不是 Java 欄位或原始碼字串</h2>
 * descriptor 是<b>對外 wire 契約本身</b>：Gateway 的 DTO、前端拿到的 JSON、gRPC 的每個
 * consumer 都從它長出來。釘原始碼字串只擋得住「這一份檔案」，釘 descriptor 擋的是
 * 「任何人在任何地方把明碼欄位加進契約」。
 *
 * <p>⚠️ 每條禁令都配一條<b>存在性</b>斷言：少了它，欄位被整個刪掉時禁令會恆真通過——
 * 而「遮罩欄位消失」與「明碼欄位出現」對使用者是同一件壞事的兩面。
 */
class UberDeliveryExposureContractTest {

    private static final List<Descriptors.Descriptor> DETAIL_MESSAGES = List.of(
            SellerUberDeliveryDetail.getDescriptor(),
            AdminUberDeliveryDetail.getDescriptor());

    @Test
    void detailMessagesExposeOnlyTheMaskedCourierPhone() {
        for (Descriptors.Descriptor message : DETAIL_MESSAGES) {
            boolean sawMasked = false;
            for (Descriptors.FieldDescriptor f : message.getFields()) {
                String name = f.getName();
                if (!name.contains("phone")) {
                    continue;
                }
                assertTrue(name.endsWith("_masked"),
                        message.getName() + "." + name + " 是明碼電話欄位。"
                                + "Uber 的外送員號碼是可撥打的臨時號碼，原值不落庫也不得進契約"
                                + "（66 §5.1.5／§8.6）——要顯示就顯示遮罩值。");
                sawMasked = true;
            }
            assertTrue(sawMasked,
                    message.getName() + " 連 courier_phone_masked 都沒有了。"
                            + "本測試若沒有這條存在性斷言，欄位被刪掉時上面那條禁令會恆真通過。");
        }
    }

    @Test
    void podArtifactViewNeverCarriesThePincode() {
        Descriptors.Descriptor pod = UberPodArtifactView.getDescriptor();
        for (Descriptors.FieldDescriptor f : pod.getFields()) {
            assertFalse(f.getName().contains("pincode"),
                    "UberPodArtifactView." + f.getName() + " 外露了 POD 的 PIN——"
                            + "那是「持有即可領取包裹」的憑證，而判斷 POD 在不在不需要它。");
        }
        // 存在性對照：本 message 仍然是「回報 POD 狀態」的那一個，禁令才有意義。
        assertNotNull(pod.findFieldByName("artifact_type"),
                "UberPodArtifactView 不再回報 artifact_type——本測試已經指不到對的東西了");
        assertNotNull(pod.findFieldByName("object_key"),
                "UberPodArtifactView 不再回報 object_key——POD 這一格等於沒有出口");
    }

    @Test
    void summaryKeepsNormalizedAndRawProviderStatusAsSeparateFields() {
        // W6-4 明列 normalized status 與 raw provider status 要能分開查；
        // 合併成一欄會讓「未知 provider 值 fail-closed」（INV-U7）失去意義。
        Descriptors.Descriptor summary = UberDeliverySummary.getDescriptor();
        assertNotNull(summary.findFieldByName("status"), "normalized 狀態欄位不見了");
        assertNotNull(summary.findFieldByName("provider_status"), "raw provider 狀態欄位不見了");
    }

    @Test
    void summaryCarriesTheAddressableKeyForRowsWithoutProviderId() {
        // delivery_id 在建單失敗／結果未知時為空；沒有 delivery_no 的話，Admin 點不開的
        // 正好是他最需要點開的那幾筆。
        Descriptors.Descriptor summary = UberDeliverySummary.getDescriptor();
        assertNotNull(summary.findFieldByName("delivery_no"),
                "少了可定址鍵 delivery_no——建單失敗的單將無法從列表打開");
        assertNotNull(summary.findFieldByName("delivery_id"),
                "少了 provider reference（W6-3 明列的一格）");
    }
}
