# B2 步驟 7：PayUni 相關 ADR 的結論

範圍：`rg -l -i payuni docs/adr`（43 份，dev `3ca78720c`）。唯讀 subagent 逐份判讀，主 session 抽查 `2026-11-13-payment-onboarding-provider-port-stripe.md:60`、`2026-09-18-invoice-service-architecture.md:132`、`2026-09-18-webhook-two-phase-transaction.md`、`2026-09-11-channel-fee-grossup-in-adapter.md` 的狀態列與引文，與表內一致。

處置：SUPERSEDED-BY-B2 的 2 份在步驟 7 改狀態列；PARTIALLY-AFFECTED 中由 B2 造成的 3 份（Stripe ADR :60、:211，單體 ADR :61、:188，invoice ADR :132）在步驟 8 改寫該句；`2026-09-18-webhook-two-phase-transaction` 隨 B3 刪 `payuni-webhook-service` 時標 superseded；其餘 PARTIALLY-AFFECTED 與狀態列漏標的不是 B2 造成（賣家 PayUni 開通在 B2 之前已移除、物流隨 2026-11-15），步驟 8 搬回主計畫的待辦。

| file | status-line-now | conclusion | evidence file:line | note |
|---|---|---|---|---|
| 2026-06-29-identity-defer-store-entity | accepted | STILL-VALID | :7,:11 | PayUni MerID 只是動機；決策（stint 在 seller aggregate）不變。 |
| 2026-07-07-identity-distributed-principal | accepted | STILL-VALID | :11-12 | MerID 一次性綁定是歷史理由，已無對象；決策不變。 |
| 2026-07-11-identity-store-stint-symmetry | accepted；A3 核對「PayUni 的 MerID 一次性綁定仍成立，沿用」 | PARTIALLY-AFFECTED | :5（狀態列）、:20、:25；`20261114055147...sql:45` | 「MerID 一次性綁定仍成立」已不實：`payuni_registrations` 已刪、`payuni_mer_id` 回空字串（`internal/legacy/sellerserver/server.go:175`）。stint remint 的決策仍成立，改寫該句即可。 |
| 2026-07-20-seller-kyc-pii-encryption-scope | superseded by 2026-11-13-seller-platform-review-and-identity-matching | STILL-VALID | :5 | OTHER-SUPERSEDED（KYC，非 B2）。 |
| 2026-08-29-membership-billing-payuni-period | accepted | STILL-VALID | :67；`2026-10-27-general-ecommerce-scope-cut.md` | 會員計費已在 10-27 scope cut 刪除，本檔 :67 自己註明「其餘各節是已刪除的流程」。非 B2；狀態列仍寫 accepted，建議改 superseded by scope-cut。 |
| 2026-09-06-fulfillment-docs-split-into-six | accepted | STILL-VALID | :86 | 物流，B3；已附 11-15 退場註記。 |
| 2026-09-06-uber-webhook-slice-in-payuni-webhook-service | superseded by 2026-11-15-fulfillment-seller-managed-shipping | STILL-VALID | :1-7 | OTHER-SUPERSEDED（B3 物流）。 |
| 2026-09-11-channel-fee-grossup-in-adapter | accepted | SUPERSEDED-BY-B2 | :10 `BalType=1` 扣 1% 後撥款、:44 `Amt` 值域、:56 「payment collection adapter（PayUni 專屬）」；b2 plan 前提（PayUni 專屬流程直接刪）、範圍「不做」:51（平台手續費與對帳報表 B10 退場） | PayUni `agent_fee`／BalType gross-up adapter 隨 PayUni 程式刪除。取代它的 Stripe ADR 沒有「通道費誰負擔」的對應段落（只有 `2026-11-13-payment-onboarding-provider-port-stripe.md` D1 收款 adapter 與 D20 修訂：SCT、賣家餘額不足由平台吸收）。建議狀態改 superseded by 該 Stripe ADR。 |
| 2026-09-11-ledger-carrier-model-compatibility | superseded by 2026-11-15-... | STILL-VALID | :63 | OTHER-SUPERSEDED（物流財務已刪）。 |
| 2026-09-11-logistics-amount-semantics-separation | accepted | STILL-VALID | :97,:103 | OTHER-SUPERSEDED 實質（物流財務結算已整體刪除，commit `5883f499e`、`125db5844`、`28a120564`，b2 plan 前提）；兄弟檔 ledger-carrier 已標 superseded，本檔狀態列漏標，建議補 superseded by 11-15。 |
| 2026-09-11-logistics-cost-true-up | accepted | STILL-VALID | :21,:45,:72 | 同上，狀態列漏標；其中「賣家負擔 PayUni 收款手續費」那項隨 B2 一併無對象。 |
| 2026-09-11-logistics-finance-domain-ownership | superseded by 2026-11-15-... | STILL-VALID | :28 | OTHER-SUPERSEDED；:28「PayUni Collection 歸 payment-service」是歷史。 |
| 2026-09-11-payuni-channel-fee-bearer | accepted | SUPERSEDED-BY-B2 | :1（標題）、:10、:24；b2 plan 前提:14-16、範圍:51 | 規則「PayUni 通道手續費由賣家負擔」只對 PayUni agent_fee 成立；Stripe 路線沒有等價段落（Stripe ADR 未決定費用負擔方，平台手續費留給已退場的 B10）。建議標 superseded by Stripe ADR D1／D20。 |
| 2026-09-11-responsibility-based-cost-attribution | accepted | STILL-VALID | :109 | 物流成本責任判定已刪（b2 plan 前提），OTHER-SUPERSEDED 實質（B3 11-15），狀態列漏標。PayUni 只是附帶。 |
| 2026-09-11-uber-commerce-model-seller-direct | superseded by 2026-11-15-... | STILL-VALID | :24,:26,:32 | OTHER-SUPERSEDED（B3）。 |
| 2026-09-12-inbound-payload-raw-bytes | accepted | STILL-VALID | :11,:27,:40,:42；`services/java/shipping-service/.../provider/capability/InboundPayload.java`、`provider/payuni/AbstractPayUniAdapter.java` | 對象是 shipping-service 的物流 provider 入站介面（`formParams`），屬 B3 刪除範圍，B2 不動。 |
| 2026-09-12-return-leg-classifier-canonical | accepted | STILL-VALID | :1,:11,:14,:29,:33,:51 | 逆向物流貨態分類（`PAYUNI_RETURN_*` 是物流狀態碼，非付款），B3。 |
| 2026-09-18-adm-delegation-authority-test | accepted | STILL-VALID | :7 | 只提 repo 名 `backend-payuni`，附帶。 |
| 2026-09-18-buyer-membership-entitlement-model | accepted | STILL-VALID | :43,:46,:102,:112 | 週期公式對齊 PayUni 定期定額，:112 已自註只剩公式成立；會員計費被 scope cut 刪除，非 B2。 |
| 2026-09-18-ga4-cross-platform-analytics | accepted | STILL-VALID | :80 | PayUni webhook 是被否決的替代案，附帶。 |
| 2026-09-18-invoice-service-architecture | accepted；ADR-01 superseded by go-modular-monolith | PARTIALLY-AFFECTED | :132「`provider_paid_at` 是 PayUni 認為的真實付款時間」；`services/invoice-service/internal/client/payment/client.go:34`、`client_test.go:99-109`、`db/schema/schema.sql:324` | 欄位仍在（invoice 經 legacy gRPC 讀 payment 的 provider_paid_at），但來源改為 Stripe 付款成功時間，「PayUni 認為」已不實；期別換算決策不變。:39,:51,:54,:55,:89,:191 的 PayUni 是被否決的替代案，仍有效。 |
| 2026-09-18-seller-capability-authorization | D3、D4、D7 superseded（A3）；已含 PayUni 刪除修訂 | STILL-VALID | :115,:125,:135,:143 | :135-143 已寫明 PayUni 開通刪除、改看轉帳能力；不需再改。 |
| 2026-09-18-webhook-two-phase-transaction | accepted | PARTIALLY-AFFECTED | :1,:16,:40,:45,:47-49；b2 plan 前提（PayUni 付款流程由「Stripe webhook（inbox、event id 去重）」取代）與「本階段依賴的不變量」webhook 列；Stripe ADR `2026-11-13-payment-onboarding-provider-port-stripe.md` 繼承段:23、決策「Stripe API 與官方 SDK」:77、`internal/platform/inbox` | 主 session 改判（2026-11-25）：兩段交易仍是 `payuni-webhook-service` 剩下的 Uber 切片的機制（`webhook/service/WebhookInboxTxOps.java`），只有金流部分被 Stripe inbox＋event id 去重取代（Stripe ADR 繼承段、`internal/platform/inbox`）。整份隨 B3 刪服務時標 superseded。決策是 payuni-webhook-service「兩段交易＋靠 PayUni 重送回收」，金流部分隨服務刪除；新機制是 platform inbox＋event id 去重＋重讀物件。注意同一服務的 Uber 切片在 B3 才刪（11-15 已 superseded Uber ADR），故服務整體在 B2＋B3 後才消失；:47 「Gateway 對 PayUni 回 502」約束隨 gateway 付款路由刪除（步驟 6B）。 |
| 2026-09-20-seller-stint-assistant-history | accepted | STILL-VALID | :17 | 附帶。 |
| 2026-09-24-go-modular-monolith | accepted；D7 superseded | PARTIALLY-AFFECTED | :61「payment｜payment-service：PayUni 收款、退款、撥款 API，會員訂閱計費」；:188「呼叫 PayUni、AMEGO、Uber…」；:194「payment 的 `createRefund` 在 transaction 內呼叫 PayUni」；:196「seller 的 `Approve` 在 transaction 外呼叫 PayUni」 | :61 的來源描述不再對應（現為 Stripe SCT 的 payment 模組，會員計費已刪）；:194、:196 是舊程式的反例描述（歷史，現已改 River job／provider port），:188 的外部清單 PayUni 應為 Stripe。決策（模組化單體）不變，建議在 B2 完成的修訂段處理。 |
| 2026-09-25-codemeta-retirement | accepted | STILL-VALID | :35,:108 | 通則「第三方代碼不能改名」不變；B2 6B 刪付款值域只是執行。 |
| 2026-09-25-openapi-toolchain | superseded by 2026-09-26-external-contract-protobuf-connect | STILL-VALID | :59 | OTHER-SUPERSEDED（Connect）。「PayUni 回跳」端點已隨 6A2 刪除（PaymentReturnController）。 |
| 2026-09-25-optimistic-concurrency-etag | accepted | PARTIALLY-AFFECTED | :60「RebindPayUni…不帶 etag」 | `RebindPayUni` 已隨 PayUni 開通刪除（`seller/CARRYOVER.md` 標已刪；seller-platform-review :83）；原則（依「狀態檢查擋不擋得住過時決定」分）不變。屬 KYC 刪除，非 B2。 |
| 2026-10-02-seller-governance-in-monolith | superseded by 2026-10-18-seller-governance-consolidated | STILL-VALID | :190,:191 | OTHER-SUPERSEDED（整合）；PayUni 只出現在引用舊 ADR 名稱。 |
| 2026-10-02-seller-kyc-payuni-in-monolith | superseded by 2026-10-19-seller-kyc-payuni-consolidated | STILL-VALID | 34 處 | OTHER-SUPERSEDED（最終被 2026-11-13 seller-platform-review 取代；開通改 Stripe 的 D1／D6／D11 在 Stripe ADR）。非 B2。 |
| 2026-10-03-media-references-owner-side | accepted | STILL-VALID | :55 | 「payuni 同步已有的同一種保底」是比喻，`payuni_sweep` 已刪（seller-platform-review :83）；media 保底掃描本身不受影響。 |
| 2026-10-03-public-store-identity-and-visibility | accepted | PARTIALLY-AFFECTED | :15「`model.Exposable`：…KYC 與 PayUni 流程核准…」、:19「PayUni MerID＝`HAWK`＋code」、:50、:56-57；`backend/internal/modules/seller/seller.go:470`（現行閘門：平台審核核准且轉帳能力啟用） | 曝光條件不再含 PayUni（改為開通閘門），MerID 已不存在，`store_code` 仍是對外店號。「不得逐號列舉」的決策與 `first_exposed_at` 仍成立。非 B2（onboarding ADR 已改規則）。 |
| 2026-10-03-recommended-sellers-scoring | accepted | PARTIALLY-AFFECTED | :17、:45「KYC 與 PayUni 的流程已核准」、:50「（KYC 與 PayUni 都核准）」 | 資格條件字面已不實，改為「可公開曝光」（同上 `seller.go:470`）；評分決策不變。 |
| 2026-10-04-address-town-code | accepted | STILL-VALID | :48,:78 | 「PayUni 已經送『臺』」只是理由旁註。 |
| 2026-10-05-notification-module | accepted | PARTIALLY-AFFECTED | :63「賣家：…KYC、PayUni、7-ELEVEN 開通結果…」 | PayUni 開通通知已停發（Stripe ADR D18；seller-platform-review :84），7-ELEVEN 開通屬 B3。事件清單需改為收款帳戶開通結果。 |
| 2026-10-05-store-exposure-to-legacy | accepted | PARTIALLY-AFFECTED | :13、:15「KYC 流程核准（含 PayUni）」、:28 | :15 描述的 `Exposable` 條件已改（同上）；`can_accept_orders` 舊推導（:13）是歷史；B2 計畫步驟 6 已把 Java checkout 改看可接單。曝光對 Java 的契約決策不變。 |
| 2026-10-18-seller-governance-consolidated | accepted | STILL-VALID | :79,:80,:137,:164 | 只引用已被取代的 kyc-payuni 檔談保留期；seller-platform-review 明載「兩段清理」仍成立。建議把引用改指 2026-11-13 檔（文件衛生，非 B2）。 |
| 2026-10-19-seller-kyc-payuni-consolidated | superseded by 2026-11-13-seller-platform-review-and-identity-matching | STILL-VALID | 34 處 | OTHER-SUPERSEDED（非 B2）。 |
| 2026-10-20-admin-review-queue | accepted | STILL-VALID | :62,:66,:72 | 已寫明刪 PayUni 重新綁定種類、新增 `PAYMENT_ACCOUNT_CREATE_FAILED`。 |
| 2026-10-27-general-ecommerce-scope-cut | accepted | STILL-VALID | :41,:56,:58,:60 | 範圍裁剪紀錄；引用的 PayUni ADR 狀態變化已各自處理。 |
| 2026-11-13-payment-onboarding-provider-port-stripe | accepted（D1–D69） | PARTIALLY-AFFECTED | :60「PayUni 的付款、退款、撥款與物流留在 Java 舊服務，由單體計畫的 B2、B3、B8–B10 搬」；:211「本機的付款與物流端到端在付款、物流模組搬進單體前不通」 | 這是取代方，不是被取代。付款部分已過時：B2 已刪 Java payment-service，付款走單體 payment（b2 plan 步驟 6 PR #489）；物流改賣家自寄（11-15）。決策不變，兩句建議在 B2 步驟 8 收尾時改寫（:211 同時僅剩物流端到端不通）。D20 修訂已完成。 |
| 2026-11-13-seller-platform-review-and-identity-matching | accepted（整合並取代 10-19 檔） | STILL-VALID | :25,:27,:83,:84,:89,:93 | 是 KYC 方面的取代方；PayUni 全是「刪除」敘述，仍準確。 |
| 2026-11-15-fulfillment-seller-managed-shipping | accepted | STILL-VALID | :7,:12,:15,:27,:34 | 物流，B3 的 SSOT；D4「不搬 PayUni 物流與 payuni-webhook-service 的物流部分」與 B2 切割一致（B2 只刪金流部分）。 |
