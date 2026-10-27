package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * POD 轉存的兩道防線（66 陷阱 A6／§13.3 T10）。
 */
@DisplayName("POD 轉存：secure/ 前綴與 ref type")
class MediaUberPodStorageTest {

    @Test
    @DisplayName("🔴 public/ 前綴的 objectKey 一律拒絕落庫（A6 沒有任何其他訊號）")
    void publicPrefixIsRejected() {
        // 用 public 上傳的症狀是「什麼事都沒發生」：上傳成功、圖片看得到、
        // 只是收件地址與門口實景可被任何拿到 objectKey 的人取得。
        assertThatThrownBy(() -> MediaUberPodStorage.requireSecureObjectKey("public/uber-pod-1.jpg"))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("secure/");
    }

    @Test
    @DisplayName("🔴 例外訊息只描述前綴，不含完整 objectKey（它本身是憑證等級的值）")
    void errorMessageDoesNotLeakTheObjectKey() {
        assertThatThrownBy(() -> MediaUberPodStorage.requireSecureObjectKey("public/very-secret-key.jpg"))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageNotContaining("very-secret-key");
    }

    @Test
    @DisplayName("null objectKey 也拒絕（不得當成「這次沒有 POD」而放行）")
    void nullObjectKeyIsRejected() {
        assertThatThrownBy(() -> MediaUberPodStorage.requireSecureObjectKey(null))
                .isInstanceOf(IllegalStateException.class);
    }

    @Test
    @DisplayName("secure/ 前綴通過（陽性對照：沒有它，把守衛寫成恆拋也會綠）")
    void securePrefixPasses() {
        assertThat(MediaUberPodStorage.requireSecureObjectKey("secure/delivery_pod/abc.jpg"))
                .isEqualTo("secure/delivery_pod/abc.jpg");
    }

    /**
     * ref type。
     *
     * <p>🔴 這一條的鑑別力在於：<b>它不是在驗「常數等於自己」</b>。
     * 斷言的是那個常數的 {@code description} 確實把 POD 指到 {@code secure/delivery_pod/}，
     * 以及它<b>不是</b> {@code SHIPPING(7104)}——後者走 {@code public/} 語意，
     * 換過去會把收件地址與門口實景放進公開資產池，而那個錯誤<b>沒有任何執行期訊號</b>（陷阱 A6）。
     */
    @Test
    @DisplayName("🔴 ref type 是 DELIVERY_POD，且它的碼表語意確實是 secure/delivery_pod/")
    void refTypeIsDeliveryPodAndItsSemanticIsSecure() {
        assertThat(MediaUberPodStorage.POD_REF_TYPE.code()).isEqualTo("DELIVERY_POD");
        assertThat(MediaUberPodStorage.POD_REF_TYPE)
                .as("SHIPPING(7104) 是 public/ 語意的資產類型，換過去直接踩 A6 且無訊號")
                .isNotEqualTo(AssetReferenceType.SHIPPING);
        assertThat(MediaUberPodStorage.POD_REF_TYPE.description())
                .as("碼表 description 是這個值與 secure/ 前綴之間唯一的書面連結")
                .contains(MediaUberPodStorage.SECURE_PREFIX + "delivery_pod/");
    }
}
