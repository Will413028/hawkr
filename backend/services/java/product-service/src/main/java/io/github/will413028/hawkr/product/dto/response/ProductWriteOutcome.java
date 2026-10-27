package io.github.will413028.hawkr.product.dto.response;

import java.util.List;

/**
 * 商品建立／更新的完整結果：商品本體 ＋ 本次未被採用的認證（19 號設計書 §9.5）。
 *
 * <p><b>為什麼另開一個型別而不是往 {@link ProductDetailDTO} 加欄位</b>：
 * {@code ProductDetailDTO} 同時是商品詳情快取（{@code prod:detail:*}）與冪等回應的載體，
 * 而 {@code certificationWarnings} 是「這一次請求的副作用」——它不屬於商品狀態，
 * 塞進去會跟著被快取、被 GET 詳情端點吐出來。
 *
 * <p>⚠️ <b>冪等重放時 warnings 為空</b>：冪等記錄存的是 {@code ProductDetailDTO}，
 * 重放不會重跑收斂點也就沒有新的警告。這是刻意的取捨——重放代表「同一個請求」，
 * 而警告的用途是提示賣家「剛剛這次送出有一筆沒進去」，第一次回應就已經給過了。
 */
public record ProductWriteOutcome(ProductDetailDTO detail,
                                  List<CertificationWarningDTO> certificationWarnings) {

    public static ProductWriteOutcome of(ProductDetailDTO detail,
                                         List<CertificationWarningDTO> warnings) {
        return new ProductWriteOutcome(detail, warnings == null ? List.of() : warnings);
    }
}
