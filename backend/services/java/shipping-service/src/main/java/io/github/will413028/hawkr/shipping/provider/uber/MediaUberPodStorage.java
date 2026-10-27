package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.media.grpc.ServerUploadSecureAndFinalizeResponse;
import io.github.will413028.hawkr.shipping.client.MediaGrpcClient;

/**
 * {@link UberPodStorage} 的產線實作：走 media 的
 * {@code ServerUploadSecureAndFinalize}（{@code secure/} 前綴）。
 *
 * <h2>🔴 為什麼是 secure 而不是沿用既有的 {@code serverUploadAndFinalize}</h2>
 * 那一支硬寫 {@code public/} 前綴。而 POD 的內容是<b>收件地址與門口實景</b>，
 * 用 public 上傳<b>沒有任何訊號</b>（66 陷阱 A6）：上傳成功、圖片看得到、一切正常，
 * 只是全世界拿到 objectKey 的人都看得到。
 * ⇒ 本類別在<b>寫回之前</b>親自驗一次前綴（{@link #requireSecureObjectKey}），
 * 那是 66 §13.3 T10 的執行期版本。
 *
 * <h2>ref type 引用 CodeMeta 生成的 enum 常數，不硬寫號碼</h2>
 * {@code AssetReferenceType.DELIVERY_POD}（{@code 7121}）由 {@code generate.sh} 從
 * {@code docs/代碼管理（CodeMeta）/業務類型碼.md} 產出，碼表 description 逐字指定
 * {@code secure/delivery_pod/} 前綴。
 *
 * <p>🔴 <b>直接引用常數而不是用字串反查</b>：該值若被移除或改名，
 * 本模組<b>編譯失敗</b>——那是最便宜、最不會被忽略的訊號。
 * 三種曾被考慮過的替代方案全都是靜默的：
 * <ul>
 *   <li>❌ 硬寫字面 {@code "7121"}（10 檔 §8：禁止硬編碼；下次 generate 換號段就對不上）</li>
 *   <li>❌ 退回 {@code SHIPPING(7104)}（它是 {@code public/} 語意的資產類型，直接踩 A6）</li>
 *   <li>❌ 靜默跳過上傳（POD 不見了，而 {@code fetchPod} 回報成功）</li>
 * </ul>
 */
public class MediaUberPodStorage implements UberPodStorage {

    /**
     * POD 的資產引用類型（66 §0.6 新增的 {@code AssetReferenceType} 值）。
     *
     * <p>🔴 <b>不得改用其他既有值</b>：{@code SHIPPING(7104)} 走 {@code public/} 語意，
     * 換過去等於把收件地址與門口實景放到公開資產池（陷阱 A6，而且沒有任何訊號）。
     */
    static final AssetReferenceType POD_REF_TYPE = AssetReferenceType.DELIVERY_POD;

    /** 機敏資產的前綴。POD 落在 {@code secure/delivery_pod/}（碼表 description 逐字）。 */
    static final String SECURE_PREFIX = "secure/";

    private final MediaGrpcClient mediaGrpcClient;

    public MediaUberPodStorage(MediaGrpcClient mediaGrpcClient) {
        this.mediaGrpcClient = mediaGrpcClient;
    }

    @Override
    public String store(String deliveryId, byte[] content, String contentType) {
        ServerUploadSecureAndFinalizeResponse resp = mediaGrpcClient.serverUploadSecureAndFinalize(
                content, filenameFor(deliveryId, contentType), contentType,
                POD_REF_TYPE.value(), deliveryId);
        return requireSecureObjectKey(resp.getObjectKey());
    }

    /**
     * 🔴 前綴守衛：不是 {@code secure/} 就<b>拒絕落庫</b>（陷阱 A6 的執行期防線）。
     *
     * @throws IllegalStateException 前綴不對。訊息只描述前綴，不含完整 objectKey
     *                               ——它本身是憑證等級的值（66 §8.6）
     */
    static String requireSecureObjectKey(String objectKey) {
        if (objectKey == null || !objectKey.startsWith(SECURE_PREFIX)) {
            throw new IllegalStateException(
                    "POD 轉存回來的 objectKey 不是 " + SECURE_PREFIX + " 前綴，拒絕落庫："
                            + describePrefix(objectKey));
        }
        return objectKey;
    }

    private static String filenameFor(String deliveryId, String contentType) {
        String ext = contentType != null && contentType.contains("png") ? ".png" : ".jpg";
        return "uber-pod-" + deliveryId + ext;
    }

    /** 只描述前綴，不把完整 objectKey 寫進例外訊息（它本身是憑證等級的值，66 §8.6）。 */
    private static String describePrefix(String objectKey) {
        if (objectKey == null) {
            return "<null>";
        }
        int slash = objectKey.indexOf('/');
        return slash > 0 ? objectKey.substring(0, slash + 1) : "<no-prefix>";
    }
}
