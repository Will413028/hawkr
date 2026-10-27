**Will 的決定（2026-10-15）：全照建議。** §0 各題照建議；17.5 外部要求的措施開放補正（修訂 2c 設計書 G21）；冷卻 7／14／28 天、上限 28 天；條款 17.4 照 §W5 的建議文字；賣家端文案照 §W6。實作的定案項（v1 簡報的 A1–A13，含審查後的修正）見文末附錄，與本文衝突時以本文為準。

# 10e 第 2c-2 刀（補正後請求解除）：從零設計的業界基準簡報

輸入：v2 簡報（本檔附錄）、獨立審查與盤點（未進版控），SSOT `step10e-2c-design.md`（下稱 D）、`step10e-measure-brief.md`（下稱 MB），條款 `website/docs/legal/seller-terms.md` §4.3、§17、§18、§21（worktree hawkr-10e，HEAD 3998fc5f1）。外部來源在 2026-10-15 查過，逐條標註；沒有直接讀到官方原文的標 **UNVERIFIED**。

定位（沿用 MB:6）：DSA、P2B 是**參考基準**，不是台灣的法遵義務；條款是 Will 的草稿，未經律師審閱，可以修訂。系統還沒上線，所以「和既有程式一致」「改動比較小」都只算 SELF-IMPOSED。

---

## 0. 一頁摘要

| 題 | 業界基準（一句） | 建議 | 和 v2 brief 的差異 | 是否重開已決定的事 |
|---|---|---|---|---|
| P0 模型 | 商家平台的限制大多是「條件型」，條件消失就解除：Google MC、Amazon（UNVERIFIED）都是一個「請求審查」管道，裡面分「不同意」（Google 限一次）和「已修正」（可以重複，用冷卻期節流）。內容平台的 strike 是「懲罰型」，每個決定只能申訴一次（YouTube）。DSA 20、P2B 11 都沒有限制申訴次數 | **保留**「申覆一次＋補正可重複」，以及跨種類同時只能有一筆待處理；這就是 Google MC 的形狀，只是我們把「理由類型」叫做 `kind`。補一條明確規則：「懲罰型（VIOLATION）沒有補正入口」的理由是期間本身就是處分，不是條件 | 沒有實質差異；理由改用條件型和懲罰型的區分，不再只引用 Google | 不重開 17.4「申覆一次」和 G14 |
| W3 待處理時能否加重 | DSA 17(1)：每次加重都是新的決定，要附理由；20(4)：申訴成立時要「不遲延地」撤回，也就是**解除不能被程序擋住**。沒有平台公開「審查中凍結」的規則 | (a)＋：待處理期間**凍結加重**（延長、改理由）；**解除一律放行**（縮短、撤銷，以及經由「接受補正」的結束）。要加重，就先以新事實附理由駁回待處理的請求 | 補上原則「凍結加重、放行解除」，並明說縮短不構成漏洞（它對賣家有利，請求以 MOOT 結案並通知）。`CanEnd=false` 可以接受，因為有等價的解除路徑 | 否 |
| W4 冷卻 | Google MC（已查證）：第二次審查仍未解決時「可能」開始一週冷卻，之後每次失敗「可能」再延長；客服不能縮短；按鈕反灰並顯示到期日。Google Ads：一次只送一件，濫用時暫停處理 7 天。DSA 23(2)–(4)：濫用時暫停一段合理期間，要先警告，期間要寫進條款 | 同一措施上**任何種類**的請求累計被駁回 ≥2 次後，再被駁回就開始冷卻：第一次 7 天，之後每次加倍，**上限 28 天**（小於 G6 的複審上限 30 天，平台自己的複審一定會先到）。輸出 `next_remediation_time`；送出前提示「這次再被駁回會進入冷卻」 | v2 只算補正的駁回、固定 7 天。改成**連申覆的駁回也一起計數**（Google 的計數就把「不同意」算進去），並且**逐次延長、設上限** | 否 |
| A3 措施結束或撤銷時的待處理補正 | 沒有平台公開這一點（UNVERIFIED）；DSA 20(4) 和 P2B 4(3) 的精神是：決定已不存在，申訴就失去標的 | 照 v2 A3：以 `CLOSED` 結案，記 `close_cause`（REVOKED／ENDED），通知賣家；**不計入冷卻次數**；措施結束後仍可申覆（17.4 不設期限） | 補上「CLOSED 不計入冷卻」 | 否 |
| W5 條款 | P2B 11(3)、DSA 20：條款要寫出管道和運作方式、在合理期間內處理、個別附理由回覆；DSA 23(4)：濫用的限制和期間要寫進條款 | 補正寫在 17.4 的第二句，範圍限「因風險」和 17.5 外部要求；寫明回覆義務（可保存的方式、結果及主要理由、合理期間）和冷卻規則。文字見 §W5 | 範圍明確納入 17.5；加上「合理期間」；冷卻寫成逐次延長、設上限 | 否 |
| W6 呈現 | Google MC：請求的狀態（Review requested、Under review、Review not approved）；冷卻時按鈕**反灰**並顯示到期日。Merchant API：每個動作帶 `is_available`，不能用時說明原因 | RISK 措施**一律顯示**入口：可以用時是按鈕，不能用時反灰，並附原因（審核中，或冷卻到某時刻）。VIOLATION 措施不顯示。顯示最新一筆裁定的結果與理由 | v2 是依 `can_request_remediation`「顯示或隱藏」，改成「反灰並附原因」 | 否（UI 文案仍要 Will 同意） |

**結論：基準沒有推翻 17.4「申覆一次」，也沒有推翻 G14「跨種類同時只能有一筆待處理」。** 真正要改 v2 的只有 W4（計數範圍、逐次延長、上限）和 W6（反灰而不是隱藏）。另外，W3 和 A3 各補了一條原則。

---

## P0. 「申覆一次＋補正可重複」這兩種請求是不是最好的模型？

### 業界基準

| 系統 | 形狀 | 來源與驗證 |
|---|---|---|
| Google Merchant Center | 單一的「Request review」，選「I fixed the issue」或「I disagree with the issue」；「**You may only have one chance to disagree with the issue**」；修正可以重複，第二次審查仍未解決時「a one-week cool down period may begin」；警告期內有一次 courtesy review | 已查：[answer 13585221](https://support.google.com/merchants/answer/13585221?hl=en)、[answer 13693195](https://support.google.com/merchants/answer/13693195?hl=en) |
| Amazon Seller Central | 帳號停用時提出一份 appeal，內含 Plan of Action（root cause、corrective、preventive），被拒後可以改寫再送；「不同意」和「已修正」走同一個表單 | **UNVERIFIED**：官方說明在登入牆後，只找到第三方指南與論壇（例如 [myamazonguy](https://myamazonguy.com/suspensions/submit-an-appeal-to-amazon/)） |
| eBay（歐盟站） | 個案告知可否申訴；6 個月內；沒有固定次數上限，但對「經常提出明顯無理由的申訴」者可以暫停處理 | 已查：[ebay.ie help 5413](https://www.ebay.ie/help/account/regulatory/appeal-action-taken-ebay?id=5413) |
| Google Ads（帳號停權） | 「If your appeal is rejected, you can re-appeal」；「Submitting one appeal at a time」；濫用時「suspend processing of certain appeals for 7 days」；至少 6 個月 | 已查：[answer 9841640](https://support.google.com/google-ads/answer/9841640?hl=en) |
| YouTube | strike 是懲罰型（90 天失效），**每個 strike 只能申訴一次** | MB 已查證：[2802032](https://support.google.com/youtube/answer/2802032?hl=en)、[185111](https://support.google.com/youtube/answer/185111?hl=en) |
| Meta | strike 累計導出限制；申訴成立時移除 strike 與限制 | MB 已查證：[restricting accounts](https://transparency.meta.com/enforcement/taking-action/restricting-accounts/) |
| 蝦皮、Lazada | 計分制，經客服或 Account Health 申訴 | **UNVERIFIED**：只找到第三方文章，不採用 |
| DSA Art.20 | 有效、免費、易於使用；至少 6 個月；(4) 理由充分時不遲延地撤回；(5) 附理由的決定；(6) 不得只由自動化作成；**沒有次數上限**；濫用以 Art.23(2) 處理（先警告，暫停一段合理期間） | 已查（轉載本）：[springlex Art.20](https://www.springlex.eu/en/packages/dsa/dsa-regulation/article-20/)、[Art.23](https://www.springlex.eu/en/packages/dsa/dsa-regulation/article-23/)；EUR-Lex 原文讀取失敗 |
| P2B Art.4(3)、Art.11 | 給商家「clarify the facts and circumstances」的機會；撤回時不遲延地恢復；在合理期間內處理；個別告知結果；**沒有次數上限** | 已查：[Art.4](https://www.legislation.gov.uk/eur/2019/1150/article/4/adopted)、[Art.11](https://www.legislation.gov.uk/eur/2019/1150/article/11/adopted) |

**共通形狀**：限制分成兩類。
- **條件型**（帳號有風險、資料不符、待驗證）：條件消失就應該解除。平台提供「已修正，請重審」，可以重複，用冷卻期或濫用暫停來節流。Google MC、Google Ads、Amazon（UNVERIFIED）、eBay 都是這一類。
- **懲罰型**（strike、固定期間的處分）：期間本身就是處分，沒有「修正」可言，只有「這個決定錯了」的申訴。YouTube 限一次。

「不同意」的次數：法規沒有設限；平台常見的做法是限一次（Google MC、YouTube），或者不設限但節流（Google Ads、eBay）。

**「申訴一次」是不是商家平台的最佳實務？** 它是兩種主流做法之一，而且主要用在 Google（商家）和 YouTube（內容）。法規要求的是「有效、合理期間、附理由」，不是「可以無限次」。限一次的前提是：申訴的裁定者獨立且附理由（18.4 已經寫了），而且平台自己仍然可以主動撤銷（我們有 `RevokeMeasure`）。只要這兩個前提成立，賣家在申覆被駁回後出現新事證時，仍有兩條路：一是平台主動撤銷，二是（RISK 措施）走補正。所以一次的限制不會讓錯誤的決定無法修正。

### 現有機制為什麼存在
- 17.4「同一措施以申覆一次為限」：條款草稿 2026-10-07（MB:34）。
- 補正入口：MB D3 依 Google MC 和條款 4.3「補正方式」加入（MB:16）。
- G14「跨種類同時只能有一筆」：依 Google MC 同時只有一個審查（D:31）。官方頁沒有直接寫「審查中不能再送」（**UNVERIFIED**）；Google Ads 有「one appeal at a time」（已查），可以支持。

### 選項
| 選項 | 內容 | 取捨 |
|---|---|---|
| A（現狀） | 兩種 `kind`：APPEAL 一次；REMEDIATION 只給 RISK、可以重複；跨種類同時一筆 | 等同 Google MC。VIOLATION（懲罰型）只有一次申覆。被駁回後出現新事證時，只能靠平台主動撤銷 |
| B | 單一請求，只用一個 `reason_type`（DISAGREE 或 FIXED）表示 | 本質上就是 A 換個名字；`kind` 已經是這個欄位，沒有要改的東西 |
| C | 申訴不限次數，以冷卻或濫用暫停節流（Google Ads、eBay） | 對賣家最寬，但懲罰型的措施可以被反覆爭執；要加上 DSA 23 式的濫用判斷（人工、要先警告），營運成本高。要改 17.4 |
| D | 不提供補正，只靠平台的複審期限（G6 `review_by`） | 賣家沒有辦法主動觸發解除。違反 4.3「補正方式」的精神，也違反 P2B 4(3)「釐清事實」的機會 |

### 限制帳本
| 限制 | 推離基準的方向 | 性質 | 來源 |
|---|---|---|---|
| 條款 17.4「申覆一次」 | 不推離（Google 和 YouTube 就是這個基準） | — | `seller-terms.md:176` |
| 台灣沒有 DSA 或 P2B 義務 | 允許比 DSA 寬鬆，但我們沒有這樣做 | REAL（法域） | MB:6 |
| 條款未經律師審閱 | 文字會再變 | REAL | `seller-terms.md:3` |

### 建議
A，不改。在 D §5 補一句設計理由：補正只開放給條件型（RISK，含 17.5 外部要求）；VIOLATION 是懲罰型，期間本身就是處分，錯誤的決定走申覆或平台撤銷。**不重開 17.4 和 G14。**

終局與觸發：如果將來出現「申覆被駁回後才出現足以推翻的新事證」的實際案例（例如法院判決或通報撤回），考慮新增「依新事證請求重新審查」（每個措施一次，要附新事證）。目前不做，因為平台的主動撤銷已經涵蓋這種情況。

---

## W3. 有請求待處理時，平台能不能延長或修改措施？

### 業界基準
- DSA 17(1)：每個限制決定都要附理由通知；加重就是新的決定（MB 已查證）。
- DSA 20(4)、P2B 4(3)：理由充分時，要**不遲延地**撤回並恢復。所以「解除」不應該被程序擋住。
- 沒有平台公開「審查中凍結措施」的規則。Google MC 的說明沒有提到審查期間新增問題的處理（**UNVERIFIED**）。
- 現有決定：申覆提出後永久凍結延長與改理由（4A-3，D §3.6 `CanExtend` 和 `CanUpdateStatement` 都要求 `appeal==nil`）。

### 現有機制為什麼存在
4A-3：申覆的裁定者面對的必須是賣家提出申覆時的那個措施，否則平台可以在申覆期間改寫理由，讓申覆失去標的。

### 選項
| 選項 | 內容 | 取捨 |
|---|---|---|
| (a)＋ | 待處理補正期間**凍結加重**（延長、改理由）；**解除放行**（縮短照常、撤銷照常，結束改走「接受補正」）。要加重就先附理由駁回補正（理由寫新事實），再修改 | 和 4A-3 同一個原則，但只在待處理期間生效（補正可以重複，永久凍結沒有道理）。賣家一定會先收到附理由的駁回，然後才收到加重的通知（DSA 17、20(5)）|
| (b) | 不凍結 | 加重可以蓋過審查中的請求，裁定時的標的會變動 |
| (c) | 任何賣家看得到的修訂都把待處理的補正以 CLOSED 結案 | 讓管理員不附理由就清掉請求，違反 20(5) 的精神 |

critique 指出的「縮短到現在等於繞過 End」：縮短對賣家有利，正是 20(4) 要求的「不遲延地解除」。它不是規避，請求以 `CLOSED(ENDED)` 結案並通知即可（見 A3）。擋住縮短反而會讓程序阻礙解除，偏離基準。`CanEnd=false`（D:235）可以接受，因為「接受補正」就是等價而且附理由的解除路徑，解除並沒有被擋。

### 限制帳本
| 限制 | 性質 | 來源 |
|---|---|---|
| 4A-3 申覆凍結 | 不推離基準（同一個原則） | Will 2026-10-12 |
| 沒有 admin 裁定 UI（第 3 刀前） | SELF-IMPOSED（切分）；影響的只是「先駁回」那一步要等第 3 刀 | D §2 |

### 建議
(a)＋。測試加兩條：「待處理補正時縮短成功，到期後請求是 `CLOSED(ENDED)`」；「待處理補正時延長回 `failed_precondition`」。終局就是這樣，沒有觸發條件。**不重開已決定的事。**

---

## W4. 重複補正的冷卻與上限

### 業界基準（已重新查證）
- **Google MC**（[13585221](https://support.google.com/merchants/answer/13585221?hl=en)）：「If your issues aren't resolved by the second review attempt, a one-week cool down period may begin」；「The cool down period may increase with each unsuccessful review afterwards」；「Google support can't bypass or shorten the cool down period」；「The review button will be disabled (greyed out) during the cool down period」，到期日顯示在 Needs attention。冷卻也會在「exhausted the option to disagree」時開始，也就是**「不同意」也算一次審查**。審查最長 7 個工作天。帳號停權頁（[13693195](https://support.google.com/merchants/answer/13693195?hl=en)）只說冷卻前有「opportunities」、每次失敗可能延長，沒有寫次數。延長的公式沒有公開（**UNVERIFIED**）。
- **Google Ads**（[9841640](https://support.google.com/google-ads/answer/9841640?hl=en)）：可以重送，一次只送一件，濫用時暫停處理 7 天。
- **Amazon**：被拒後可以改寫 POA 重送，沒有公開的冷卻期（**UNVERIFIED**，只有第三方來源）。
- **DSA 23(2)–(4)**（[springlex](https://www.springlex.eu/en/packages/dsa/dsa-regulation/article-23/)）：對經常提出明顯無理由申訴的人，**先警告**，再暫停處理一段合理期間；要逐案評估；條款要寫明政策、考量的事實與暫停期間。

### 現有機制
目前沒有任何冷卻。v2 W4(b) 是「同一措施第二次以後的補正駁回，起算 7 天」。

### 選項
| 選項 | 內容 | 取捨 |
|---|---|---|
| (a) 不設 | — | 理由（人工、量級小）是 SELF-IMPOSED；`remediation_requests` 內嵌在讀取回應裡，沒有分頁（critique W4），次數沒有上限就會一直長 |
| (b) v2 | 只算補正的駁回，≥2 次後固定 7 天 | 比 Google 少計申覆的駁回，也沒有逐次延長 |
| (c) 基準 | 同一措施上**任何種類被駁回（REJECTED）的請求** ≥2 次之後，每次再被駁回就開始冷卻：7 天、14 天、28 天，之後都是 28 天（上限）。`CLOSED` 不計 | 計數和逐次延長都照 Google。上限 28 天是我們補的：Google 沒有公開上限，而我們的 RISK 措施必須在 30 天內複審（G6），冷卻不應該超過平台自己的複審週期。讀取回應的成長被限制在大約每 28 天一筆 |
| (d) 次數上限 | 例如 N 次後只剩平台複審 | 基準裡沒有平台這樣做；冷卻已經能控制成長，不需要 |

送出前的警告（對應 DSA 23(2)「先警告」和 Google「Make sure you understand…」）：如果這次送出後被駁回會開始冷卻，表單要顯示「若本次經駁回，將於 X 日內不得再提出」。

### 限制帳本
| 限制 | 推離基準的方向 | 性質 | 來源 |
|---|---|---|---|
| Google 延長的公式和上限沒有公開 | 我們要自己訂數字 | REAL（資料缺漏） | 13585221、13693195 |
| G6 複審上限 30 天 | 冷卻的上限要小於它 | 內部設計，但已經是 SSOT；冷卻配合它，不是推離 | D:22 |
| 少一個欄位、少一行文案 | 偏向 (a) | SELF-IMPOSED | v2 |

### 建議
(c)。契約：`Measure.next_remediation_time`（OUTPUT_ONLY，只在冷卻中出現）。`can_request_remediation` 的條件加上「not in cooldown」。計數在 model 裡由 `requests` 推導，不另外儲存。

**和 v2 的差異**：申覆的駁回也計入；逐次延長；上限 28 天；送出前警告。**不重開已決定的事**（G14 不變）。

---

## A3. 措施結束或撤銷時，待處理的補正怎麼處理

### 業界基準
- 沒有平台公開這一點（**UNVERIFIED**）。
- DSA 20(4) 和 P2B 4(3) 的邏輯：決定已經撤回或結束，請求就失去標的。DSA 20(5) 要求告知申訴人。

### 建議
維持 v2 A3 的作法（critique HIGH 的修法）：不變量是「補正只在措施 ACTIVE 時有標的」。每個需要用到「一筆待處理」名額的寫入路徑都要強制這一點；job 和 Reconcile 只負責及時，不負責正確。

補兩點：
1. `CLOSED` 帶 `close_cause`（REVOKED／ENDED），CHECK：REVOKED 必須有 admin，ENDED 可以沒有（critique LOW 的修法）。
2. **CLOSED 不計入 W4 的冷卻次數**，結案要通知賣家（G4 `MeasureRequestClosed` 已經存在，加一個欄位）。

措施結束後仍可申覆（17.4 不設期限，D:237）。**不重開已決定的事。**

限制帳本：`suspension` 不能匯入 `measure`（critique §0），所以結案程式放在 measure 套件，由 Module 層接到 job。這是 REAL（現有套件方向），但不推離基準，只決定程式放在哪裡。

---

## W5. 條款文字

### 基準
- P2B 11(3)：條款要寫出內部申訴的管道與運作方式；11(1)：在合理期間內處理；11(2)(c)：個別告知結果。
- DSA 20(5)：附理由的決定。DSA 23(4)：濫用的限制、考量的事實與期間要寫進條款。
- P2B 4(3)：撤回後不遲延地恢復。
- 條款 4.3 已經要求說明「補正方式」。18.4 的複核者規則只適用於申覆；補正的複查不要求換人（G21）。這符合 DSA 20(6) 的要求（由合格人員監督、不得只靠自動化），20(6) 並沒有要求換人。

### 建議文字（17.4 改寫；不另加一項，以免 17.5 的編號移動，proto（`seller.proto`、`governance.proto`）、設計書 32、runbook、seller `AGENTS.md`、CARRYOVER 都引用 17.5；`rg -l "17\.5"`）
> 4. 乙方得就前述措施提出事證申覆，不設期限；同一措施以申覆一次為限。甲方因第一項所定風險，或依第五項配合外部要求而採取措施者，乙方於風險消除或外部要求解除後，得檢具事證請求解除措施；此項請求不計入申覆次數，得於前次請求經回覆後再次提出。甲方應於合理期間內以可保存方式回覆結果及主要理由，並於確認風險消除時儘速解除措施。同一措施的申覆及解除請求累計經駁回二次以上者，其後每經駁回一次，乙方於七日內不得再提出解除請求，期間依次加倍，最長二十八日；甲方應於提出時告知此限制。

- 範圍：「因第一項所定**風險**」排除依違規作成的措施（critique MED：「前述措施」包含違規依據）；明確納入 17.5 的外部要求（基準 P2B 4(3)：條件消失就應該恢復）。EXTERNAL_ORDER 是否也要能補正，**這一點會改 G21 和 `can_request_remediation`（RISK 底下所有的 risk_ground 都包含在內，目前沒有排除），要 Will 確認**。建議納入。
- 「合理期間」：不寫固定天數（P2B 的用語）。內部 SLO 可以參考 Google 的 7 個工作天，但不寫進契約，這是 Will 的商業選擇。
- 版本行改成 commit 當日；已開店的賣家不要求重新同意（G20）。
- runbook：RISK 措施的 `statement_to_seller` 要寫出補正方式（4.3）。

限制帳本：條款未經律師審閱（REAL）；台灣沒有 P2B（REAL，但我們選擇照著做）。

---

## W6. 賣家端怎麼呈現請求狀態

### 基準
- Google MC（已查，13585221）：狀態分為「Review requested」「Under review」「Review not approved」；冷卻時按鈕**反灰**，並在 Needs attention 顯示到期日。
- Merchant API（MB 已查，[issueresolution v1](https://developers.google.com/merchant/api/reference/rpc/google.shopping.merchant.issueresolution.v1)）：每個動作帶 `is_available`，不能用時附原因。
- DSA 20(5)：附理由的結果。

### 建議（UI 文案，依 AGENTS 要先取得 Will 同意）
- RISK 措施：「已改善，請求解除」**一律顯示**。不能用時反灰並附原因：「解除請求審核中」「申訴審核中」「YYYY-MM-DD HH:mm 後可再提出」。VIOLATION 措施不顯示（它不是條件型，顯示反灰的按鈕會誤導）。
- 狀態優先順序（critique MED）：待處理的請求 > 冷卻 > 最新一筆已裁定的結果。最新一筆已裁定的請求（不分種類）顯示結果與理由；較舊的請求放在詳情頁（seller-app 的詳情頁屬於 A8）。
- 送出表單：沿用申訴的表單（參數化 `SellerAppealDialog`，見 inventory §6），加上 W4 的冷卻警告。
- 原因由 server 推導：client 只讀 `can_request_remediation`、`next_remediation_time` 和 `requests[].state`，不自行計算冷卻（AIP-216，同 G16）。

**和 v2 的差異**：從「依布林隱藏」改成「反灰並附原因」。

---

## 實作清單裡受本簡報影響的增量（其餘照 v2 和 critique §38）
- model：計算冷卻（從 REJECTED 的次數推導）、`NextRemediationTime`；`CanRequestRemediation` 加上冷卻條件。
- proto：`Measure.next_remediation_time`、`MeasureRequest.close_cause`。
- schema：`close_cause` 和它的 CHECK。
- 測試：W3 的兩條；W4 的計數（申覆駁回也計入、CLOSED 不計入、7→14→28→28 天、上限）；A3 的 CLOSED 不計入。
- 條款 17.4、runbook；website 和 seller-app 的反灰狀態與冷卻警告。

## 還沒有驗證的
- Amazon 的申訴、POA、重送規則：官方頁在登入牆後，只有第三方來源。
- 蝦皮、Lazada：只有第三方來源，不採用。
- Google MC：審查期間能不能再送、審查期間新增問題怎麼處理、問題自行消失時審查會不會關閉、冷卻延長的公式和上限：官方頁沒有寫。
- DSA：EUR-Lex 原文讀取失敗，只核對了 springlex 的轉載本。
- 條款文字沒有經過律師審閱。

---

## 附錄：依基準直接定案的實作項（v2 簡報 A 段，審查後修正；與上文衝突時以上文為準）


| # | 定案 | 依據 |
|---|---|---|
| A1 | 佐證與申訴完全共用（請求層的 evidence、讀取紀錄、檔案路由、機敏上傳），零 schema 改動 | G18 |
| A2 | 申訴被駁回後仍可補正：`can_request_remediation` = RISK ∧ ACTIVE ∧ 沒有待處理請求 ∧ 不在冷卻（W4） | G14、§5、measure-brief D3 |
| A3 | **補正只在措施 ACTIVE 時有標的**：`CanDecideRemediation` 要求 ACTIVE；`CanAppeal` 不把「非 ACTIVE 措施上的待處理補正」算成待處理；`SubmitAppeal` 在同一個 transaction 先把這種補正以 `CLOSED` 結案再寫申訴。到期 job 與每小時的 Reconcile 也結案（只為了及時，正確性不依賴它們；Reconcile 本來就為 job 遺失而存在）。CHECK 放寬：CLOSED 可無 `decided_by_admin_id`（系統結案）；`MeasureRequestClosed` 加 `Cause`（REVOKED／MOOT）讓 A8 區分 | 審查 HIGH：job 遺失時請求會永久佔住 `one_pending` 而擋掉 17.4 的申覆 |
| A4 | 第 3 刀前 admin 沒有裁定介面：接受（§2 切分；第 3 刀緊接著做）。待處理補正時「結束」按鈕依 `can_end` 消失是正確行為 | §2 |
| A5 | `appeal` 只放申訴，補正在 `remediation_requests`（新到舊），不加 kind enum；ACCEPTED 的註解依欄位分開寫 | §5 |
| A6 | 接受補正＝寫一版 `ends_at=now`（作者＝裁定者，statement 不變）＋Recompute＋`MeasureEnded`；認定、`revoke_reason` 不動；結束後仍可申訴。抽出 End 的核心給兩者共用 | §5「不是撤銷」、§3.6 |
| A7 | `remediation_state` 是集合篩選（有任一筆此狀態的補正）；`=PENDING` 加入既有的收件匣排序（優先序 `review_due` > 申訴待裁定 > 補正待處理，沿用現有 switch，組合仍合法）；FilterScope 加新欄位 | 既有契約 `list.go:147`；第 3 刀的合併收件匣 |
| A8 | **修訂 §5**：`DecideRemediation` 不帶 `request_id`。同一措施最多一筆待處理請求，etag 已保證裁定的是管理員看到的那筆；而且 TS 與 Dart client 看到名為 `request_id` 的欄位就當冪等鍵自動重試（`errors.ts:174-179`、`errors.dart:219-225`） | G14、AIP-155 |
| A9 | etag：提出、駁回、系統結案只讓後台 version 加一；接受寫新版，兩種 etag 都變 | G10、G11 |
| A10 | **修訂 §2「只增值」**：`measure_requests` 加 `basis`（migration 由 measures 回填），複合 FK `(measure_id, basis) → measures (id, basis)`，CHECK `kind <> 'REMEDIATION' OR basis = 'RISK'`；devseed 測試的直接 INSERT 跟著改 | G13 以 DB 保證依據不變量；「只增值」是切分時的估計，不是限制 |
| A11 | `MeasureEnded` 不加原因：沒有訂閱者，同一 transaction 的 `MeasureRequestDecided{REMEDIATION, ACCEPTED}` 可關聯；觸發是 A8 通知 | G4 |
| A12 | 讀取時一次讀全部請求的佐證（不 N+1） | — |
| A13 | 補正裁定不擋原決定者 | G21 |


---

## 實作後的設計審查（Will 2026-10-15 全照建議）

design-review 發現兩個契約層的問題，簡報由未參與的 subagent 起草：
- **D1 請求的契約形狀（取代上文附錄 A5、A7）**：`appeal` 與 `remediation_requests` 合成帶 `kind` 的 `requests`（新到舊）；請求的對外狀態由伺服器推導：措施不在 ACTIVE 時，儲存為 PENDING 的補正輸出 `CLOSED`／`ENDED`、`decide_time` 為措施結束時間（job 寫回時用同一個時間，收斂前後輸出相同；DB 唯一索引仍看儲存值）；不能請求解除的原因由伺服器輸出（`remediation_unavailable_reasons`，依優先序，Google Merchant API `is_available`＋`reasons` 的做法），client 不再推導；後台篩選 `appeal_state`、`remediation_state` 換成 `request_state`＋選填 `request_kind`，待處理收件匣一條查詢，條件與推導的狀態同義並以資料集測試鎖住。理由：兩個 client 各自重複推導、措施結束到 job 結案之間兩端同時顯示「可以申訴」與「審核中」；舊形狀的約束全是 SELF-IMPOSED。
- **D2 待處理補正的時限**：同時一筆待處理（G14）符合基準，缺的是時限。內部 SLO 7 個日曆天，寫進 runbook 與設計書 32，第 3 刀的收件匣顯示等候天數並標出超時；契約不變、不加撤回。觸發：第一個要求撤回的實際案例，或上線後裁定中位數超過 SLO。
- 已結束措施上待處理請求的結案改由 measure 自己的 job 與掃描（只掃已結束措施的待處理請求），不再掛在停權的到期 job 與對帳上。
