package io.github.will413028.hawkr.media.storage;

import io.github.will413028.hawkr.media.exception.MediaServiceException;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.test.util.ReflectionTestUtils;
import software.amazon.awssdk.auth.credentials.AwsBasicCredentials;
import software.amazon.awssdk.auth.credentials.StaticCredentialsProvider;
import software.amazon.awssdk.core.client.config.ClientOverrideConfiguration;
import software.amazon.awssdk.core.retry.RetryPolicy;
import software.amazon.awssdk.core.sync.RequestBody;
import software.amazon.awssdk.services.s3.S3Client;
import software.amazon.awssdk.services.s3.model.CopyObjectRequest;
import software.amazon.awssdk.services.s3.model.DeleteObjectRequest;
import software.amazon.awssdk.services.s3.model.HeadObjectRequest;
import software.amazon.awssdk.services.s3.model.HeadObjectResponse;
import software.amazon.awssdk.services.s3.model.NoSuchKeyException;
import software.amazon.awssdk.services.s3.model.PutObjectRequest;
import software.amazon.awssdk.services.s3.presigner.S3Presigner;
import software.amazon.awssdk.services.s3.presigner.model.PresignedGetObjectRequest;
import software.amazon.awssdk.services.s3.presigner.model.PresignedPutObjectRequest;

import java.io.ByteArrayInputStream;
import java.net.InetSocketAddress;
import java.net.URI;
import java.net.URL;
import java.time.Duration;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.never;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@link R2StorageService} 單元測試（mock S3 client 層級）。
 * 驗 put / delete / copy / head 的 objectKey 傳遞與 presign，及 404→null 語意。
 */
class R2StorageServiceTest {

    private static final String BUCKET = "hawkr-media-dev";

    private S3Client s3Client;
    private S3Presigner s3Presigner;
    private R2StorageService service;

    @BeforeEach
    void setUp() {
        s3Client = mock(S3Client.class);
        s3Presigner = mock(S3Presigner.class);
        service = new R2StorageService();
        ReflectionTestUtils.setField(service, "bucketName", BUCKET);
        ReflectionTestUtils.setField(service, "s3Client", s3Client);
        ReflectionTestUtils.setField(service, "s3Presigner", s3Presigner);
    }

    @Test
    @DisplayName("uploadStream：正確 bucket/key/contentType 傳遞給 S3 putObject")
    void uploadStreamPassesObjectKey() {
        byte[] data = "hello".getBytes();
        service.uploadStream("public/product/1/x.jpg",
            new java.io.ByteArrayInputStream(data), "image/jpeg", data.length);

        ArgumentCaptor<PutObjectRequest> captor = ArgumentCaptor.forClass(PutObjectRequest.class);
        verify(s3Client).putObject(captor.capture(), any(RequestBody.class));
        assertEquals(BUCKET, captor.getValue().bucket());
        assertEquals("public/product/1/x.jpg", captor.getValue().key());
        assertEquals("image/jpeg", captor.getValue().contentType());
    }

    @Test
    @DisplayName("uploadStream：R2 API timeout 轉成 media upload failure")
    void uploadStreamWrapsR2ApiTimeout() throws Exception {
        HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        server.createContext("/", exchange -> {
            try {
                Thread.sleep(2_000);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            } finally {
                exchange.close();
            }
        });
        server.start();

        S3Client timeoutClient = S3Client.builder()
            .endpointOverride(URI.create("http://127.0.0.1:" + server.getAddress().getPort()))
            .credentialsProvider(StaticCredentialsProvider.create(
                AwsBasicCredentials.create("test-access", "test-gpsMarker")))
            .region(software.amazon.awssdk.regions.Region.of("auto"))
            .overrideConfiguration(ClientOverrideConfiguration.builder()
                .apiCallAttemptTimeout(Duration.ofMillis(100))
                .apiCallTimeout(Duration.ofMillis(500))
                .retryPolicy(RetryPolicy.none())
                .build())
            .build();
        R2StorageService timeoutService = new R2StorageService(timeoutClient, BUCKET);

        try {
            MediaServiceException error = assertThrows(MediaServiceException.class,
                () -> timeoutService.uploadStream(
                    "secure/test/timeout.attempt", new ByteArrayInputStream(new byte[]{1}),
                    "application/octet-stream", 1));

            assertEquals("檔案上傳失敗", error.getMessage());
            assertEquals(500, error.getHttpStatusCode());
        } finally {
            timeoutService.destroy();
            server.stop(0);
        }
    }

    @Test
    @DisplayName("deleteObject：正確 key 傳遞")
    void deletePassesObjectKey() {
        service.deleteObject("tmp/20260216/a.jpg");
        ArgumentCaptor<DeleteObjectRequest> captor = ArgumentCaptor.forClass(DeleteObjectRequest.class);
        verify(s3Client).deleteObject(captor.capture());
        assertEquals(BUCKET, captor.getValue().bucket());
        assertEquals("tmp/20260216/a.jpg", captor.getValue().key());
    }

    @Test
    @DisplayName("copyObject：source/destination key 傳遞")
    void copyPassesKeys() {
        service.copyObject("tmp/20260216/a.jpg", "public/product/1/20260216/a.jpg");
        ArgumentCaptor<CopyObjectRequest> captor = ArgumentCaptor.forClass(CopyObjectRequest.class);
        verify(s3Client).copyObject(captor.capture());
        assertEquals(BUCKET, captor.getValue().sourceBucket());
        assertEquals("tmp/20260216/a.jpg", captor.getValue().sourceKey());
        assertEquals(BUCKET, captor.getValue().destinationBucket());
        assertEquals("public/product/1/20260216/a.jpg", captor.getValue().destinationKey());
    }

    private static final byte[] JPEG_WITH_GPS = jpegWithGps();

    private static byte[] jpegWithGps() {
        byte[] gpsMarker = "GPS-25.0330N-121.5654E".getBytes(java.nio.charset.StandardCharsets.ISO_8859_1);
        int len = 2 + 6 + gpsMarker.length;
        java.io.ByteArrayOutputStream o = new java.io.ByteArrayOutputStream();
        o.writeBytes(new byte[] {(byte) 0xFF, (byte) 0xD8, (byte) 0xFF, (byte) 0xE1, (byte) (len >> 8), (byte) len});
        o.writeBytes("Exif\0\0".getBytes(java.nio.charset.StandardCharsets.ISO_8859_1));
        o.writeBytes(gpsMarker);
        o.writeBytes(new byte[] {(byte) 0xFF, (byte) 0xDA, 0, 4, 0, 0, 1, 2, 3, (byte) 0xFF, (byte) 0xD9});
        return o.toByteArray();
    }

    private static boolean hasGps(byte[] d) {
        return new String(d, java.nio.charset.StandardCharsets.ISO_8859_1).contains("GPS-25");
    }

    @Test
    @DisplayName("copyObject 到 public/：圖片以剝除 metadata 後的位元組寫入，且不走伺服器端複製")
    void copyToPublicStripsMetadata() {
        ReflectionTestUtils.setField(service, "publicPrefix", "public");
        when(s3Client.getObjectAsBytes(any(software.amazon.awssdk.services.s3.model.GetObjectRequest.class)))
            .thenReturn(software.amazon.awssdk.core.ResponseBytes.fromByteArray(
                software.amazon.awssdk.services.s3.model.GetObjectResponse.builder().contentType("image/jpeg").build(),
                JPEG_WITH_GPS));

        service.copyObject("tmp/20260216/a.jpg", "public/product/1/20260216/a.jpg");

        ArgumentCaptor<PutObjectRequest> req = ArgumentCaptor.forClass(PutObjectRequest.class);
        ArgumentCaptor<RequestBody> body = ArgumentCaptor.forClass(RequestBody.class);
        verify(s3Client).putObject(req.capture(), body.capture());
        verify(s3Client, never()).copyObject(any(CopyObjectRequest.class));
        assertEquals("public/product/1/20260216/a.jpg", req.getValue().key());
        assertEquals("image/jpeg", req.getValue().contentType());
        byte[] stored = readAll(body.getValue());
        assertFalse(hasGps(stored));
    }

    @Test
    @DisplayName("copyObject 到 public/：結構壞掉的圖片拒絕，不寫入也不退回原檔複製")
    void copyToPublicRejectsCorruptImage() {
        ReflectionTestUtils.setField(service, "publicPrefix", "public");
        byte[] corrupt = java.util.Arrays.copyOf(JPEG_WITH_GPS, 12);
        when(s3Client.getObjectAsBytes(any(software.amazon.awssdk.services.s3.model.GetObjectRequest.class)))
            .thenReturn(software.amazon.awssdk.core.ResponseBytes.fromByteArray(
                software.amazon.awssdk.services.s3.model.GetObjectResponse.builder().contentType("image/jpeg").build(),
                corrupt));

        assertThrows(MediaServiceException.class,
            () -> service.copyObject("tmp/20260216/a.jpg", "public/product/1/20260216/a.jpg"));
        verify(s3Client, never()).copyObject(any(CopyObjectRequest.class));
        verify(s3Client, never()).putObject(any(PutObjectRequest.class), any(RequestBody.class));
    }

    @Test
    @DisplayName("copyObject 到 public/：不是圖片的物件照舊伺服器端複製")
    void copyToPublicNonImageUsesServerSideCopy() {
        ReflectionTestUtils.setField(service, "publicPrefix", "public");
        when(s3Client.getObjectAsBytes(any(software.amazon.awssdk.services.s3.model.GetObjectRequest.class)))
            .thenReturn(software.amazon.awssdk.core.ResponseBytes.fromByteArray(
                software.amazon.awssdk.services.s3.model.GetObjectResponse.builder().build(),
                "%PDF-1.7".getBytes(java.nio.charset.StandardCharsets.ISO_8859_1)));

        service.copyObject("tmp/20260216/a.pdf", "public/shipping/1/20260216/a.pdf");

        verify(s3Client).copyObject(any(CopyObjectRequest.class));
    }

    @Test
    @DisplayName("uploadStream 到 public/：圖片剝除 metadata 後上傳")
    void uploadToPublicStripsMetadata() {
        ReflectionTestUtils.setField(service, "publicPrefix", "public");

        service.uploadStream("public/shipping/20260216/u/a.jpg",
            new ByteArrayInputStream(JPEG_WITH_GPS), "image/jpeg", JPEG_WITH_GPS.length);

        ArgumentCaptor<RequestBody> body = ArgumentCaptor.forClass(RequestBody.class);
        verify(s3Client).putObject(any(PutObjectRequest.class), body.capture());
        assertFalse(hasGps(readAll(body.getValue())));
    }

    @Test
    @DisplayName("uploadStream 到 tmp/：不處理，位元組原樣（轉正時才剝除）")
    void uploadToTmpIsUntouched() {
        ReflectionTestUtils.setField(service, "publicPrefix", "public");

        service.uploadStream("tmp/20260216/a.jpg",
            new ByteArrayInputStream(JPEG_WITH_GPS), "image/jpeg", JPEG_WITH_GPS.length);

        ArgumentCaptor<RequestBody> body = ArgumentCaptor.forClass(RequestBody.class);
        verify(s3Client).putObject(any(PutObjectRequest.class), body.capture());
        assertTrue(hasGps(readAll(body.getValue())));
    }

    private static byte[] readAll(RequestBody body) {
        try (java.io.InputStream in = body.contentStreamProvider().newStream()) {
            return in.readAllBytes();
        } catch (java.io.IOException e) {
            throw new java.io.UncheckedIOException(e);
        }
    }

    @Test
    @DisplayName("statObject：回傳實際 size/contentType")
    void statObjectReturnsStat() {
        when(s3Client.headObject(any(HeadObjectRequest.class)))
            .thenReturn(HeadObjectResponse.builder()
                .contentLength(2048L).contentType("image/png").build());

        StorageObjectStat stat = service.statObject("public/product/1/a.png");
        assertEquals(2048L, stat.contentLength());
        assertEquals("image/png", stat.contentType());

        ArgumentCaptor<HeadObjectRequest> captor = ArgumentCaptor.forClass(HeadObjectRequest.class);
        verify(s3Client).headObject(captor.capture());
        assertEquals("public/product/1/a.png", captor.getValue().key());
    }

    @Test
    @DisplayName("statObject：物件不存在（NoSuchKey）→ null")
    void statObjectMissingReturnsNull() {
        when(s3Client.headObject(any(HeadObjectRequest.class)))
            .thenThrow(NoSuchKeyException.builder().message("no such key").build());
        assertNull(service.statObject("public/missing.jpg"));
    }

    @Test
    @DisplayName("presign PUT：回傳 presigner 產生的 URL")
    void presignUpload() throws Exception {
        PresignedPutObjectRequest presigned = mock(PresignedPutObjectRequest.class);
        when(presigned.url()).thenReturn(new URL("https://acc.r2.cloudflarestorage.com/put"));
        when(s3Presigner.presignPutObject(any(software.amazon.awssdk.services.s3.presigner.model.PutObjectPresignRequest.class)))
            .thenReturn(presigned);

        String url = service.generatePresignedUploadUrl("tmp/a.jpg", "image/jpeg", 600);
        assertEquals("https://acc.r2.cloudflarestorage.com/put", url);
    }

    @Test
    @DisplayName("presign GET：回傳 presigner 產生的 URL")
    void presignDownload() throws Exception {
        PresignedGetObjectRequest presigned = mock(PresignedGetObjectRequest.class);
        when(presigned.url()).thenReturn(new URL("https://acc.r2.cloudflarestorage.com/get"));
        when(s3Presigner.presignGetObject(any(software.amazon.awssdk.services.s3.presigner.model.GetObjectPresignRequest.class)))
            .thenReturn(presigned);

        String url = service.generatePresignedDownloadUrl("public/a.jpg", 600);
        assertEquals("https://acc.r2.cloudflarestorage.com/get", url);
    }

    @Test
    @DisplayName("未配置（mock/skip fallback）：isEnabled=false，presign 回模擬 URL、statObject 回 null")
    void notConfiguredFallback() {
        R2StorageService bare = new R2StorageService();
        // s3Client/s3Presigner 皆 null（未 init）
        org.junit.jupiter.api.Assertions.assertFalse(bare.isEnabled());
        assertNull(bare.statObject("public/a.jpg"));
        org.junit.jupiter.api.Assertions.assertEquals(
            "https://mock-r2.example.com/upload/tmp/a.jpg",
            bare.generatePresignedUploadUrl("tmp/a.jpg", "image/jpeg", 600));
    }

    @Test
    @DisplayName("copyObject：來源不存在（NoSuchKey）→ notFound")
    void copyMissingSourceThrows() {
        when(s3Client.copyObject(any(CopyObjectRequest.class)))
            .thenThrow(NoSuchKeyException.builder().message("no such key").build());
        assertThrows(MediaServiceException.class,
            () -> service.copyObject("tmp/missing.jpg", "public/x.jpg"));
    }
}
