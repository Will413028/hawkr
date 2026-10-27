package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.media.converter.MediaAssetConverter;
import io.github.will413028.hawkr.media.crypto.AesStreamCipher;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.media.repository.MediaAssetRepository;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import io.github.will413028.hawkr.media.storage.StorageObjectStat;
import io.github.will413028.hawkr.media.storage.StorageService;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.SimpleTransactionStatus;

import java.io.ByteArrayInputStream;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.concurrent.atomic.AtomicReference;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code serverUploadSecureAndFinalize} 的加密（#3024）。
 *
 * <p><b>本票的缺陷</b>：這一支把明文直接送進 R2，而 asset 標成 {@code is_secure=true}、
 * {@code encryption_key} 為 NULL。它唯一的使用者是發票／收據 PDF（{@code ref_type=7120}）
 * ——帶著完整買受人姓名、統編與金額的稅務憑證。
 *
 * <p><b>加解密用真的 {@link AesStreamCipher} 跑完整往返</b>，不是驗「有沒有呼叫 cipher」：
 * 後者對「呼叫了但把明文也一起寫出去」完全無感，而那正是本票要防的形態。
 * ⇒ 斷言的對象是**真的送進 storage 的那串 bytes**。
 */
class ServerUploadSecureEncryptionTest {

    private static final String REF_TYPE = AssetReferenceType.INVOICE_DOCUMENT.value();
    private static final String REF_ID = "7499573277841424384";
    private static final byte[] PDF = "%PDF-1.4 買受人 陳小明 統編 77777177 金額 1200".getBytes(StandardCharsets.UTF_8);

    /** 32 bytes → 合法的 AES-256 主密鑰。 */
    private static final String MASTER_KEY = Base64.getEncoder()
            .encodeToString("0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.UTF_8));

    @BeforeAll
    static void initTsid() {
        if (!TsidUtil.isInitialized()) {
            TsidUtil.init(100, 0);
        }
    }

    private MediaAssetRepository mediaAssetRepository;
    // 🔴 AssetReferenceService 用替身而不是真的：本測試的對象是**加密**，
    // 而真的那支會去 repository 驗資產存在（linkImages），逼本測試去 stub 一堆
    // 與加密無關的東西。引用建立本身由 MediaAssetServiceTest 的
    // uploadsToSecurePrefixAndLinksReference 守著，不在這裡重複。
    private AssetReferenceService assetReferenceService;
    private StorageService storageService;
    private AtomicReference<byte[]> uploaded;

    private MediaAssetService serviceWith(AesStreamCipher cipher) {
        mediaAssetRepository = mock(MediaAssetRepository.class);
        assetReferenceService = mock(AssetReferenceService.class);
        storageService = mock(StorageService.class);
        uploaded = new AtomicReference<>();

        // 把「真的送進 storage 的位元組」抓下來——本測試所有斷言的對象都是它。
        // uploadStream 回 void ⇒ 用 doAnswer(...).when(...) 而不是 when(...).thenAnswer(...)。
        org.mockito.Mockito.doAnswer(i -> {
            try (InputStream in = i.getArgument(1)) {
                uploaded.set(in.readAllBytes());
            }
            return null;
        }).when(storageService).uploadStream(anyString(), any(), anyString(), anyLong());

        PlatformTransactionManager txm = mock(PlatformTransactionManager.class);
        when(txm.getTransaction(any())).thenReturn(new SimpleTransactionStatus());

        return new MediaAssetService(
                mediaAssetRepository,
                mock(UploadSessionRepository.class),
                mock(StagedTokenService.class),
                storageService,
                mock(MediaAssetConverter.class),
                mock(MediaUploadValidator.class),
                cipher,
                assetReferenceService,
                txm);
    }

    @BeforeEach
    void setUp() {
        // 每個測試自己選 cipher（真的／沒有主密鑰的），serviceWith 負責重建替身。
    }

    // ------------------------------------------------------------------

    @Test
    @DisplayName("🔴 送進 R2 的是密文，不是 PDF 明文")
    void uploadsCiphertextNotPlaintext() {
        MediaAssetService svc = serviceWith(new AesStreamCipher(MASTER_KEY));

        MediaAssetEntity asset = svc.serverUploadSecureAndFinalize(
                PDF, "ZA10068421.pdf", "application/pdf", REF_TYPE, REF_ID);

        byte[] stored = uploaded.get();
        assertThat(stored).as("storage 應該收到東西").isNotNull();

        // ① 逐位元不同 —— 這一條就是本票。
        assertThat(stored).as("送進 R2 的位元組與明文相同 ⇒ 根本沒加密").isNotEqualTo(PDF);
        // ② 連 magic bytes 都不能留：%PDF- 開頭等於任何拿到物件的人都知道這是什麼。
        assertThat(new String(stored, 0, Math.min(5, stored.length), StandardCharsets.ISO_8859_1))
                .as("密文不得以 %%PDF- 開頭").isNotEqualTo("%PDF-");
        // ③ 長度 = 明文 + GCM auth tag。少了 tag 表示走的不是 GCM。
        assertThat(stored.length).isEqualTo(PDF.length + AesStreamCipher.GCM_TAG_BYTES);
        // ④ 金鑰必須落庫，否則讀回來的路徑會走明文分支、拿到密文當 PDF。
        assertThat(asset.getEncryptionKey()).as("encryption_key 必須非空").isNotBlank();
        assertThat(asset.getIsSecure()).isTrue();
        // ⑤ mime_type 記的是**明文**的型別（呼叫端要的是明文），不是 R2 上那份密文的。
        assertThat(asset.getMimeType()).isEqualTo("application/pdf");
        // ⑥ size_bytes 同樣是明文口徑，與 readAssetBytes 解密後的長度對得上。
        assertThat(asset.getSizeBytes()).isEqualTo((long) PDF.length);
    }

    @Test
    @DisplayName("往返：上傳後用同一顆 cipher 讀回來，拿到的是原本那份 PDF")
    void roundTripsThroughTheRealCipher() {
        AesStreamCipher cipher = new AesStreamCipher(MASTER_KEY);
        MediaAssetService svc = serviceWith(cipher);

        MediaAssetEntity asset = svc.serverUploadSecureAndFinalize(
                PDF, "ZA10068421.pdf", "application/pdf", REF_TYPE, REF_ID);

        byte[] stored = uploaded.get();
        // 讓讀取路徑看到剛剛那份密文與那把金鑰。
        when(mediaAssetRepository.selectByObjectKey(asset.getObjectKey())).thenReturn(asset);
        when(storageService.getObjectStream(asset.getObjectKey()))
                .thenAnswer(i -> new ByteArrayInputStream(stored));
        when(storageService.statObject(asset.getObjectKey()))
                .thenReturn(new StorageObjectStat(stored.length, "application/octet-stream"));
        when(assetReferenceService.hasReference(asset.getId(), REF_TYPE, REF_ID)).thenReturn(true);

        var dto = svc.serverReadSecureAsset(asset.getObjectKey(), REF_TYPE, REF_ID);

        assertThat(dto.fileData()).as("解密後必須逐位元等於原始 PDF").isEqualTo(PDF);
        assertThat(dto.contentType()).isEqualTo("application/pdf");
    }

    /**
     * 🔴 陰性對照：#3024 之前寫進去的資產（{@code encryption_key} 為 NULL）仍必須讀得到。
     *
     * <p>少了這一格，「一律解密」的實作會通過上面所有測試，而把 dev/prod 上既有的
     * 歷史發票全部鎖死——那個症狀只在讀舊資料時才出現，新資料完全正常。
     */
    @Test
    @DisplayName("陰性對照：既有的明文資產（encryption_key 為 NULL）仍讀得回來")
    void legacyPlaintextAssetsStayReadable() {
        MediaAssetService svc = serviceWith(new AesStreamCipher(MASTER_KEY));

        String legacyKey = "secure/invoice_document/20260901/legacy/OLD.pdf";
        MediaAssetEntity legacy = new MediaAssetEntity();
        legacy.setId(2685914682L);
        legacy.setObjectKey(legacyKey);
        legacy.setRefType(REF_TYPE);
        legacy.setIsSecure(true);
        legacy.setMimeType("application/pdf");
        legacy.setEncryptionKey(null); // ← 舊資產
        legacy.setSizeBytes(999_999L);

        when(mediaAssetRepository.selectByObjectKey(legacyKey)).thenReturn(legacy);
        when(storageService.getObjectStream(legacyKey)).thenAnswer(i -> new ByteArrayInputStream(PDF));
        when(storageService.statObject(legacyKey))
                .thenReturn(new StorageObjectStat(PDF.length, "application/pdf"));
        when(assetReferenceService.hasReference(legacy.getId(), REF_TYPE, REF_ID)).thenReturn(true);

        assertThat(svc.serverReadSecureAsset(legacyKey, REF_TYPE, REF_ID).fileData()).isEqualTo(PDF);
    }

    /**
     * 🔴 主密鑰不可用時 fail loud，**而且什麼都不寫**。
     *
     * <p>退回明文正是本缺陷本身，而且完全靜默：上傳成功、asset 標著 is_secure，
     * 只有靜態儲存是裸的。上傳失敗則是響的——INV.pdf 是帶重試與 DLQ 的排程 worker。
     */
    @Test
    @DisplayName("🔴 沒有主密鑰時拒絕上傳，且不得留下任何物件或資料列")
    void refusesToWritePlaintextWhenMasterKeyMissing() {
        MediaAssetService svc = serviceWith(new AesStreamCipher("")); // 沒有主密鑰

        assertThatThrownBy(() -> svc.serverUploadSecureAndFinalize(
                PDF, "ZA10068421.pdf", "application/pdf", REF_TYPE, REF_ID))
                .isInstanceOf(MediaServiceException.class);

        assertThat(uploaded.get()).as("拒絕之後不得有任何物件被送進 storage").isNull();
        verify(mediaAssetRepository, never()).insert(any());
    }
}
