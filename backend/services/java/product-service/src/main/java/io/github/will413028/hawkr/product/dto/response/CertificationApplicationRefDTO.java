package io.github.will413028.hawkr.product.dto.response;

/**
 * 冪等記錄的回應載體：只存申請 ID（19 號設計書 §16.3）。
 *
 * <p><b>為什麼只存 ID 而不是整個申請</b>：{@code idempotency_request.response_body} 是一份
 * <b>寫入當下的快照</b>。認證申請在送出後還會被 ADM 裁決、被換號 SUPERSEDE、被撤銷——
 * 存整份就等於讓「重放同一個 idempotencyKey」回傳一個可能已經過時的狀態，
 * 而呼叫端無從分辨它是舊的。存 ID 再重讀，重放拿到的永遠是當前真實狀態。
 *
 * @param applicationId 申請 TSID（字串；TSID 以數字出 JSON 是本 repo 有前例的精度事故）
 */
public record CertificationApplicationRefDTO(String applicationId) {
}
