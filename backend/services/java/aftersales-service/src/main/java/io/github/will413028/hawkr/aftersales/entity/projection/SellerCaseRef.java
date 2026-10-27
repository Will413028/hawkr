package io.github.will413028.hawkr.aftersales.entity.projection;

/**
 * 賣家售後 case 分類 projection（賣家工作台 Dashboard count 用，09 §4.4）。
 * <p>
 * 只取分類所需的 order_no / status / appeal_state 三欄，交由
 * {@code AfterSaleCaseStatusRules} 在記憶體做 canonical 分類（active / terminal-override），
 * 避免回傳完整 AfterSaleCaseEntity 但只 SELECT 部分欄位的 partial-projection 反模式
 * （docs/開發規範/Java 微服務開發規範.md §9.2）。
 */
public record SellerCaseRef(String orderNo, String status, String appealState) {}
