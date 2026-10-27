package io.github.will413028.hawkr.payuni.webhook.event;

/**
 * 代理商通知事件（商家狀態異動等）
 *
 * @param merId         商家編號
 * @param merStatus     商家狀態（S=審核通過, 1=審核通過（數字）, 其他依 PayUni 文件）
 * @param merStatusNote 狀態說明
 * @param executeTime   執行時間（格式：yyyyMMddHHmmss）
 */
public record AgentNotifyEvent(
        String merId,
        String merStatus,
        String merStatusNote,
        String executeTime
) implements PayUniWebhookEvent {
}
