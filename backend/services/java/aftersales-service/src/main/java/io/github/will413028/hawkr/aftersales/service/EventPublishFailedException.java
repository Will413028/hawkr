package io.github.will413028.hawkr.aftersales.service;

/**
 * MQ domain event 發佈失敗 sentinel（issue #2294 出路 4）。
 *
 * <p>用途：讓「永久狀態轉換 ＋ publish」的短 transaction 在事件送不出去時整個 rollback，
 * 使該筆仍滿足候選述詞、下一個 tick 重跑並重發（設計書 09 §8.5）。
 * 若不 rollback，狀態會永久離開候選集合而事件永久遺失——那正是本 issue 要修的形狀。
 *
 * <p>{@code RuntimeException} 而非 checked：它必須能穿過 Spring AOP proxy 觸發
 * 預設的 rollback 規則（{@code RuntimeException} / {@code Error}）。
 *
 * <p>排程呼叫端的處置一律是「記 error log、繼續下一筆」——不需要特別 catch 本型別，
 * 既有的 per-item {@code catch (Exception e)} 已經是正確行為。
 */
public class EventPublishFailedException extends RuntimeException {

    public EventPublishFailedException(String eventType, String caseNo) {
        super("MQ event 發佈失敗，短 tx rollback 等待下一輪重試: eventType=" + eventType + ", caseNo=" + caseNo);
    }
}
