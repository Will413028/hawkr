# 通報流程的業界與法規基準（研究，2026-10-18）

研究者是 subagent；讀過的一手來源下載在本機（未進版控）。標記：VERIFIED（讀過一手來源）、SECONDARY（新聞、部落格、搜尋摘要）、INFERENCE（推論）。全國法規資料庫的資料截止日 115-10-08。

## 摘要
1. DSA：任何人都能通知；只有留了聯絡方式的通知人才會收到收件確認；決定通知須附救濟方式（16(1)(4)(5)）。前言 50 允許匿名、一則通知可列多項（VERIFIED）。理由書同時涵蓋單一商品的可見度限制與帳號停權，須寫條款依據、事實、是否自動化、救濟途徑；通知人也能對「不處置」申訴，至少 6 個月（17、20）。對濫報者先警告再暫停受理（23）。小微平台不適用 Art 20–32，Art 16–18 照樣適用（VERIFIED）。
2. P2B 第 4 條：限制個別商品時，理由書在生效前或生效時給；全面終止提前 30 日；理由書引用第三方通知的內容，並給賣家澄清事實的機會（VERIFIED）。
3. 台灣食安法現行沒有平台條款，也沒有第 9-1 條（全文 grep「網路｜網際網路｜平臺｜平台」為 0，VERIFIED）。監察院 2026-06 報告：衛福部說「電商平臺僅被視為數位賣場提供者」；實務是食藥署查到違規後請平台下架，「通常2天內」完成（VERIFIED）。
4. 行政院 2026-07-23 通過食安法修正草案（VERIFIED 草案原文）：新增第 43-1 條，平台接獲主管機關通知後兩個工作日內限制瀏覽或移除；自收到通知起保存行為人個資與網路使用紀錄 180 日；建立風險管控機制；免負賠償責任；違反依第 48-2 條罰 10 萬至 50 萬。沒有找到三讀報導（SECONDARY，負面結論）。
5. 詐欺犯罪危害防制條例第 36–38 條是現行法唯一明文規範「電商業者」配合義務的條文（VERIFIED）：經通知後配合處理、暫停涉詐帳號、收到調取通知三日內提供資料、免責。
6. 著作權法 ISP 程序（VERIFIED）：通知六項要素；不合格式時平台與權利人的補正期各 5 個工作日；回復通知後權利人 10 個工作日內提出起訴證明，否則平台最遲 14 個工作日內恢復。商標法沒有 ISP 條文（VERIFIED）。
7. 食安檢舉的行政面（VERIFIED）：主管機關七日內移送管轄機關、三十日內回覆檢舉人；匿名不予獎勵；行政程序法第 173 條：未具真實姓名或住址的陳情得不予處理。個資法第 20 條：交出檢舉人資料需法律明文、公共利益或當事人同意等依據。推論：轉介時以平台名義當檢舉人，檢舉人身分只在取得同意後提供（INFERENCE）。
8. GPSR 第 2(2)(b) 條明文不適用食品（VERIFIED）；食品在歐盟的基準是 General Food Law 178/2002 第 19 條。GPSR 第 22 條只當平台義務的參考：機關命令 2 個工作日、產品安全通知 3 個工作日、直接通知已購買者、經常違規的賣家先警告再停權。Pledge+ 同樣排除食品。
9. 工單與案件系統都沒有獨立的「調查中」狀態，分的是「在等誰」（VERIFIED）：Zendesk 的 Pending 等提出者、On-hold 等第三方，SLA 依此暫停；merge 是永久的、會關閉來源工單。ROOST Coop（開源）同一個目標只開一個 job，多筆檢舉累積在 job 的 reportHistory（VERIFIED 原始碼）。
10. DSA 透明資料庫的理由書 schema 可直接當「決定」的資料模型（VERIFIED）：商品可見度、服務、金流、帳號四個維度各自獨立、各有結束日；另有來源類型、依據（違法或違反條款）與自動化旗標。
11. 權利人管道與回饋：eBay 移除時把權利人的姓名與聯絡方式給賣家（VERIFIED）；Yahoo 拍賣不給權利人賣家資料，理由是個資法（VERIFIED），且把一般檢舉公開顯示在商品頁（食安法第 46-1 條處罰散播食安謠言，是否公開顯示是 INFERENCE）；露天 PUB 把會員檢舉轉給權利人確認，24 小時內下架（VERIFIED）；Amazon Report a Violation 有即時狀態追蹤（VERIFIED）；Amazon 與蝦皮的「要求賣家補件、期限、申訴」只有二手資料（SECONDARY）；momo、PChome 沒有公開文件。

## 1. EU DSA 與 P2B（全部 VERIFIED，EUR-Lex CELEX:32022R2065、32019R1150）
- 16(1) 任何人可通知、只以電子方式：「allow any individual or entity to notify them of … specific items of information that the individual or entity considers to be illegal content」。
- 16(2) 要素：理由、精確位置、姓名與 email（CSAM 例外）、善意聲明。
- 前言 50：「should allow, but not require, the identification of the individual or the entity submitting a notice.」；可「notify multiple specific items … through a single notice」。
- 16(4)：「Where the notice contains the electronic contact information … shall, without undue delay, send a confirmation of receipt」。
- 16(5)：「notify that individual or entity of its decision in respect of the information to which the notice relates, providing information on the possibilities for redress」。
- 16(6)：「in a timely, diligent, non-arbitrary and objective manner」；用了自動化要在決定通知中說明。
- 前言 52：涉及生命安全時「act without delay」。
- 17(1)(a)：「any restrictions of the visibility of specific items of information …, including removal of content, disabling access to content, or demoting content」；(d)「suspension or termination of the recipient of the service's account」。
- 17(2)：「at the latest from the date that the restriction is imposed」。
- 17(3)：(b) 來源（通知或主動調查，必要時通知人身分）；(c) 自動化；(e)「a reference to the contractual ground relied on and explanations…」；(f) 內部申訴、訴訟外、司法救濟。前言 54：只在必要時揭露通知人（例如智財）。
- 9(5)：機關命令仍要告知被處置者。Art 3(t)：content moderation 同時涵蓋內容可見度與帳號能力。
- Art 18：涉及生命安全的刑事嫌疑「promptly inform the law enforcement or judicial authorities」。
- Art 20：通知人也可申訴，至少 6 個月；不處置也可申訴；有理由時「reverse its decision … without undue delay」；不得只靠自動化。
- Art 22 受信任通報者優先處理。Art 23：對經常提供明顯違法內容者與經常提出明顯無據通知者，都「suspend, for a reasonable period of time and after having issued a prior warning」；政策寫進服務條款。
- 規模豁免：19(1) Section 3（Art 20–28）不適用小微平台；Art 29 對 Section 4（30–32）同樣豁免；16–18 沒有豁免。
- 24(5)：決定與理由書送 Commission 公開資料庫，不含個資。
- 30(3)：「request that the trader remedy that situation without delay or within the period set…」，不補正「swiftly suspend」；30(4) 賣家可申訴。31(3) 隨機查核官方資料庫。32：通知「insofar as it has their contact details, consumers who purchased the illegal product」，回溯 6 個月；聯絡不到的在介面公告。
- P2B 4(1) 個別商品的限制「prior to or at the time of the restriction or suspension taking effect」給理由書；4(2) 全面終止「at least 30 days prior」；4(4) 例外；4(3)「give the business user the opportunity to clarify the facts and circumstances」；4(5) 理由書含「contents of third party notifications」；11 內部申訴（small enterprise 豁免）。

## 2. 台灣法規
- 食安法（108-06-12，VERIFIED）：沒有平台條款；第 3 條第 7 款食品業者含「運送、貯存、販賣」（INFERENCE：若平台自營倉儲、冷鏈或以自己名義販賣，本身就是食品業者）；第 7 條第 5 項「應即主動停止製造、加工、販賣及辦理回收，並通報直轄市、縣（市）主管機關」；第 9 條第 1 項食品業者保存來源文件；第 29 條傳播業者保存委託刊播者資料六個月；第 41 條機關得要求「食品業者」提供來源、數量、販賣對象等（對象不是平台）；第 43 條對檢舉人身分保密並得獎勵；第 46-1 條散播食安謠言罰則；第 47 條拒絕查核、拒不提供資料罰則；第 52 條命回收、公布業者。
- 監察院 2026-06-17 報告（VERIFIED）：「電商平臺僅被視為數位賣場提供者」；「若查獲違規則請電商平臺配合下架，平臺業者通常2天內完成下架」；缺口「未明確課予電商平臺業者審核上架商品之責任義務，亦未強制要求平臺業者覈實賣家身分資訊」；114 年違規 7 成是無須登錄的個人賣家；衛福部定型化契約應記載事項：販售頁揭露「食品業者登錄字號」。
- 行政院食安法修正草案（2026-07-23，VERIFIED 草案原文）：第 3 條第 13 款「網際網路業者」；第 43-1 條「應於收受通知之次日起二個工作日內，限制瀏覽或移除該違法資訊」、建立「違法資訊風險管控機制及必要措施」、免負賠償責任、保存「行為人之個人資料及網路使用紀錄資料，應自收受第一項主管機關通知之日起保存一百八十日」；第 43-2 條 ISP 限制接取；第 43-3 條輸入食品揭露許可；第 48-2 條罰 10 萬至 50 萬並按次處罰；草案第 7 條「應自發現或知悉二十四小時內，通報」。三讀：未找到（SECONDARY）。
- 衛福部網頁（105-12-01，VERIFIED）：平台應查明輸入食品適法性，不符時「要求賣家修正資訊或逕行將產品資訊下架」；善良管理人；引消保法第 23 條。食藥署 2011 函釋（VERIFIED）：平台是行為人或傳播業者「應依實際相關事證據以核判」；「對任何人使用該網站均應有紀錄可稽」。1919 食安專線「全程錄音、密件登錄」（SECONDARY）。
- 檢舉辦法（VERIFIED）：第 2 條檢舉要載姓名、身分證號、聯絡方式、地址；無管轄權時「七日內移送該機關，並通知檢舉人」；第 3 條「三十日內，通知檢舉人」；第 7 條匿名不予獎勵；第 8 條多人檢舉同一事，獎金給最先檢舉者；第 9 條保密。行政程序法第 173 條：「無具體之內容或未具真實姓名或住址者」得不予處理。INFERENCE：以平台名義轉介可避開匿名不受理，並收到 30 日內回覆；轉介可設「等待主管機關回覆」，期限 30 日，遭移送時再加 7 日。
- 消保法（VERIFIED）：第 10 條有危害之虞的商品「應即回收」；第 18 條第 5 款通訊交易告知申訴方式；第 23 條媒體經營者明知或可得而知廣告不實時連帶責任；第 33 條地方政府調查時可查詢。
- 詐欺犯罪危害防制條例（VERIFIED）：第 2 條第 7 款「電商業者」定義；第 36 條第 3 項配合並暫停涉詐帳號；第 37 條第 2 項「三日內提供資料，並保存六個月」；第 38 條免責；第 41 條罰則。
- 數產署 2023-07-13（VERIFIED）：「業者有審核下架或配合主管機關通知後下架之義務」。2026-05 關鍵字通報平台（SECONDARY）。
- 著作權法第 90-4 至 90-12 條與實施辦法（VERIFIED）：90-4 告知三次侵權終止服務、公告窗口；90-7 接獲通知「立即移除或使他人無法進入」；90-9 轉送使用者、回復通知轉送權利人、10 個工作日起訴證明、否則至遲 14 個工作日恢復；90-10 善意移除免責；90-11 不實通知賠償；辦法第 3 條六項要素、多數著作可同一通知；第 4 條補正 5＋5 個工作日，逾期「視為未提出通知」且「不得作為網路服務提供者知悉侵權情事之依據」；第 5、6 條回復通知。商標法沒有 ISP 條文。
- 個資法（VERIFIED）：第 8 條蒐集時告知利用對象；第 20 條特定目的外利用限於法律明文、公共利益、當事人同意等；INFERENCE：表單告知寫明「可能轉介主管機關」，檢舉人身分以明示同意為依據才交給機關。

## 3. GPSR 與食品（VERIFIED）
- GPSR 2(2)(b)：不適用食品。22(1)(2) 機關與消費者單一窗口；22(4) 機關命令 2 個工作日；22(8) 產品安全通知 3 個工作日；22(11) 先警告再停權經常違規的賣家；22(12) 直接通知已購買者、通知賣家下架決定、回應供應鏈資料請求。Art 35–37 召回：直接通知、不得用「voluntary」「precautionary」淡化、至少兩種救濟。Pledge+ 排除食品，承諾 2／3／5 個工作日。Amazon 召回：個人化 email 與「Your Recalls and Product Safety Alerts」頁。
- GFL 178/2002 第 19 條：零售與通路「passing on relevant information necessary to trace a food, cooperating in the action taken by producers」；必要時「recall from consumers」。

## 4. 案件的狀態實務（VERIFIED，除另註）
- Zendesk：Pending =「waiting for more information from the requester」；On-hold =「waiting for information or action from someone other than the requester」，提出者看到的是 Open；提出者回覆時 Pending 自動回到 Open；Solved 預設 4 天後 Closed；Requester wait time 在 Pending 暫停；merge「permanent and can't be undone」，被併入的工單被關閉。
- Jira Service Management：Waiting for Triage／Support、Waiting for Customer（顧客看到「Requester Action Needed」）、Resolved；SLA 在 Waiting for customer 暫停、Resolution set 停止。
- ServiceNow CSM：New → Open → Awaiting Info（只能由 agent 的 Request Info 進入）→ Resolved → Closed；顧客更新時回到 Open；Closed 不能再更新。Incident 的 On Hold reason：Awaiting Caller、Change、Problem、Vendor（SECONDARY）。
- ROOST Coop：「Jobs are created when a Report is routed to a Queue」；決策 Ignore、Move、自訂 Action；申訴 Uphold／Overturn；`QueueOperations.ts`：「Enqueues a new job …, or does nothing if a job already exists in that queue for the same item」，job 帶 reportHistory；「Invalidate reports」移除濫報者的紀錄；「Two moderators will never receive the same job」；沒有 investigating 狀態。
- Hive：被處置者與檢舉者都能申訴；狀態 in review、overturned、upheld。
- DSA 透明資料庫 schema：decision_visibility／decision_monetary／decision_provision／decision_account 四者獨立、各有 end_date；decision_ground 分 ILLEGAL_CONTENT 與 INCOMPATIBLE_CONTENT；decision_facts 必填；source_type：SOURCE_ARTICLE_16、SOURCE_TRUSTED_FLAGGER、SOURCE_TYPE_OTHER_NOTIFICATION、SOURCE_VOLUNTARY；automated_detection／automated_decision；content_type 含 PRODUCT；category 含 UNSAFE_AND_PROHIBITED_PRODUCTS；puid 是理由書 ID。
- 法定期限（可用於等待狀態）：著作權補正 5＋5 個工作日；起訴證明 10 個工作日；恢復至遲 14 個工作日；主管機關移送 7 日、回覆 30 日；詐欺調取 3 日、保存 6 個月；食安草案 2 個工作日、保存 180 日、業者通報 24 小時；食藥署請下架「通常2天內」；GPSR 2／3 個工作日；Pledge+ 2／3／5 個工作日；DSA 申訴至少 6 個月；P2B 終止提前 30 日；DSA 16「without undue delay」。
- INFERENCE：狀態分的是「在等誰」，SLA 是否暫停取決於此；T&S 以目標為單位開 case，每筆通知保留自己的回覆線（DSA 16(5)）；Zendesk 的 merge 會關閉來源，不適合多人檢舉同一目標；對外顯示的狀態可比內部少。

## 5. 調查期間要求賣家配合
- 法規範式（VERIFIED）：DSA 30(3) 限期補正否則停權；P2B 4(3) 給賣家澄清事實的機會；GPSR 22(12)(b)(e)(h)；食安法第 9 條（賣家保存來源文件）、第 41 條（機關直接向食品業者要求）。平台對賣家只能靠契約（INFERENCE）。
- Amazon（SECONDARY）：Account Health 的「Manage your compliance」與「Product Compliance Requests」可上傳文件，期限顯示在 Account Health 頁；恢復要交 Plan of Action；BSA cure notice 7 日、無故終止提前 30 日、停權時寫原因與申訴方式。
- 蝦皮（SECONDARY）：違規商品「暫時下架」，收到通知後「2 個工作天內」申訴並附佐證，逾期刪除並計分；計分申訴 14 天、10 個工作天內審核。官方 PDF 把未經核准的食品列為禁售（VERIFIED）。
- eBay（VERIFIED）：移除通知含權利人的姓名與聯絡方式；賣家可請權利人撤回；累犯從限制到永久停權。
- momo、PChome：沒有公開文件。

## 6. 回饋通知人與匿名
- DSA 16(4)(5)、20(1)、前言 54（VERIFIED）。台灣主管機關 30 日內回覆檢舉人（VERIFIED）。
- Amazon Report a Violation：「a Submission History tab tracks every report's status in real time」，須登錄 Brand Registry；未登錄的用公開表單（VERIFIED）。eBay VeRO 沒寫是否告知結果。Etsy counter notice 時「Etsy emails both parties」（SECONDARY）。
- 匿名：DSA 允許但收不到確認與決定；行政程序法第 173 條得不處理；Yahoo 拍賣隱藏檢舉人帳號，但把檢舉類型與意見顯示在商品頁，並規定同一商品 48 小時內勿連續檢舉（VERIFIED）。

## 7. 權利人與第三方管道
- Amazon 公開 Report Infringement 表單與 Brand Registry（管道 VERIFIED）；eBay VeRO NOCI（VERIFIED）；Etsy IP 表單（SECONDARY）；蝦皮 Brand IP Portal（SECONDARY）；Yahoo 拍賣：商標與專利權人交「權利人聲明書」與「檢舉通知書」，刪除後把權利人姓名與聯絡方式提供給賣家、不提供賣家聯絡方式給權利人（VERIFIED）；露天 PUB 24 小時內下架、累犯停權（VERIFIED）。
- INFERENCE：表單涵蓋實施辦法第 3 條六項要素、一則可列多個商品；缺件進「待補正」5 個工作日，逾期關閉且不構成知悉；移除後轉送賣家；回復通知後「待起訴證明」10 個工作日，期滿未起訴 14 個工作日內恢復；仿冒與商標沒有法定程序，證據門檻自訂。

## 8. 商品層級與帳號層級
- 歐盟（VERIFIED）：DSA 17(1)(a) 與 (c)(d) 是不同措施，資料庫各用獨立欄位、各有結束日；P2B 個別商品的限制在生效前或生效時給理由，全面終止提前 30 日；GPSR 命令針對特定商品並可擴及同款；22(11) 與 DSA 23(1) 先警告再停權。
- 台灣：食藥署要求的是商品下架，平台通常 2 天內完成（VERIFIED）；草案第 43-1 條只規範商品或內容（VERIFIED 草案）；帳號層級：詐欺條例第 36 條、著作權法第 90-4 條三振、Yahoo 與露天累犯停權（VERIFIED）；蝦皮先暫時下架再累計點數（SECONDARY）。
- 食品下架之後（VERIFIED）：DSA 32 通知購買者（回溯 6 個月）；GFL 19；食安法第 7 條第 5 項、第 52 條。

## 9. 研究者給的設計含意（全部 INFERENCE）
1. 通知與案件分開：通知面向通知人（收件確認、決定通知、申訴窗口、補正期限）；案件面向目標，同一商品或店家只開一個進行中的案件，聚合多筆通知；不用會關閉來源的 merge。
2. 決定紀錄照理由書 schema：商品可見度、店家或帳號、金流各為獨立維度，各有起訖；依據分違法（附條號）與違反條款；記來源類型與自動化。
3. 狀態只表達「在等誰」與期限：待補正、待賣家回應、待主管機關回覆（30 日），不設「調查中」；SLA 依等待的對象暫停。
4. 主管機關通道獨立：單一窗口、來函 2 個工作日內處理、收到通知時啟動 180 日證據保存；機關命令與一般通知分開記，仍要告知被處置者。
5. 轉介時以平台名義當檢舉人，檢舉人身分只在明示同意下提供；表單告知寫明可能轉介。
6. 允許匿名食安檢舉，但讓檢舉人知道代價。
7. 不公開顯示檢舉內容（食安法第 46-1 條）；濫報先警告再暫停受理（DSA 23）。
8. 食品不套用 GPSR；召回以 GFL 19、食安法第 7 條第 5 項與第 52 條、DSA 32 為基準；若 hawkr 自己做倉儲、配送或販賣，平台本身就是食品業者，要對照產品範圍確認。
