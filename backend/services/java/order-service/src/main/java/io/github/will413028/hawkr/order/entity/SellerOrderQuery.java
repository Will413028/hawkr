package io.github.will413028.hawkr.order.entity;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 賣家訂單列表的查詢條件（56 號設計書 §6.1.10 的 Query 契約）。
 *
 * <p>列表與筆數是兩句獨立 SQL（手寫 LIMIT/OFFSET ＋ 獨立 COUNT），條件一旦分岔，
 * {@code total} 就會與 {@code records} 對不上。故兩邊共用同一個本物件實例——
 * 條件只被組裝一次，呼叫端沒有「兩次各打一串參數」的機會。SQL 層的對應保證見
 * {@link io.github.will413028.hawkr.order.repository.OrderRepository#SELLER_ORDER_FILTER}。
 *
 * <p>各欄位皆為已正規化後的值：{@code statusList} 已拆逗號、{@code keywordPattern} 已跳脫
 * 並補上前後 {@code %}、日期已解析成 UTC {@code LocalDateTime}；null 一律代表「不篩選」。
 * 正規化邏輯在 {@code OrderService.sellerOrderQuery}。
 *
 * <p>getter 命名沿用 JavaBean 慣例而非 record accessor：MyBatis 的 Reflector 只認
 * {@code getXxx}/{@code isXxx}，改成 record 會讓 {@code #{storeId}} 在執行期解析不到屬性。
 */
public class SellerOrderQuery {

    private long storeId;
    private long buyerId;
    private List<String> statusList;
    private String keywordPattern;
    private LocalDateTime dateFrom;
    private LocalDateTime dateTo;

    public SellerOrderQuery(long storeId, long buyerId, List<String> statusList,
                            String keywordPattern, LocalDateTime dateFrom, LocalDateTime dateTo) {
        this.storeId = storeId;
        this.buyerId = buyerId;
        this.statusList = statusList;
        this.keywordPattern = keywordPattern;
        this.dateFrom = dateFrom;
        this.dateTo = dateTo;
    }

    public long getStoreId() { return storeId; }
    public void setStoreId(long storeId) { this.storeId = storeId; }

    public long getBuyerId() { return buyerId; }
    public void setBuyerId(long buyerId) { this.buyerId = buyerId; }

    public List<String> getStatusList() { return statusList; }
    public void setStatusList(List<String> statusList) { this.statusList = statusList; }

    public String getKeywordPattern() { return keywordPattern; }
    public void setKeywordPattern(String keywordPattern) { this.keywordPattern = keywordPattern; }

    public LocalDateTime getDateFrom() { return dateFrom; }
    public void setDateFrom(LocalDateTime dateFrom) { this.dateFrom = dateFrom; }

    public LocalDateTime getDateTo() { return dateTo; }
    public void setDateTo(LocalDateTime dateTo) { this.dateTo = dateTo; }
}
