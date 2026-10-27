package io.github.will413028.hawkr.media.storage;

import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import org.springframework.beans.factory.annotation.Value;

import java.time.LocalDate;
import java.time.format.DateTimeFormatter;
import java.util.UUID;

/**
 * 儲存實作共用基底：objectKey 生成與副檔名消毒等純邏輯（與 backend 無關），
 * 由 {@link R2StorageService} 沿用（抽象基底保留，便於未來替換/新增 backend 時共用，
 * 工程契約 §15）。
 *
 * <p>objectKey 結構規範不變（08 號 §2.2）：暫存兩種深度、日期段以 {@code \d{8}} 定位。
 */
public abstract class AbstractStorageService implements StorageService {

    protected static final DateTimeFormatter DATE_FORMATTER = DateTimeFormatter.ofPattern("yyyyMMdd");

    @Value("${media.storage.public-prefix:public}")
    protected String publicPrefix;

    @Value("${media.storage.secure-prefix:secure}")
    protected String securePrefix;

    @Value("${media.storage.temp-prefix:tmp}")
    protected String tempPrefix;

    @Override
    public String generateObjectKey(String filename, boolean isSecure) {
        String dateStr = LocalDate.now().format(DATE_FORMATTER);
        String uuid = UUID.randomUUID().toString();
        String extension = sanitizedExtension(filename);

        String prefix = isSecure ? securePrefix : tempPrefix;

        return String.format("%s/%s/%s%s", prefix, dateStr, uuid, extension);
    }

    @Override
    public String generateFinalObjectKey(String originalKey, boolean isSecure, String refType, Long ownerId) {
        // 取得原始檔名與日期段。
        // 暫存 key 結構不只一種深度（08 §4.1 step 5）：
        //   - 一般上傳：tmp/{yyyyMMdd}/{uuid}{ext}（3 段）
        //   - import 暫存：tmp/{yyyyMMdd}/{storeId}/import/{uuid}{ext}（5 段）
        // 故 {date} 段必須「以 yyyyMMdd 樣式定位」，禁止用「倒數第二段」這種與深度耦合的假設，
        // 否則 import key 會把字面 "import" 當成日期段，產出 public/product/{storeId}/import/{uuid}（#1203）。
        String[] parts = originalKey.split("/");
        String filename = parts[parts.length - 1];
        String dateStr = null;
        for (String part : parts) {
            if (isYyyyMmDd(part)) {
                dateStr = part;
                break;
            }
        }
        if (dateStr == null) {
            // fallback：暫存 key 不含可辨識日期段時用今天日期
            dateStr = LocalDate.now().format(DATE_FORMATTER);
        }

        String visibility = isSecure ? securePrefix : publicPrefix;
        String category = AssetReferenceType.fromValue(refType).code().toLowerCase();

        return String.format("%s/%s/%d/%s/%s", visibility, category, ownerId, dateStr, filename);
    }

    /**
     * 取得檔案副檔名並消毒（49 號 §8.1）。
     * <p>拒絕含路徑分隔符 {@code /}、{@code \} 或穿越序列 {@code ..} 的副檔名，
     * 避免惡意檔名污染 objectKey。</p>
     */
    protected String sanitizedExtension(String filename) {
        if (filename == null || !filename.contains(".")) {
            return "";
        }
        String extension = filename.substring(filename.lastIndexOf("."));
        if (extension.contains("/") || extension.contains("\\") || extension.contains("..")) {
            throw MediaServiceException.invalidArgument("非法檔名副檔名: " + extension);
        }
        return extension;
    }

    /** 判斷字串是否為 8 位數字（yyyyMMdd）日期段。 */
    protected static boolean isYyyyMmDd(String s) {
        if (s == null || s.length() != 8) {
            return false;
        }
        for (int i = 0; i < 8; i++) {
            if (!Character.isDigit(s.charAt(i))) {
                return false;
            }
        }
        return true;
    }
}
