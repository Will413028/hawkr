package io.github.will413028.hawkr.media.storage;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import jakarta.annotation.PostConstruct;
import jakarta.annotation.PreDestroy;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.stereotype.Service;
import software.amazon.awssdk.auth.credentials.AwsBasicCredentials;
import software.amazon.awssdk.auth.credentials.StaticCredentialsProvider;
import software.amazon.awssdk.core.ResponseBytes;
import software.amazon.awssdk.core.ResponseInputStream;
import software.amazon.awssdk.core.sync.RequestBody;
import software.amazon.awssdk.regions.Region;
import software.amazon.awssdk.services.s3.S3Client;
import software.amazon.awssdk.services.s3.model.CopyObjectRequest;
import software.amazon.awssdk.services.s3.model.DeleteObjectRequest;
import software.amazon.awssdk.services.s3.model.GetObjectRequest;
import software.amazon.awssdk.services.s3.model.GetObjectResponse;
import software.amazon.awssdk.services.s3.model.HeadObjectRequest;
import software.amazon.awssdk.services.s3.model.HeadObjectResponse;
import software.amazon.awssdk.services.s3.model.NoSuchKeyException;
import software.amazon.awssdk.services.s3.model.PutObjectRequest;
import software.amazon.awssdk.services.s3.model.S3Exception;
import software.amazon.awssdk.services.s3.presigner.S3Presigner;
import software.amazon.awssdk.services.s3.presigner.model.GetObjectPresignRequest;
import software.amazon.awssdk.services.s3.presigner.model.PutObjectPresignRequest;

import java.io.InputStream;
import java.net.URI;
import java.time.Duration;

/**
 * Cloudflare R2 儲存實作（唯一儲存後端）。
 *
 * <p>R2 為 S3 相容 API（SigV4），以 AWS S3 SDK v2 對 R2 endpoint
 * （{@code https://{accountId}.r2.cloudflarestorage.com}）操作。方法面與 backend 無關，
 * 由 {@link StorageService} 定義；objectKey 生成與消毒沿用 {@link AbstractStorageService}。
 *
 * <p>SigV4 presigned PUT 只綁 Content-Type，無法表達 size 範圍條件
 * （{@code content-length-range} 為 S3 POST policy 專屬）——size 上限一律由 finalize
 * HEAD 回查兜底（49 號 §8.1）。R2 region 固定 {@code auto}。
 *
 * <p>{@code hawkr.media.storage.backend} 缺省或 {@code r2} 時生效；任何其他值
 * （如遺留的 {@code oss}）不裝配 bean，讓服務啟動即 fail-fast（OSS 遷移已完成、無退路）。
 */
@Service
@ConditionalOnProperty(name = "hawkr.media.storage.backend", havingValue = "r2", matchIfMissing = true)
public class R2StorageService extends AbstractStorageService {

    private static final ObsLogger log = ObsLogger.forClass(R2StorageService.class);

    @Value("${media.r2.account-id:}")
    private String accountId;

    @Value("${media.r2.access-key-id:}")
    private String accessKeyId;

    @Value("${media.r2.secret-access-key:}")
    private String secretAccessKey;

    @Value("${media.r2.bucket-name:}")
    private String bucketName;

    private S3Client s3Client;
    private S3Presigner s3Presigner;

    /**
     * Spring 建構用的預設 constructor；R2 client 仍由 {@link #init()} 依環境設定建立。
     */
    public R2StorageService() {
    }

    /**
     * 測試用注入點：讓測試能以受控 endpoint 驅動真實 AWS SDK timeout，
     * 不需要改動任何 dev/prod R2 設定或 secret。
     */
    R2StorageService(S3Client s3Client, String bucketName) {
        this.s3Client = s3Client;
        this.bucketName = bucketName;
    }

    @PostConstruct
    public void init() {
        if (accountId != null && !accountId.isBlank()
            && accessKeyId != null && !accessKeyId.isBlank()
            && secretAccessKey != null && !secretAccessKey.isBlank()
            && bucketName != null && !bucketName.isBlank()) {
            URI endpoint = URI.create("https://" + accountId + ".r2.cloudflarestorage.com");
            StaticCredentialsProvider creds = StaticCredentialsProvider.create(
                AwsBasicCredentials.create(accessKeyId, secretAccessKey));
            this.s3Client = S3Client.builder()
                .endpointOverride(endpoint)
                .credentialsProvider(creds)
                .region(Region.of("auto"))
                .build();
            this.s3Presigner = S3Presigner.builder()
                .endpointOverride(endpoint)
                .credentialsProvider(creds)
                .region(Region.of("auto"))
                .build();
            log.info("r2.init.success", "R2 客戶端初始化成功", "bucket", bucketName);
        } else {
            log.warn("r2.init.skip", "R2 配置不完整，跳過初始化（開發模式）");
        }
    }

    @PreDestroy
    public void destroy() {
        if (s3Client != null) {
            s3Client.close();
        }
        if (s3Presigner != null) {
            s3Presigner.close();
        }
        log.info("r2.shutdown", "R2 客戶端關閉");
    }

    @Override
    public boolean isEnabled() {
        return s3Client != null;
    }

    @Override
    public String generatePresignedUploadUrl(String objectKey, String contentType, int expiresSec) {
        if (s3Presigner == null) {
            log.warn("r2.presign.mock", "R2 未配置，回傳模擬 URL");
            return "https://mock-r2.example.com/upload/" + objectKey;
        }
        try {
            PutObjectRequest putRequest = PutObjectRequest.builder()
                .bucket(bucketName)
                .key(objectKey)
                .contentType(contentType)
                .build();
            PutObjectPresignRequest presignRequest = PutObjectPresignRequest.builder()
                .signatureDuration(Duration.ofSeconds(expiresSec))
                .putObjectRequest(putRequest)
                .build();
            String url = s3Presigner.presignPutObject(presignRequest).url().toString();
            log.info("r2.presign.success", "生成預簽名 URL 成功", "object_key", objectKey);
            return url;
        } catch (Exception e) {
            log.error("r2.presign.failed", "生成預簽名 URL 失敗", e);
            throw MediaServiceException.internal("生成上傳 URL 失敗", e);
        }
    }

    @Override
    public String generatePresignedDownloadUrl(String objectKey, int expiresSec) {
        if (s3Presigner == null) {
            return "https://mock-r2.example.com/download/" + objectKey;
        }
        try {
            GetObjectRequest getRequest = GetObjectRequest.builder()
                .bucket(bucketName)
                .key(objectKey)
                .build();
            GetObjectPresignRequest presignRequest = GetObjectPresignRequest.builder()
                .signatureDuration(Duration.ofSeconds(expiresSec))
                .getObjectRequest(getRequest)
                .build();
            return s3Presigner.presignGetObject(presignRequest).url().toString();
        } catch (Exception e) {
            log.error("r2.presign.download.failed", "生成下載 URL 失敗", e);
            throw MediaServiceException.internal("生成下載 URL 失敗", e);
        }
    }

    @Override
    public void uploadStream(String objectKey, InputStream inputStream, String contentType, long size) {
        if (s3Client == null) {
            log.warn("r2.upload.mock", "R2 未配置，跳過實際上傳");
            return;
        }
        try {
            if (isPublicKey(objectKey)) {
                byte[] data = inputStream.readAllBytes();
                byte[] stripped = stripOrReject(data);
                putPublicBytes(objectKey, stripped != null ? stripped : data, contentType, null, null);
                log.info("r2.upload.success", "檔案上傳成功", "object_key", objectKey);
                return;
            }
            PutObjectRequest request = PutObjectRequest.builder()
                .bucket(bucketName)
                .key(objectKey)
                .contentType(contentType)
                .contentLength(size)
                .build();
            s3Client.putObject(request, RequestBody.fromInputStream(inputStream, size));
            log.info("r2.upload.success", "檔案上傳成功", "object_key", objectKey);
        } catch (MediaServiceException e) {
            throw e;
        } catch (Exception e) {
            log.error("r2.upload.failed", "檔案上傳失敗", e);
            throw MediaServiceException.internal("檔案上傳失敗", e);
        }
    }

    @Override
    public InputStream getObjectStream(String objectKey) {
        if (s3Client == null) {
            throw MediaServiceException.internal("R2 未配置");
        }
        try {
            GetObjectRequest request = GetObjectRequest.builder()
                .bucket(bucketName)
                .key(objectKey)
                .build();
            ResponseInputStream<GetObjectResponse> stream = s3Client.getObject(request);
            return stream;
        } catch (NoSuchKeyException e) {
            log.warn("r2.download.not_found", "檔案不存在", "object_key", objectKey);
            throw MediaServiceException.notFound("檔案不存在: " + objectKey);
        } catch (Exception e) {
            log.error("r2.download.failed", "取得檔案串流失敗", e);
            throw MediaServiceException.internal("取得檔案失敗", e);
        }
    }

    @Override
    public void deleteObject(String objectKey) {
        if (s3Client == null) {
            log.warn("r2.delete.mock", "R2 未配置，跳過實際刪除");
            return;
        }
        try {
            s3Client.deleteObject(DeleteObjectRequest.builder()
                .bucket(bucketName)
                .key(objectKey)
                .build());
            log.info("r2.delete.success", "檔案刪除成功", "object_key", objectKey);
        } catch (Exception e) {
            log.error("r2.delete.failed", "檔案刪除失敗", e);
            throw MediaServiceException.internal("檔案刪除失敗", e);
        }
    }

    @Override
    public void copyObject(String sourceKey, String destinationKey) {
        if (s3Client == null) {
            log.warn("r2.copy.mock", "R2 未配置，跳過實際複製",
                "source", sourceKey + " → " + destinationKey);
            return;
        }
        try {
            if (isPublicKey(destinationKey) && copyPublicImageWithoutMetadata(sourceKey, destinationKey)) {
                return;
            }
            s3Client.copyObject(CopyObjectRequest.builder()
                .sourceBucket(bucketName)
                .sourceKey(sourceKey)
                .destinationBucket(bucketName)
                .destinationKey(destinationKey)
                .build());
            log.info("r2.copy.success", "R2 物件複製成功",
                "path", sourceKey + " → " + destinationKey);
        } catch (NoSuchKeyException e) {
            log.warn("r2.copy.not_found", "來源物件不存在", "source", sourceKey);
            throw MediaServiceException.notFound("來源檔案不存在: " + sourceKey);
        } catch (MediaServiceException e) {
            throw e;
        } catch (Exception e) {
            log.error("r2.copy.failed", "R2 物件複製失敗", e);
            throw MediaServiceException.internal("R2 物件複製失敗", e);
        }
    }

    @Override
    public StorageObjectStat statObject(String objectKey) {
        if (s3Client == null) {
            return null;
        }
        try {
            HeadObjectResponse resp = s3Client.headObject(HeadObjectRequest.builder()
                .bucket(bucketName)
                .key(objectKey)
                .build());
            return new StorageObjectStat(
                resp.contentLength() != null ? resp.contentLength() : 0L,
                resp.contentType());
        } catch (NoSuchKeyException e) {
            return null;
        } catch (S3Exception e) {
            // HEAD 無回應 body，SDK 對 404 通常拋 S3Exception(statusCode=404) 而非 NoSuchKeyException
            if (e.statusCode() == 404) {
                return null;
            }
            log.error("r2.head.failed", "HEAD 物件失敗", e);
            throw MediaServiceException.internal("HEAD 物件失敗", e);
        } catch (Exception e) {
            log.error("r2.head.failed", "HEAD 物件失敗", e);
            throw MediaServiceException.internal("HEAD 物件失敗", e);
        }
    }

    // ---- 公開圖片 metadata 剝除 ----
    // 不變量：寫進 public/ 的圖片位元組不含 EXIF（GPS）、XMP、IPTC 等 metadata，只保留顯示方向。
    // 公開出口（image gateway 的 original）原樣直出儲存的位元組，所以在寫入這一刻處理，
    // 之後的下載、後台與備份就不必各自再處理。所有寫入 public/ 的路徑（轉正的 copy、後端直傳）都經過這裡。

    private boolean isPublicKey(String key) {
        return publicPrefix != null && key != null && key.startsWith(publicPrefix + "/");
    }

    /** 來源是可辨認的圖片時，讀出、剝除、以剝除後的位元組寫入目的地並回傳 true；不是圖片回傳 false（呼叫端走一般複製）。 */
    private boolean copyPublicImageWithoutMetadata(String sourceKey, String destinationKey) {
        ResponseBytes<GetObjectResponse> src = s3Client.getObjectAsBytes(GetObjectRequest.builder()
            .bucket(bucketName)
            .key(sourceKey)
            .build());
        GetObjectResponse meta = src.response();
        byte[] stripped = stripOrReject(src.asByteArrayUnsafe());
        if (stripped == null) {
            return false;
        }
        putPublicBytes(destinationKey, stripped, meta.contentType(), meta.contentDisposition(), meta.cacheControl());
        log.info("r2.copy.success", "R2 物件複製成功（已剝除圖片 metadata）",
            "path", sourceKey + " → " + destinationKey);
        return true;
    }

    private void putPublicBytes(String key, byte[] data, String contentType, String contentDisposition, String cacheControl) {
        byte[] body = data;
        s3Client.putObject(PutObjectRequest.builder()
            .bucket(bucketName)
            .key(key)
            .contentType(contentType)
            .contentDisposition(contentDisposition)
            .cacheControl(cacheControl)
            .contentLength((long) body.length)
            .build(), RequestBody.fromBytes(body));
    }

    private static byte[] stripOrReject(byte[] data) {
        try {
            return ImageMetadataStripper.strip(data);
        } catch (IllegalArgumentException e) {
            log.warn("r2.strip.rejected", "圖片結構無法解析，拒絕寫入公開路徑", "reason", e.getMessage());
            throw MediaServiceException.invalidArgument("圖片檔案損毀或格式不正確，無法使用");
        }
    }
}
