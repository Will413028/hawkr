export interface FaqItem {
	q: string;
	a: string;
	/** Optional highlighted reminder / limitation shown under the answer. */
	note?: string;
	/** Optional ordered, detailed operation steps shown under the answer. */
	steps?: string[];
	open?: boolean;
}

export interface FaqCategory {
	id: string;
	name: string;
	icon: string;
	/** Which audience this category belongs to (drives the 買家/賣家 switch). */
	audience: "buyer" | "seller";
	items: FaqItem[];
}

export const faqCategories: FaqCategory[] = [
	// ───────────────────────── 買家 BUYER ─────────────────────────
	{
		id: "buyer-account",
		name: "帳號與登入",
		icon: "user",
		audience: "buyer",
		items: [
			{
				q: "如何註冊帳號？",
				a: "您可以在App或網站上點擊「註冊」按鈕，填寫基本資料即可完成註冊。支援手機號碼與Email兩種方式。",
				steps: [
					"點擊右上角的「註冊」。",
					"選擇以手機號碼或Email註冊。",
					"輸入手機／Email並設定密碼。",
					"收取並輸入驗證碼完成驗證。",
					"註冊完成，即可開始選購。",
				],
			},
			{
				q: "如何登入？",
				a: "開啟App或網站，輸入您的帳號（手機號碼或Email）與密碼即可登入。也可使用LINE或Google快速登入。",
				steps: [
					"點「登入」。",
					"輸入手機號碼或Email與密碼。",
					"或選擇LINE／Google快速登入。",
					"點「登入」進入帳號。",
				],
			},
			{
				q: "忘記密碼怎麼辦？",
				a: "在登入頁點擊「忘記密碼」，輸入註冊的手機號碼或Email，依指示收取驗證碼即可重設密碼。",
				steps: [
					"在登入頁點「忘記密碼」。",
					"輸入註冊的手機號碼或Email。",
					"收取簡訊／Email驗證碼並輸入。",
					"設定新密碼後重新登入。",
				],
			},
			{
				q: "登入過期怎麼辦？",
				a: "為了保障帳號安全，登入狀態會在一段時間後自動過期。請重新輸入帳號密碼登入即可。",
			},
			{
				q: "如何登出？",
				a: "進入「我的帳號」頁面，點選最下方的「登出」按鈕即可安全登出。",
				steps: [
					"進入「我的帳號」。",
					"捲動到頁面最下方。",
					"點「登出」即可安全登出。",
				],
			},
		],
	},
	{
		id: "buyer-purchase",
		name: "購買與付款",
		icon: "order",
		audience: "buyer",
		items: [
			{
				q: "如何購買商品？",
				a: "在商品頁選擇規格與數量後點「加入購物車」，於購物車確認後即可前往結帳。",
				steps: [
					"瀏覽或搜尋想買的商品。",
					"在商品頁選擇規格與數量。",
					"點「加入購物車」。",
					"進入購物車確認商品與金額。",
					"點「前往結帳」並填寫收件資訊。",
					"選擇付款方式完成下單。",
				],
			},
			{
				q: "有哪些付款方式？",
				a: "目前支援信用卡線上付款與超商付款等方式，實際可用方式以結帳頁顯示為準。",
			},
			{
				q: "商品與運費發票由誰開立？",
				a: "消費者端的商品及運費，由賣家作為發票銷售人，依實際交易內容及金額開立全額發票給買家；發票所載銷售人名稱及統一編號應為賣家資料。統一另依其與賣家間的合作契約及結算資料開立相應發票給賣家。取消、退款或折讓時，消費者端發票由賣家辦理；如依上游契約及結算關係影響統一原開立的發票，則由統一另行辦理。",
			},
			{
				q: "如何使用優惠券？",
				a: "結帳時於「優惠券」欄位選擇已領取且符合條件的優惠券即可折抵。",
				steps: [
					"先到商品或活動頁領取優惠券。",
					"結帳時展開「優惠券」欄位。",
					"選擇符合條件的優惠券。",
					"確認折抵金額後完成付款。",
				],
			},
			{
				q: "可以修改或取消訂單嗎？",
				a: "訂單付款後若賣家尚未出貨，可在「我的訂單」嘗試取消；已出貨的訂單請改走退換貨流程。",
			},
			{
				q: "下單後在哪裡查看訂單？",
				a: "進入「我的帳號」>「我的訂單」即可查看所有訂單的狀態與明細。",
				steps: [
					"進入「我的帳號」。",
					"點「我的訂單」。",
					"選擇訂單即可查看狀態與明細。",
				],
			},
		],
	},
	{
		id: "buyer-delivery",
		name: "配送與到貨",
		icon: "store",
		audience: "buyer",
		items: [
			{
				q: "有哪些配送方式？",
				a: "依賣家設定提供黑貓宅配到府或7-11超商取貨。",
			},
			{
				q: "如何查詢物流進度？",
				a: "進入「我的訂單」開啟訂單詳情，即可查看最新配送狀態與物流單號。",
				steps: [
					"進入「我的訂單」。",
					"點選要查詢的訂單。",
					"在訂單詳情查看物流狀態與單號。",
				],
			},
			{
				q: "超商取貨要注意什麼？",
				a: "請於通知到店後的期限內（通常為7天）持取貨資訊到店取貨，逾期未取訂單可能會被退回。",
			},
		],
	},
	{
		id: "buyer-return",
		name: "退換貨與退款",
		icon: "question",
		audience: "buyer",
		items: [
			{
				q: "如何申請退換貨？",
				a: "在「我的訂單」找到該筆訂單，點「申請退換貨」並填寫原因與上傳照片後送出。",
				note: "僅能在「確認收貨前」提出售後（退換貨）申請；一旦確認收貨或訂單完成，將無法再申請。",
				steps: [
					"進入「我的訂單」並開啟訂單。",
					"點「申請退換貨」。",
					"選擇退貨原因並上傳商品照片。",
					"送出申請，等待賣家審核。",
				],
			},
			{
				q: "退款多久會收到？",
				a: "賣家確認收到退貨後會進行退款，款項依原付款方式退回，作業時間約數個工作天。",
			},
			{
				q: "生鮮商品可以退貨嗎？",
				a: "生鮮商品若有品質問題（如損壞、不新鮮），請於收貨後盡快拍照並申請，由賣家協助處理。",
			},
			{
				q: "申請被拒絕怎麼辦？",
				a: "若對賣家的處理結果有疑義，可於案件中提出申訴，由平台介入協助釐清。",
			},
		],
	},
	{
		id: "buyer-coupon",
		name: "優惠與收藏",
		icon: "coupon",
		audience: "buyer",
		items: [
			{
				q: "如何領取優惠券？",
				a: "於賣家頁、商品頁或活動頁點「領取」即可，領取後會存在「我的優惠券」。",
				steps: [
					"在賣家頁或活動頁找到優惠券。",
					"點「領取」。",
					"到「我的帳號」>「我的優惠券」查看。",
					"結帳時即可選用。",
				],
			},
			{
				q: "如何收藏商品？",
				a: "在商品頁點愛心圖示即可收藏，收藏的商品會出現在「我的收藏」方便日後回購。",
			},
			{
				q: "如何追蹤喜歡的賣家？",
				a: "進入賣家頁點「追蹤」，之後可在追蹤清單快速回訪，並接收新品與動態通知。",
			},
			{
				q: "商品補貨會通知我嗎？",
				a: "在已售完的商品頁點「補貨通知」，補貨後系統會發送通知提醒您。",
			},
		],
	},
	{
		id: "buyer-interaction",
		name: "評價與互動",
		icon: "chat",
		audience: "buyer",
		items: [
			{
				q: "如何聯絡賣家？",
				a: "在商品頁或賣家頁點「聊聊」即可與賣家即時對話，詢問商品或出貨問題。",
				steps: [
					"進入商品頁或賣家頁。",
					"點「聊聊」開啟聊天室。",
					"輸入問題並送出，等待賣家回覆。",
				],
			},
			{
				q: "如何給商品評價？",
				a: "訂單完成後，可在「我的訂單」對購買的商品給星等與留言評價。",
				steps: [
					"進入「我的訂單」找到已完成訂單。",
					"點「評價」。",
					"給星等並填寫心得。",
					"送出後即顯示在商品頁。",
				],
			},
			{
				q: "可以在貼文下留言嗎？",
				a: "可以。在賣家的動態貼文下方即可留言互動，賣家也會回覆您的留言。",
			},
		],
	},
	// ───────────────────────── 賣家 SELLER ─────────────────────────
	{
		id: "store",
		name: "店舖管理",
		icon: "store",
		audience: "seller",
		items: [
			{
				q: "如何更換店舖頭像？",
				a: "進入「店舖管理」>「店舖資訊」，點擊頭像區域即可上傳新的店舖頭像圖片。",
				steps: [
					"進入「店舖管理」>「店舖資訊」。",
					"點擊頭像區域。",
					"上傳新圖片並裁切。",
					"點「儲存商店資訊」即更新。",
				],
			},
			{
				q: "如何更換店舖封面圖片？",
				a: "在「店舖管理」>「店舖資訊」頁面，點擊封面圖片區域即可更換。建議尺寸為1600x400像素（4:1）。",
				steps: [
					"進入「店舖管理」>「店舖資訊」。",
					"點擊封面圖片區域。",
					"上傳建議1600x400像素（4:1）的圖片。",
					"儲存後即更新。",
				],
			},
			{
				q: "店舖頁面有哪些標籤？",
				a: "店舖頁面包含「商品」、「貼文」和「買家評價」三個標籤，方便顧客瀏覽您的商品與評價。",
			},
			{
				q: "如何回覆顧客留言？",
				a: "進入「店舖管理」>「留言管理」，可以查看並回覆顧客在您商品或貼文下的留言。",
				steps: [
					"進入「店舖管理」>「留言管理」。",
					"找到顧客的留言。",
					"輸入回覆內容並送出。",
				],
			},
			{
				q: "如何使用圖片編輯器？",
				a: "上傳商品或貼文圖片時，系統提供內建圖片編輯器，可進行裁剪、旋轉、調整亮度等基本操作。",
			},
		],
	},
	{
		id: "product",
		name: "商品上架",
		icon: "vegetable",
		audience: "seller",
		items: [
			{
				q: "如何上架新商品？",
				a: "進入「店舖管理」>「商品管理」>「新增商品」，填寫商品資訊、上傳圖片、設定價格與庫存即可上架。",
				steps: [
					"進入「店舖管理」>「商品管理」。",
					"點「新增商品」。",
					"填寫名稱、分類與產地。",
					"上傳商品圖片（第一張為封面）。",
					"設定規格、價格與庫存。",
					"點「儲存並上架」。",
				],
			},
			{
				q: "如何修改商品價格？",
				a: "在「商品管理」列表中找到要修改的商品，點擊「編輯」即可修改價格、庫存等資訊。",
				steps: [
					"進入「商品管理」列表。",
					"找到商品並點「編輯」。",
					"修改價格或庫存。",
					"儲存即生效。",
				],
			},
			{
				q: "如何下架商品？",
				a: "在「商品管理」中找到商品，點擊「下架」按鈕。下架後商品不會被刪除，可隨時重新上架。",
				steps: [
					"進入「商品管理」。",
					"找到要下架的商品。",
					"點「下架」。",
					"需要時可再次「上架」。",
				],
			},
			{
				q: "如何刪除商品？",
				a: "在「商品管理」中找到商品，點擊「更多」>「刪除」。請注意，刪除後無法恢復。",
				steps: [
					"進入「商品管理」。",
					"找到要刪除的商品（僅草稿／下架可刪）。",
					"點「刪除」並確認。",
				],
			},
			{
				q: "商品圖片建議規格？",
				a: "建議使用800x800像素以上的正方形圖片，背景乾淨明亮。最多可上傳9張圖片，第一張為封面圖。",
			},
		],
	},
	{
		id: "order",
		name: "訂單管理",
		icon: "order",
		audience: "seller",
		items: [
			{
				q: "如何查看訂單？",
				a: "進入「訂單管理」可查看所有訂單狀態，包含待處理、已出貨、已完成等。",
			},
			{
				q: "如何處理退貨申請？",
				a: "在「訂單管理」>「退貨申請」中查看顧客的退貨請求，審核後可選擇同意或拒絕。",
			},
			{
				q: "如何為訂單安排出貨？",
				a: "進入「訂單管理」開啟待出貨訂單，依配送方式點「預約出貨」（黑貓）或「預約取號」（7-11），完成後即可列印託運單並等待取件。",
				steps: [
					"進入「訂單管理」開啟待出貨訂單。",
					"黑貓：點「預約出貨」並選擇出貨日。",
					"7-11：點「預約取號」。",
					"下載並列印託運單PDF。",
					"等待物流取件或交貨。",
				],
			},
			{
				q: "託運單可以重複列印嗎？有期限嗎？",
				a: "可在訂單詳情點「下載寄件單PDF」重複列印；託運單自建立起4天內有效，逾期請重新取號。",
			},
			{
				q: "可以取消訂單嗎？",
				a: "僅「待出貨」狀態的訂單可由賣家取消。於訂單詳情點「取消訂單」並填寫原因，系統會自動退款給買家。",
				steps: [
					"開啟「待出貨」狀態的訂單詳情。",
					"點「取消訂單」。",
					"選擇原因並填寫備註。",
					"確認後系統自動退款給買家。",
				],
			},
		],
	},
	{
		id: "wallet",
		name: "錢包與收款",
		icon: "wallet",
		audience: "seller",
		items: [
			{
				q: "如何提領收款？",
				a: "進入「錢包」頁面，點擊「提領」按鈕，輸入金額與銀行帳戶資訊即可申請提領。",
				steps: [
					"進入「錢包」頁。",
					"點「提領」。",
					"輸入提領金額與銀行帳戶。",
					"送出申請。",
				],
			},
			{
				q: "收款多久入帳？",
				a: "訂單完成後，款項會在T+7個工作天內撥入您的錢包。提領至銀行帳戶約需1-3個工作天。",
			},
			{
				q: "為什麼還不能收款？",
				a: "需先完成商家認證，系統會自動代為向PayUni建立收款帳戶；待PayUni審核通過後即可正式收款。",
			},
			{
				q: "在哪裡查看交易明細？",
				a: "進入「錢包與認證」頁，往下捲動即可載入每筆成交訂單的金額與日期。",
			},
			{
				q: "合作方案與結算資料怎麼確認？",
				a: "申請通過後，平台會提供適用的合作文件與結算說明。雙方完成確認並簽約後，依生效契約及每期對帳資料辦理；如有疑問，請聯繫合作窗口。",
			},
		],
	},
	{
		id: "customer",
		name: "顧客管理",
		icon: "follow",
		audience: "seller",
		items: [
			{
				q: "如何查看訂閱者？",
				a: "在「顧客管理」>「訂閱者」中可查看關注您店舖的用戶列表。",
			},
			{
				q: "如何將客戶設為VIP或黑名單？",
				a: "進入「客戶管理」點選客戶開啟詳情，使用上方分類按鈕即可設定為一般、VIP或黑名單。",
				steps: [
					"進入「客戶管理」。",
					"點選客戶開啟詳情。",
					"使用上方分類按鈕選擇一般／VIP／黑名單。",
				],
			},
			{
				q: "可以為客戶貼標籤嗎？",
				a: "可以。在客戶詳情的標籤編輯區輸入標籤，方便分群管理與後續行銷。",
			},
			{
				q: "如何只發優惠券給特定客戶？",
				a: "在客戶詳情點「發優惠券」，系統會自動帶入該客戶為指定發放對象。",
			},
			{
				q: "在哪裡看客戶的購買紀錄？",
				a: "客戶詳情頁會顯示累計訂單、累計消費與歷史訂單，點訂單編號可直接查看訂單詳情。",
			},
		],
	},
	{
		id: "chat",
		name: "聊天功能",
		icon: "chat",
		audience: "seller",
		items: [
			{
				q: "如何開始與顧客聊天？",
				a: "當顧客發送訊息時，您會在「聊天」頁面收到通知。點擊即可開始對話。",
			},
			{
				q: "買家離線時訊息會送達嗎？",
				a: "會。即使對方離線，訊息仍會送出，待對方上線後即可收到。",
			},
			{
				q: "可以在聊天中傳送圖片嗎？",
				a: "可以。點輸入框旁的圖片圖示即可選圖傳送。",
			},
			{
				q: "聊天顯示連線異常怎麼辦？",
				a: "系統會自動嘗試重連，期間傳送的訊息仍會送達；若持續異常，請檢查網路或重新整理頁面。",
			},
		],
	},
	{
		id: "coupon",
		name: "優惠券",
		icon: "coupon",
		audience: "seller",
		items: [
			{
				q: "如何建立優惠券？",
				a: "進入「行銷工具」>「優惠券」>「新增」，設定折扣類型、金額、使用期限等條件。",
				steps: [
					"進入「行銷工具」>「優惠券」。",
					"點「新增」。",
					"選擇折扣類型並設定金額與門檻。",
					"設定發行對象與有效期間。",
					"點「建立並發行」。",
				],
			},
			{
				q: "可以建立哪些類型的優惠券？",
				a: "支援現金折抵與折扣百分比兩種；百分比券需另外設定單次折抵上限。",
			},
			{
				q: "優惠券發行後可以修改嗎？",
				a: "發行後內容即固定無法修改。如需調整，請先「收回」該張優惠券再重新建立。",
			},
			{
				q: "如何停用或收回優惠券？",
				a: "在「優惠券」列表點該券的「收回」並確認即可，買家將無法再領取或使用。",
			},
		],
	},
	{
		id: "notify",
		name: "通知設定",
		icon: "notify",
		audience: "seller",
		items: [
			{
				q: "如何管理通知？",
				a: "進入「設定」>「通知設定」，可自訂訂單通知、聊天通知、行銷活動等通知開關。",
			},
			{
				q: "哪些通知無法關閉？",
				a: "為確保不漏接出貨狀況，「配送異常通知」為賣家強制開啟、無法關閉；其餘類別皆可自由開關。",
			},
			{
				q: "買家與賣家的通知設定相同嗎？",
				a: "不同。通知設定會依目前身分（買家／賣家）分別套用，互不影響。",
			},
			{
				q: "如何標記通知為已讀？",
				a: "在「通知」頁點該則通知即可標為已讀，或點「全部標為已讀」一次清除未讀。",
			},
		],
	},
	{
		id: "troubleshoot",
		name: "常見問題排解",
		icon: "question",
		audience: "seller",
		items: [
			{
				q: "頁面無法載入怎麼辦？",
				a: "請嘗試重新整理頁面、清除瀏覽器快取，或使用不同的瀏覽器。若問題持續，請聯繫客服。",
			},
			{
				q: "圖片上傳失敗怎麼辦？",
				a: "請確認圖片格式與大小（建議800x800像素以上的JPG或PNG），網路穩定後再試；多次失敗請聯繫客服。",
			},
			{
				q: "商品無法上架是什麼原因？",
				a: "常見原因為必填欄位未完成，或未選到第三層分類。請依畫面提示補齊後再上架。",
			},
			{
				q: "登入後找不到賣家中心？",
				a: "請確認已建立賣場並完成手機驗證；完成後重新登入一次，賣家中心就會出現在帳號選單。",
			},
		],
	},
];

/**
 * Top-level nav items.
 * `panel` (optional) drives the Mastercard-style mega menu: a full-width
 * panel that slides down from the navbar on hover/focus, with grouped
 * columns of links + an optional rich feature card on the right.
 */
export interface NavMegaLink {
	label: string;
	href: string;
	desc?: string;
	badge?: string;
}

export interface NavMegaColumn {
	title: string;
	links: NavMegaLink[];
	/**
	 * When set, MegaMenu ignores the static `links` and renders the
	 * matching dynamic list passed in via its `dynamicLinks` prop.
	 * Lets the column's heading + position stay static while the
	 * items come from the backend (categories tree, trending tags).
	 */
	source?: "categories" | "hashtags";
}

export interface NavMegaFeature {
	eyebrow?: string;
	title: string;
	desc: string;
	href: string;
	cta: string;
	image?: string;
}

export interface NavItem {
	label: string;
	href: string;
	panel?: {
		columns: NavMegaColumn[];
		feature?: NavMegaFeature;
		/**
		 * When true, the megamenu renders a <BannerCarousel> in the right
		 * slot (in place of `feature`). Heading defaults to "最新消息".
		 * Banners come from ContentPublicService.ListLiveBanners — see `lib/banners/repo.ts`.
		 */
		banners?: boolean;
		/**
		 * When true, the right slot renders the week's most-viewed post
		 * (eyebrow + image + content snippet + view count). Fed by
		 * `MegaMenuDynamicLinks.topPost` — SSR-prefetched in layout via
		 * `fetchTopWeeklyPost`. Mutually exclusive with `banners` /
		 * `feature`.
		 */
		weeklyPost?: boolean;
	};
}

export const navItems: NavItem[] = [
	{
		label: "Hawkr 動態",
		href: "/feed",
		panel: {
			// 熱門 hashtag column is the only entry here — its links get
			// injected client-side by Navbar's `withTrendingHashtags` from
			// /public/tags/trending. The right slot renders the week's
			// most-viewed post (see `weeklyPost` below) so nothing on this
			// panel is hardcoded — all content is backed by real data.
			columns: [
				{
					title: "熱門 hashtag",
					links: [],
				},
			],
			weeklyPost: true,
		},
	},
	{
		label: "Hawkr 市集",
		href: "/market/products",
		panel: {
			// Both columns render dynamic data prefetched by the root layout
			// — categories from /public/categories/tree (first level),
			// hashtags from /public/tags/trending (sorted by post count
			// desc). Static `links` stay empty; MegaMenu fills them from
			// its `dynamicLinks` prop at render time. Empty source data =
			// column skipped entirely (no hardcoded fallback).
			columns: [
				{ title: "商品分類", links: [], source: "categories" },
				{ title: "熱門 hashtag", links: [], source: "hashtags" },
			],
			// Right slot renders a <BannerCarousel size="compact"> with
			// heading "最新消息" — replaces both the old 產地專區 column
			// and the 主題活動 feature card in one go.
			banners: true,
		},
	},
	{
		label: "搜尋",
		href: "/search",
	},
];
