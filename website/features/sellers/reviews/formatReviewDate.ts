/**
 * 賣家評價日期格式化。
 *
 * 實作與「為什麼不用 `toLocaleDateString("zh-TW")`」（hawkr-website#374）
 * 全部收斂在 `@/lib/date/taipei`——同一個 root cause 在 PDP 評價、賣家中心
 * 客戶／優惠券／商品評論、會員卡明細、瀏覽紀錄都出現過，共用一份實作才不會
 * 修一處漏七處。本檔只保留評價列表用得慣的名字。
 */
export { formatTaipeiDate as formatReviewDate } from "@/lib/date/taipei";
