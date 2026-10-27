package io.github.will413028.hawkr.media.controller;

import io.github.will413028.hawkr.common.utils.code.MediaUploadSessionStatus;
import io.github.will413028.hawkr.media.crypto.AesStreamCipher;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.entity.UploadSessionEntity;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.media.repository.AssetReferenceRepository;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import io.github.will413028.hawkr.media.service.MediaAssetService;
import io.github.will413028.hawkr.media.service.StagedTokenService;
import io.github.will413028.hawkr.media.storage.StorageObjectStat;
import io.github.will413028.hawkr.media.storage.StorageService;
import jakarta.servlet.ReadListener;
import jakarta.servlet.ServletInputStream;
import jakarta.servlet.ServletOutputStream;
import jakarta.servlet.WriteListener;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import javax.crypto.CipherInputStream;
import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.time.LocalDateTime;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * issue #2759：同一 secure upload session 的並發 PUT 不得共用同一個 R2 object key。
 * <p>
 * 修法：每次 {@code secureUpload} 呼叫都生成 per-attempt 專屬 key（不重用 session 建立時
 * 就固定的 {@code session.getObjectKey()}），成功時把贏家的 key 隨 CAS 一起寫回 DB；
 * 輸掉 CAS 的 attempt 自我刪除剛上傳、確定沒有任何 DB 列引用的物件。
 */
@ExtendWith(MockitoExtension.class)
class SecureMediaControllerTest {

    @Mock
    private UploadSessionRepository uploadSessionRepository;
    @Mock
    private AssetReferenceRepository assetReferenceRepository;
    @Mock
    private MediaAssetService mediaAssetService;
    @Mock
    private StorageService storageService;
    @Mock
    private StagedTokenService stagedTokenService;
    @Mock
    private AesStreamCipher aesStreamCipher;

    private SecureMediaController controller;

    private static final Long ACCOUNT_ID = 1001L;
    private static final String SESSION_OBJECT_KEY = "secure/20260830/original-session-key.jpg";

    @BeforeEach
    void setUp() {
        controller = new SecureMediaController(
                uploadSessionRepository,
                assetReferenceRepository,
                mediaAssetService,
                storageService,
                stagedTokenService,
                aesStreamCipher
        );
    }

    private UploadSessionEntity initSession(UUID id) {
        UploadSessionEntity session = new UploadSessionEntity();
        session.setId(id);
        session.setAccountId(ACCOUNT_ID);
        session.setObjectKey(SESSION_OBJECT_KEY);
        session.setIsSecure(true);
        session.setContentType("image/jpeg");
        session.setStatus(MediaUploadSessionStatus.INIT.value());
        session.setExpiresAt(LocalDateTime.now().plusMinutes(30));
        return session;
    }

    private HttpServletRequest fakeRequest() throws IOException {
        HttpServletRequest request = mock(HttpServletRequest.class);
        byte[] body = "fake-plaintext".getBytes();
        ServletInputStream sis = new ServletInputStream() {
            private final ByteArrayInputStream delegate = new ByteArrayInputStream(body);

            @Override
            public int read() {
                return delegate.read();
            }

            @Override
            public boolean isFinished() {
                return delegate.available() == 0;
            }

            @Override
            public boolean isReady() {
                return true;
            }

            @Override
            public void setReadListener(ReadListener readListener) {
            }
        };
        when(request.getInputStream()).thenReturn(sis);
        // getContentLengthLong() 只在 contentLength header 缺席時才會被呼叫；
        // 這裡的測試一律帶了 header（見 secureUpload 呼叫），故不 stub 它。
        return request;
    }

    @Test
    void secureUpload_usesUniquePerAttemptObjectKey_notSessionCreationKey() throws IOException {
        UUID sessionId = UUID.randomUUID();
        UploadSessionEntity session = initSession(sessionId);

        when(uploadSessionRepository.selectByIdAndAccountId(eq(sessionId), eq(ACCOUNT_ID)))
                .thenReturn(session);
        when(aesStreamCipher.generateWrappedKey()).thenReturn("wrapped-key");
        when(aesStreamCipher.encryptingStream(any(), anyString()))
                .thenReturn(mock(CipherInputStream.class));
        when(uploadSessionRepository.updateStatusExpiresAtEncryptionKeyAndObjectKey(
                eq(sessionId), anyString(), anyString(), any(), anyString(), anyString()))
                .thenReturn(1);
        when(stagedTokenService.generateToken(eq(sessionId), eq(ACCOUNT_ID), eq(true), any()))
                .thenReturn("staged-token");

        controller.secureUpload(
                ACCOUNT_ID, "SELLER", "image/jpeg", 14L,
                sessionId.toString(), fakeRequest());

        org.mockito.ArgumentCaptor<String> uploadedKey = org.mockito.ArgumentCaptor.forClass(String.class);
        verify(storageService).uploadStream(uploadedKey.capture(), any(), anyString(), anyLong());
        assertNotEquals(SESSION_OBJECT_KEY, uploadedKey.getValue(),
                "每次上傳都該用全新 per-attempt key，不可重用 session 建立時就固定的 objectKey");

        verify(uploadSessionRepository).updateStatusExpiresAtEncryptionKeyAndObjectKey(
                eq(sessionId), eq(MediaUploadSessionStatus.INIT.value()),
                eq(MediaUploadSessionStatus.COMPLETED.value()), any(),
                anyString(), eq(uploadedKey.getValue()));
    }

    @Test
    void secureUpload_whenConcurrentAttemptAlreadyWon_deletesOwnUploadAndDoesNotOverwriteWinner() throws IOException {
        UUID sessionId = UUID.randomUUID();
        UploadSessionEntity session = initSession(sessionId);

        when(uploadSessionRepository.selectByIdAndAccountId(eq(sessionId), eq(ACCOUNT_ID)))
                .thenReturn(session);
        when(aesStreamCipher.generateWrappedKey()).thenReturn("wrapped-key");
        when(aesStreamCipher.encryptingStream(any(), anyString()))
                .thenReturn(mock(CipherInputStream.class));
        // 模擬：另一個並發 attempt 已經先贏得 CAS，本次 affected=0。
        when(uploadSessionRepository.updateStatusExpiresAtEncryptionKeyAndObjectKey(
                eq(sessionId), anyString(), anyString(), any(), anyString(), anyString()))
                .thenReturn(0);

        assertThrows(MediaServiceException.class, () -> controller.secureUpload(
                ACCOUNT_ID, "SELLER", "image/jpeg", 14L,
                sessionId.toString(), fakeRequest()));

        org.mockito.ArgumentCaptor<String> uploadedKey = org.mockito.ArgumentCaptor.forClass(String.class);
        verify(storageService).uploadStream(uploadedKey.capture(), any(), anyString(), anyLong());
        // 輸家只能刪自己剛上傳、確定沒有 DB 列引用的物件——key 天生唯一，不會誤刪贏家。
        verify(storageService).deleteObject(eq(uploadedKey.getValue()));
        verify(storageService, never()).deleteObject(eq(SESSION_OBJECT_KEY));
        verify(stagedTokenService, never()).generateToken(any(), any(), any(), any());
    }

    @Test
    void secureUpload_whenR2UploadTimesOut_keepsSessionRetryableAndDoesNotIssueStagedToken() throws IOException {
        UUID sessionId = UUID.randomUUID();
        UploadSessionEntity session = initSession(sessionId);

        when(uploadSessionRepository.selectByIdAndAccountId(eq(sessionId), eq(ACCOUNT_ID)))
                .thenReturn(session);
        when(aesStreamCipher.generateWrappedKey()).thenReturn("wrapped-key");
        when(aesStreamCipher.encryptingStream(any(), anyString()))
                .thenReturn(mock(CipherInputStream.class));
        doThrow(MediaServiceException.internal("檔案上傳失敗", new RuntimeException("R2 timeout")))
                .when(storageService).uploadStream(anyString(), any(), anyString(), anyLong());

        MediaServiceException error = assertThrows(MediaServiceException.class, () -> controller.secureUpload(
                ACCOUNT_ID, "SELLER", "image/jpeg", 14L,
                sessionId.toString(), fakeRequest()));

        assertEquals("檔案上傳失敗", error.getMessage());
        assertEquals(MediaUploadSessionStatus.INIT.value(), session.getStatus());
        assertEquals(SESSION_OBJECT_KEY, session.getObjectKey());
        assertNull(session.getEncryptionKey());
        verify(uploadSessionRepository, never()).updateStatusExpiresAtEncryptionKeyAndObjectKey(
                any(), anyString(), anyString(), any(), anyString(), anyString());
        verify(stagedTokenService, never()).generateToken(any(), any(), any(), any());
        // R2 upload 尚未成功，不屬於 CAS loser；controller 不應誤刪既有 session object。
        verify(storageService, never()).deleteObject(anyString());
    }

    /**
     * Content-Length 必須來自 R2 上實際物件大小，不能信任「建立/轉正當下宣告」的
     * fileSize——那個值可能跟實際上傳的位元組數對不上（client 誤算、或行動裝置在
     * 宣告後又重新編碼過檔案）。宣告值 ≠ 實際值時，舊寫法要嘛截斷回應（宣告 < 實際）
     * 要嘛讓呼叫端永久卡住等待永遠不會到的剩餘 bytes（宣告 > 實際）——兩者都在 dev
     * 上以受控實驗重現過。
     */
    private HttpServletResponse mockResponseWithOutputStream() throws IOException {
        HttpServletResponse response = mock(HttpServletResponse.class);
        ServletOutputStream sos = new ServletOutputStream() {
            @Override
            public boolean isReady() {
                return true;
            }

            @Override
            public void setWriteListener(WriteListener writeListener) {
            }

            @Override
            public void write(int b) {
            }
        };
        when(response.getOutputStream()).thenReturn(sos);
        return response;
    }

    @Test
    void securePreview_setsContentLengthFromActualR2ObjectSize_notDeclaredSessionFileSize() throws IOException {
        UUID sessionId = UUID.randomUUID();
        UploadSessionEntity session = initSession(sessionId);
        session.setStatus(MediaUploadSessionStatus.COMPLETED.value());
        session.setEncryptionKey("wrapped-key");
        session.setExpiresAt(LocalDateTime.now().plusHours(1));
        session.setFileSize(999L); // 建立時宣告的值——刻意跟實際 R2 物件大小對不上

        when(uploadSessionRepository.selectByIdAndAccountId(eq(sessionId), eq(ACCOUNT_ID)))
                .thenReturn(session);
        // 實際存在 R2 上的密文大小 = 100（明文）+ 16（GCM tag）= 116
        when(storageService.statObject(eq(SESSION_OBJECT_KEY)))
                .thenReturn(new StorageObjectStat(116L, "image/jpeg"));
        when(storageService.getObjectStream(eq(SESSION_OBJECT_KEY)))
                .thenReturn(mock(java.io.InputStream.class));
        CipherInputStream decrypted = mock(CipherInputStream.class);
        when(decrypted.read(any(byte[].class))).thenReturn(-1);
        when(aesStreamCipher.decryptingStream(any(), eq("wrapped-key"))).thenReturn(decrypted);

        HttpServletResponse response = mockResponseWithOutputStream();

        controller.securePreview(ACCOUNT_ID, "SELLER", sessionId.toString(), response);

        verify(response).setContentLengthLong(100L);
        verify(response, never()).setContentLengthLong(999L);
    }

    /**
     * #3149 §4.9.5：Content-Disposition 的兩件事——CJK 檔名的 RFC 6266／5987 編碼，
     * 以及 <b>disposition type 不可統一成 attachment</b>。
     *
     * <p>🔴 後者是本節裡唯一「編譯與測試全綠、行為卻錯」的坑：四個消費點裡兩個是預覽、
     * 用 {@code inline}，改成 {@code attachment} 會讓賣家與 ADM 審核員按下「預覽」
     * 變成下載檔案。
     *
     * <p>期望值全部硬寫：{@code 中文報表.pdf} 的 UTF-8 百分比編碼是
     * {@code %E4%B8%AD%E6%96%87%E5%A0%B1%E8%A1%A8.pdf}（獨立以 UTF-8 位元組核算，
     * 非呼叫被測程式求得）。
     */
    private String captureContentDisposition(HttpServletResponse response) {
        org.mockito.ArgumentCaptor<String> value = org.mockito.ArgumentCaptor.forClass(String.class);
        verify(response).setHeader(eq("Content-Disposition"), value.capture());
        return value.getValue();
    }

    @Test
    void securePreview_usesInlineDisposition_andRfc5987EncodesCjkFilename() throws IOException {
        UUID sessionId = UUID.randomUUID();
        UploadSessionEntity session = initSession(sessionId);
        session.setStatus(MediaUploadSessionStatus.COMPLETED.value());
        session.setEncryptionKey("wrapped-key");
        session.setExpiresAt(LocalDateTime.now().plusHours(1));
        // 台灣賣家上傳的檔案，中文檔名是常態不是邊緣情況
        session.setOriginalFilename("中文報表.pdf");

        when(uploadSessionRepository.selectByIdAndAccountId(eq(sessionId), eq(ACCOUNT_ID)))
                .thenReturn(session);
        when(storageService.statObject(eq(SESSION_OBJECT_KEY)))
                .thenReturn(new StorageObjectStat(116L, "image/jpeg"));
        when(storageService.getObjectStream(eq(SESSION_OBJECT_KEY)))
                .thenReturn(mock(java.io.InputStream.class));
        CipherInputStream decrypted = mock(CipherInputStream.class);
        when(decrypted.read(any(byte[].class))).thenReturn(-1);
        when(aesStreamCipher.decryptingStream(any(), eq("wrapped-key"))).thenReturn(decrypted);

        HttpServletResponse response = mockResponseWithOutputStream();

        controller.securePreview(ACCOUNT_ID, "SELLER", sessionId.toString(), response);

        String disposition = captureContentDisposition(response);
        // 🔴 預覽必須是 inline
        assertTrue(disposition.startsWith("inline"),
                "預覽的 disposition 必須是 inline，實得：" + disposition);
        assertFalse(disposition.startsWith("attachment"),
                "改成 attachment 會讓「預覽」變成下載檔案，實得：" + disposition);
        // RFC 5987：直接把 UTF-8 位元組塞進 filename="..." 瀏覽器拿到的是亂碼或被截斷的名字
        assertTrue(disposition.contains("filename*=UTF-8''%E4%B8%AD%E6%96%87%E5%A0%B1%E8%A1%A8.pdf"),
                "缺少 RFC 5987 的 filename* 參數，實得：" + disposition);
    }

    @Test
    void secureDownload_usesAttachmentDisposition_andRfc5987EncodesCjkFilename() throws IOException {
        Long assetId = 556L;
        String objectKey = "secure/invoice_document/886/20260830/9f1c2b3a.pdf";
        MediaAssetEntity asset = new MediaAssetEntity();
        asset.setId(assetId);
        asset.setUploaderAccountId(ACCOUNT_ID);
        asset.setObjectKey(objectKey);
        asset.setIsSecure(true);
        asset.setEncryptionKey("wrapped-key");
        asset.setMimeType("application/pdf");
        asset.setSizeBytes(100L);
        // 與 objectKey 尾段（9f1c2b3a.pdf）刻意不同
        asset.setOriginalFilename("中文發票.pdf");

        when(mediaAssetService.getAsset(eq(assetId))).thenReturn(asset);
        when(storageService.statObject(eq(objectKey)))
                .thenReturn(new StorageObjectStat(116L, "application/pdf"));
        when(storageService.getObjectStream(eq(objectKey)))
                .thenReturn(mock(java.io.InputStream.class));
        CipherInputStream decrypted = mock(CipherInputStream.class);
        when(decrypted.read(any(byte[].class))).thenReturn(-1);
        when(aesStreamCipher.decryptingStream(any(), eq("wrapped-key"))).thenReturn(decrypted);

        HttpServletResponse response = mockResponseWithOutputStream();

        controller.secureDownload(ACCOUNT_ID, "SELLER", assetId.toString(), response);

        String disposition = captureContentDisposition(response);
        // 🔴 下載必須是 attachment（與上一格互為對照：兩個端點的 type 本來就不同）
        assertTrue(disposition.startsWith("attachment"),
                "下載的 disposition 必須是 attachment，實得：" + disposition);
        assertTrue(disposition.contains("filename*=UTF-8''%E4%B8%AD%E6%96%87%E7%99%BC%E7%A5%A8.pdf"),
                "缺少 RFC 5987 的 filename* 參數，實得：" + disposition);
        // 原始檔名優先於 objectKey 尾段
        assertFalse(disposition.contains("9f1c2b3a.pdf"),
                "應以 original_filename 為準，不得回退到 objectKey 尾段，實得：" + disposition);
    }

    /**
     * 引號迴歸（#3149）：檔名裡的 {@code "} 不得把 Content-Disposition 的 quoted-string
     * 提前收尾。
     *
     * <p>背景：設計書 §4.9.5 原本假設 {@code ContentDisposition} builder 會逃逸引號——
     * <b>在我們實際使用的 spring-web 6.1.2 上那是假的</b>（實測
     * {@code filename("a\"b.pdf", UTF_8)} 輸出 {@code filename="=?UTF-8?Q?a"b.pdf?="}，
     * 引號原樣輸出；只有不帶 charset 的多載走 {@code encodeQuotedPairs} 才逃逸，
     * 而 7.0.6 已在 builder 層修掉）。⇒ 防線不能建立在 Spring 版本上。
     *
     * <p><b>本格刻意餵未消毒的值</b>：{@code original_filename} 直接放
     * {@code a"b.pdf}，走的是 {@code resolveDisplayName} 讀取側的消毒。寫入側同樣會擋
     * （{@code MediaAssetServiceTest} 的兩支直傳測試），兩層都在——這一格驗的是**讀取側
     * 那一層**，因為 V9 之前的列與回退路徑不受寫入側保護。
     */
    @Test
    void secureDownload_filenameFromWritePathCannotBreakQuotedString() throws IOException {
        Long assetId = 557L;
        String objectKey = "secure/invoice_document/886/20260830/9f1c2b3a.pdf";
        MediaAssetEntity asset = new MediaAssetEntity();
        asset.setId(assetId);
        asset.setUploaderAccountId(ACCOUNT_ID);
        asset.setObjectKey(objectKey);
        asset.setIsSecure(true);
        asset.setEncryptionKey("wrapped-key");
        asset.setMimeType("application/pdf");
        asset.setSizeBytes(100L);
        // 🔴 未消毒的值直接落在欄位上（模擬 V9 之前的列、或任何漏了寫入側消毒的路徑）
        asset.setOriginalFilename("a\"b.pdf");

        when(mediaAssetService.getAsset(eq(assetId))).thenReturn(asset);
        when(storageService.statObject(eq(objectKey)))
                .thenReturn(new StorageObjectStat(116L, "application/pdf"));
        when(storageService.getObjectStream(eq(objectKey)))
                .thenReturn(mock(java.io.InputStream.class));
        CipherInputStream decrypted = mock(CipherInputStream.class);
        when(decrypted.read(any(byte[].class))).thenReturn(-1);
        when(aesStreamCipher.decryptingStream(any(), eq("wrapped-key"))).thenReturn(decrypted);

        HttpServletResponse response = mockResponseWithOutputStream();

        controller.secureDownload(ACCOUNT_ID, "SELLER", assetId.toString(), response);

        String disposition = captureContentDisposition(response);
        assertEquals("attachment; filename=\"=?UTF-8?Q?ab.pdf?=\"; filename*=UTF-8''ab.pdf",
                disposition);
        // quoted-string 必須恰好由兩個引號界定：多一個就是被提前收尾
        assertEquals(2, disposition.chars().filter(c -> c == '"').count(),
                "Content-Disposition 出現非界定用的裸引號：" + disposition);
    }

    @Test
    void secureDownload_setsContentLengthFromActualR2ObjectSize_notStaleSizeBytes() throws IOException {
        Long assetId = 555L;
        String objectKey = "secure/product/886/20260830/original.jpg";
        MediaAssetEntity asset = new MediaAssetEntity();
        asset.setId(assetId);
        asset.setUploaderAccountId(ACCOUNT_ID);
        asset.setObjectKey(objectKey);
        asset.setIsSecure(true);
        asset.setEncryptionKey("wrapped-key");
        asset.setMimeType("image/jpeg");
        asset.setSizeBytes(999L); // 轉正當下複製自 session 宣告值——同樣可能跟實際對不上

        when(mediaAssetService.getAsset(eq(assetId))).thenReturn(asset);
        when(storageService.statObject(eq(objectKey)))
                .thenReturn(new StorageObjectStat(116L, "image/jpeg"));
        when(storageService.getObjectStream(eq(objectKey)))
                .thenReturn(mock(java.io.InputStream.class));
        CipherInputStream decrypted = mock(CipherInputStream.class);
        when(decrypted.read(any(byte[].class))).thenReturn(-1);
        when(aesStreamCipher.decryptingStream(any(), eq("wrapped-key"))).thenReturn(decrypted);

        HttpServletResponse response = mockResponseWithOutputStream();

        controller.secureDownload(ACCOUNT_ID, "SELLER", assetId.toString(), response);

        verify(response).setContentLengthLong(100L);
        verify(response, never()).setContentLengthLong(999L);
    }
}
