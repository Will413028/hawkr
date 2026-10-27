package io.github.will413028.hawkr.product.dto.request;

import com.fasterxml.jackson.annotation.JsonInclude;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.Serializable;

/**
 * 行內片段 DTO（inline run；02 §3.6 inline-run 模型）
 * 對應 gRPC InlineRun。
 *
 * <p>text block 的 {@code runs} 與 table cell 的 {@code runs} 皆用此型別。
 * 僅序列化非 null 欄位（bold/italic/underline 僅 true 時存在，維持正規形）。
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
@JsonInclude(JsonInclude.Include.NON_NULL)
public class InlineRunDTO implements Serializable {
    private static final long serialVersionUID = 1L;

    /** 片段純文字（禁 HTML tag） */
    private String text;
    /** 行內粗體（僅 true 時存在） */
    private Boolean bold;
    /** 行內斜體（僅 true 時存在） */
    private Boolean italic;
    /** 行內底線（僅 true 時存在） */
    private Boolean underline;
    /** 行內色碼 #RRGGBB（空=無） */
    private String color;
    /** 行內連結 http(s):// 絕對 URL（空=非連結） */
    private String href;
}
