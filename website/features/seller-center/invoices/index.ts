/**
 * 賣家面電子發票 sub-barrel（58A §B-2.2 (3)）。
 *
 * S01–S12 的畫面都已在這裡（S03 AMEGO 串接、S04 自行開票責任條款、S05 收據代製
 * 授權在 2026-08／09 陸續補齊）。新增畫面時記得同時 export，否則 `app/` 底下的
 * route 會找不到它——那是一個 tsc 才會叫、但很容易在 rebase 時被吃掉的連結。
 *
 * S12（進項分頁）不是獨立的 Page：它是 `SellerInvoiceDocumentsPage` 的 `?tab=received`
 * 分頁，實作在 `./documents/SellerReceivedInvoicesPanel.tsx`。
 */

export { SellerAmegoSetupPage } from "./amego/SellerAmegoSetupPage";
export { SellerInvoiceAppealDetailPage } from "./appeals/SellerInvoiceAppealDetailPage";
export { SellerInvoiceAppealsPage } from "./appeals/SellerInvoiceAppealsPage";
export { SellerInvoiceDocumentsPage } from "./documents/SellerInvoiceDocumentsPage";
export { SellerInvoiceJobPage } from "./jobs/SellerInvoiceJobPage";
export { ReceiptAuthPage } from "./receipt-authorization/ReceiptAuthPage";
export { SellerInvoiceDetailPage } from "./SellerInvoiceDetailPage";
export { SellerInvoiceOverviewPage } from "./SellerInvoiceOverviewPage";
export { SellerOrderInvoiceCard } from "./SellerOrderInvoiceCard";
export { SelfIssueTermsPage } from "./self-issue-terms/SelfIssueTermsPage";
export { SellerTaxStatusPage } from "./tax-status/SellerTaxStatusPage";
