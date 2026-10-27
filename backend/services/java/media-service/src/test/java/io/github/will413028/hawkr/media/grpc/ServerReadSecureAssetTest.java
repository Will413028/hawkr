package io.github.will413028.hawkr.media.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.media.converter.MediaAssetConverter;
import io.github.will413028.hawkr.media.crypto.AesStreamCipher;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.repository.AssetReferenceRepository;
import io.github.will413028.hawkr.media.repository.MediaAssetRepository;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import io.github.will413028.hawkr.media.service.AssetReferenceService;
import io.github.will413028.hawkr.media.service.ImportAssetService;
import io.github.will413028.hawkr.media.service.MediaAssetService;
import io.github.will413028.hawkr.media.service.MediaUploadValidator;
import io.github.will413028.hawkr.media.service.StagedTokenService;
import io.github.will413028.hawkr.media.service.UploadSessionService;
import io.github.will413028.hawkr.media.storage.StorageObjectStat;
import io.github.will413028.hawkr.media.storage.StorageService;
import io.grpc.Context;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.transaction.PlatformTransactionManager;

import javax.crypto.CipherInputStream;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Base64;
import java.util.List;
import java.util.concurrent.atomic.AtomicReference;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * {@code ServerReadSecureAsset}（58 §8.1.6，issue #3008）。
 *
 * <p>三件事在守：
 * <ol>
 *   <li><b>internal-only</b>：它把一份機敏檔的明文整個吐出來，沒有任何使用者授權檢查
 *       （呼叫端不是使用者）。少了 {@code isInternalCall()} 這道閘門，任何持有 JWT 的人
 *       只要猜得到三元組就能讀走別人的發票。</li>
 *   <li><b>三元組全等才放行</b>：{@code object_key} 會出現在呼叫端的 DB 與 log 裡，
 *       單獨拿它當憑證等於「知道路徑就能讀」。</li>
 *   <li><b>越權與不存在逐欄相同</b>：訊息或碼有任何差異，這支 RPC 就變成一個可枚舉的
 *       oracle——攻擊者拿一個 object_key 掃 ref_id，靠回應差異就能把歸屬問出來。</li>
 * </ol>
 *
 * <p><b>授權述詞不 mock。</b> {@link AssetReferenceService} 與 {@link MediaAssetService}
 * 都用真的，替身只放到 repository 與儲存層；而 repository 替身<b>只認唯一正解</b>
 * （Mockito 未匹配的 {@code boolean} 回 false、物件回 null），所以「填錯任何一欄也會通過」
 * 這種假綠在這裡不成立（{@link #referenceStubOnlyAcceptsExactTriple} 直接驗這件事）。
 * 加解密用真的 {@link AesStreamCipher} 跑完整往返，不是回一個寫死的 byte 陣列。
 */
class ServerReadSecureAssetTest {

    private static final String OBJECT_KEY = "secure/invoice_document/20260902/1a2b/INV-AB12345678.pdf";
    private static final String MISSING_KEY = "secure/invoice_document/20260902/zzzz/none.pdf";
    private static final String REF_TYPE = AssetReferenceType.INVOICE_DOCUMENT.value();
    private static final String REF_ID = "2649605557";
    private static final long ASSET_ID = 2685914682L;
    private static final byte[] PDF = "%PDF-1.4 fake invoice bytes".getBytes(StandardCharsets.UTF_8);

    /** 32 bytes → 合法的 AES-256 主密鑰，讓 AesStreamCipher 真的能加解密。 */
    private static final String MASTER_KEY = Base64.getEncoder()
            .encodeToString("0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.UTF_8));

    private MediaAssetRepository mediaAssetRepository;
    private AssetReferenceRepository assetReferenceRepository;
    private StorageService storageService;
    private AesStreamCipher cipher;
    private MediaGrpcService service;

    @BeforeEach
    void setUp() {
        mediaAssetRepository = mock(MediaAssetRepository.class);
        assetReferenceRepository = mock(AssetReferenceRepository.class);
        storageService = mock(StorageService.class);
        cipher = new AesStreamCipher(MASTER_KEY);

        AssetReferenceService assetReferenceService =
                new AssetReferenceService(assetReferenceRepository, mediaAssetRepository);
        MediaAssetService mediaAssetService = new MediaAssetService(
                mediaAssetRepository,
                mock(UploadSessionRepository.class),
                mock(StagedTokenService.class),
                storageService,
                mock(MediaAssetConverter.class),
                mock(MediaUploadValidator.class),
                cipher,
                assetReferenceService,
                mock(PlatformTransactionManager.class));

        service = new MediaGrpcService(
                mediaAssetService,
                assetReferenceService,
                mock(MediaAssetConverter.class),
                mock(UploadSessionService.class),
                mock(ImportAssetService.class));
    }

    // ==================== fixtures ====================

    private static MediaAssetEntity asset(boolean secure, String encryptionKey) {
        MediaAssetEntity a = new MediaAssetEntity();
        a.setId(ASSET_ID);
        a.setObjectKey(OBJECT_KEY);
        a.setRefType(REF_TYPE);
        a.setIsSecure(secure);
        a.setMimeType("application/pdf");
        a.setEncryptionKey(encryptionKey);
        // size_bytes 刻意設成錯的：它來自 client 上傳前的宣告值，讀取路徑不得拿它當長度基準
        a.setSizeBytes(999_999L);
        return a;
    }

    /**
     * 明文落庫（#3024 之前 {@code serverUploadSecureAndFinalize} 寫出來的形態：
     * {@code encryption_key} 為 NULL）。
     *
     * <p>⚠️ 該支自 2026-09-02 起會加密，但 dev/prod 上**既有**的資產仍是這個形態
     * ⇒ 這一格是「不要把歷史發票鎖死」的陰性對照，不得因為上傳已修好就刪掉。
     */
    private void givenPlaintextObject() {
        when(mediaAssetRepository.selectByObjectKey(OBJECT_KEY)).thenReturn(asset(true, null));
        when(storageService.getObjectStream(OBJECT_KEY))
                .thenAnswer(i -> new ByteArrayInputStream(PDF));
        when(storageService.statObject(OBJECT_KEY))
                .thenReturn(new StorageObjectStat(PDF.length, "application/pdf"));
    }

    /** 唯一正解才回 true——填錯任何一欄，Mockito 預設回 false。 */
    private void givenReference() {
        when(assetReferenceRepository.existsReference(ASSET_ID, REF_TYPE, REF_ID)).thenReturn(true);
    }

    private record Result(ServerReadSecureAssetResponse response, Throwable error) {
    }

    private Result call(boolean internal, String objectKey, String refType, String refId) {
        AtomicReference<ServerReadSecureAssetResponse> resp = new AtomicReference<>();
        List<Throwable> errors = new ArrayList<>();
        StreamObserver<ServerReadSecureAssetResponse> observer = new StreamObserver<>() {
            @Override
            public void onNext(ServerReadSecureAssetResponse value) {
                resp.set(value);
            }

            @Override
            public void onError(Throwable t) {
                errors.add(t);
            }

            @Override
            public void onCompleted() {
            }
        };

        Context ctx = Context.current().withValue(GrpcAuthContext.INTERNAL_CALL, internal);
        ctx.run(() -> service.serverReadSecureAsset(
                ServerReadSecureAssetRequest.newBuilder()
                        .setObjectKey(objectKey)
                        .setRefType(refType)
                        .setRefId(refId)
                        .build(),
                observer));

        return new Result(resp.get(), errors.isEmpty() ? null : errors.get(0));
    }

    private Result callInternal(String objectKey, String refType, String refId) {
        return call(true, objectKey, refType, refId);
    }

    // ==================== 測試 ====================

    @Test
    @DisplayName("三元組相符 → 回原始 bytes（明文落庫，encryption_key 為 NULL）")
    void returnsBytesWhenTripleMatches() {
        givenPlaintextObject();
        givenReference();

        Result r = callInternal(OBJECT_KEY, REF_TYPE, REF_ID);

        assertThat(r.error()).isNull();
        assertThat(r.response().getCode()).isEqualTo(GrpcResponseCode.SUCCESS.code());
        assertThat(r.response().getFileData().toByteArray()).isEqualTo(PDF);
        assertThat(r.response().getContentType()).isEqualTo("application/pdf");
        assertThat(r.response().getFilename()).isEqualTo("INV-AB12345678.pdf");
    }

    @Test
    @DisplayName("#3149：original_filename 存在時優先於 objectKey 尾段（兩值刻意不同）")
    void filenamePrefersOriginalOverObjectKeyTail() {
        MediaAssetEntity a = asset(true, null);
        // 🔴 兩個值必須不同：objectKey 尾段是 "INV-AB12345678.pdf"，原始檔名是中文。
        // 相同的話「永遠回尾段」的實作也會全綠——ADM 人工解鎖證據的下載檔名正是走這裡。
        a.setOriginalFilename("發票-ZA10068421.pdf");
        when(mediaAssetRepository.selectByObjectKey(OBJECT_KEY)).thenReturn(a);
        when(storageService.getObjectStream(OBJECT_KEY))
                .thenAnswer(i -> new ByteArrayInputStream(PDF));
        when(storageService.statObject(OBJECT_KEY))
                .thenReturn(new StorageObjectStat(PDF.length, "application/pdf"));
        givenReference();

        Result r = callInternal(OBJECT_KEY, REF_TYPE, REF_ID);

        assertThat(r.response().getFilename()).isEqualTo("發票-ZA10068421.pdf");
        assertThat(r.response().getFilename()).isNotEqualTo("INV-AB12345678.pdf");
    }

    @Test
    @DisplayName("#3149：per-attempt key 的舊資料回退時剝掉 .attempt-{uuid}（#2759）")
    void filenameFallbackStripsAttemptSuffixForLegacyRows() {
        String attemptKey = "secure/certification/20260830/"
                + "3f2504e0-4f89-11d3-9a0c-0305e82c3301.pdf"
                + ".attempt-6ba7b810-9dad-11d1-80b4-00c04fd430c8";
        MediaAssetEntity a = asset(true, null);
        a.setObjectKey(attemptKey);
        a.setOriginalFilename(null); // V9 之前的列
        when(mediaAssetRepository.selectByObjectKey(attemptKey)).thenReturn(a);
        when(storageService.getObjectStream(attemptKey))
                .thenAnswer(i -> new ByteArrayInputStream(PDF));
        when(storageService.statObject(attemptKey))
                .thenReturn(new StorageObjectStat(PDF.length, "application/pdf"));
        when(assetReferenceRepository.existsReference(ASSET_ID, REF_TYPE, REF_ID)).thenReturn(true);

        Result r = callInternal(attemptKey, REF_TYPE, REF_ID);

        // 剝完仍是一串 uuid（那正是 #3149 的重點：回退只是讓副檔名恢復正確）
        assertThat(r.response().getFilename())
                .isEqualTo("3f2504e0-4f89-11d3-9a0c-0305e82c3301.pdf");
    }

    @Test
    @DisplayName("加密落庫 → 走真實 AES-GCM 往返後回明文（不是回密文，也不是靜默截斷）")
    void decryptsEncryptedObject() {
        String wrappedKey = cipher.generateWrappedKey();
        byte[] ciphertext = encrypt(PDF, wrappedKey);

        when(mediaAssetRepository.selectByObjectKey(OBJECT_KEY)).thenReturn(asset(true, wrappedKey));
        when(storageService.getObjectStream(OBJECT_KEY))
                .thenAnswer(i -> new ByteArrayInputStream(ciphertext));
        when(storageService.statObject(OBJECT_KEY))
                .thenReturn(new StorageObjectStat(ciphertext.length, "application/octet-stream"));
        givenReference();

        Result r = callInternal(OBJECT_KEY, REF_TYPE, REF_ID);

        assertThat(r.response().getCode()).isEqualTo(GrpcResponseCode.SUCCESS.code());
        assertThat(r.response().getFileData().toByteArray()).isEqualTo(PDF);
        // 密文比明文多一個 16 bytes 的 GCM auth tag：斷言回的是明文不是密文
        assertThat(ciphertext.length).isEqualTo(PDF.length + AesStreamCipher.GCM_TAG_BYTES);
    }

    @Test
    @DisplayName("ref_id 不符 → 回應與「資產不存在」逐欄相同（有任何差異就是可枚舉的 oracle）")
    void mismatchedRefIdIsIndistinguishableFromMissing() {
        givenPlaintextObject();
        givenReference();

        // (a) 三元組不符：object_key 存在，ref_id 是別人的
        Result denied = callInternal(OBJECT_KEY, REF_TYPE, "9999999999");
        // (b) 資產不存在：selectByObjectKey 未 stub 該 key → null
        Result missing = callInternal(MISSING_KEY, REF_TYPE, REF_ID);

        assertThat(denied.error()).isNull();
        assertThat(missing.error()).isNull();
        // proto 訊息的相等是逐欄比對：code / message / file_data / content_type / filename 全部要一樣
        assertThat(denied.response()).isEqualTo(missing.response());
        assertThat(denied.response().getCode()).isEqualTo(GrpcResponseCode.NOT_FOUND.code());
        assertThat(denied.response().getFileData().isEmpty()).isTrue();
    }

    @Test
    @DisplayName("ref_type 不符 → 同樣與「資產不存在」逐欄相同")
    void mismatchedRefTypeIsIndistinguishableFromMissing() {
        givenPlaintextObject();
        givenReference();

        Result denied = callInternal(OBJECT_KEY, AssetReferenceType.OTHER.value(), REF_ID);
        Result missing = callInternal(MISSING_KEY, REF_TYPE, REF_ID);

        assertThat(denied.response()).isEqualTo(missing.response());
    }

    @Test
    @DisplayName("非內部呼叫一律 PERMISSION_DENIED，且不得吐出任何 bytes")
    void rejectsNonInternalCallers() {
        givenPlaintextObject();
        givenReference();

        Result r = call(false, OBJECT_KEY, REF_TYPE, REF_ID);

        assertThat(r.response()).as("被拒絕時不得吐出任何 data").isNull();
        assertThat(r.error()).isInstanceOf(StatusRuntimeException.class);
        assertThat(Status.fromThrowable(r.error()).getCode()).isEqualTo(Status.Code.PERMISSION_DENIED);
    }

    @Test
    @DisplayName("三元組任一欄為空 → INVALID_PARAM（沒有述詞可驗就不放行，不 fallback）")
    void rejectsEmptyTripleFields() {
        givenPlaintextObject();
        givenReference();

        assertThat(callInternal("", REF_TYPE, REF_ID).response().getCode())
                .isEqualTo(GrpcResponseCode.INVALID_PARAM.code());
        assertThat(callInternal(OBJECT_KEY, "", REF_ID).response().getCode())
                .isEqualTo(GrpcResponseCode.INVALID_PARAM.code());
        assertThat(callInternal(OBJECT_KEY, REF_TYPE, "").response().getCode())
                .isEqualTo(GrpcResponseCode.INVALID_PARAM.code());
    }

    @Test
    @DisplayName("非機敏資產不從這支出（公開資產走 URL；此路徑不洩漏授權述詞）")
    void rejectsNonSecureAsset() {
        when(mediaAssetRepository.selectByObjectKey(OBJECT_KEY)).thenReturn(asset(false, null));
        givenReference();

        Result r = callInternal(OBJECT_KEY, REF_TYPE, REF_ID);

        assertThat(r.response().getCode()).isEqualTo(GrpcResponseCode.INVALID_PARAM.code());
        assertThat(r.response().getFileData().isEmpty()).isTrue();
    }

    @Test
    @DisplayName("讀回的長度與儲存端不符 → INTERNAL，不得把截斷內容當成功回出去")
    void rejectsTruncatedRead() {
        when(mediaAssetRepository.selectByObjectKey(OBJECT_KEY)).thenReturn(asset(true, null));
        when(storageService.getObjectStream(OBJECT_KEY))
                .thenAnswer(i -> new ByteArrayInputStream(new byte[]{0x25, 0x50}));
        when(storageService.statObject(OBJECT_KEY))
                .thenReturn(new StorageObjectStat(PDF.length, "application/pdf"));
        givenReference();

        Result r = callInternal(OBJECT_KEY, REF_TYPE, REF_ID);

        assertThat(r.response().getCode()).isEqualTo(GrpcResponseCode.INTERNAL_ERROR.code());
        assertThat(r.response().getFileData().isEmpty()).isTrue();
    }

    @Test
    @DisplayName("替身鑑別力自檢：未 stub 的三元組一律回 false（否則上面每一條越權測試都是假綠）")
    void referenceStubOnlyAcceptsExactTriple() {
        givenReference();

        assertThat(assetReferenceRepository.existsReference(ASSET_ID, REF_TYPE, REF_ID)).isTrue();
        assertThat(assetReferenceRepository.existsReference(ASSET_ID, REF_TYPE, "9999999999")).isFalse();
        assertThat(assetReferenceRepository.existsReference(ASSET_ID + 1, REF_TYPE, REF_ID)).isFalse();
        assertThat(assetReferenceRepository.existsReference(ASSET_ID, AssetReferenceType.OTHER.value(), REF_ID))
                .isFalse();
        assertThat(mediaAssetRepository.selectByObjectKey(MISSING_KEY)).isNull();
    }

    // ==================== helper ====================

    private byte[] encrypt(byte[] plain, String wrappedKey) {
        try (InputStream in = new ByteArrayInputStream(plain);
             CipherInputStream encrypting = cipher.encryptingStream(in, wrappedKey)) {
            ByteArrayOutputStream out = new ByteArrayOutputStream();
            encrypting.transferTo(out);
            return out.toByteArray();
        } catch (Exception e) {
            throw new IllegalStateException("測試 fixture 加密失敗", e);
        }
    }
}
