"use client";

import "@/styles/features/delegated-orders.css";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { OrderStatus } from "@/lib/codemeta/enums";
import { buildImageUrl } from "@/lib/media/cdn";
import { useDelegatedSession } from "../state/DelegatedSessionContext";
import { delegatedAnomalyUrl } from "../state/session";
import {
	formatCancelNote,
	formatOrderAmount,
	formatOrderDateTime,
	isTerminalOrderStatus,
	orderStatusText,
	orderStatusTone,
	paymentMethodText,
	shippingMethodText,
} from "./display";
import RevealRecipientDialog from "./RevealRecipientDialog";
import type { DelegatedOrderDetail } from "./repo";
import {
	useDelegatedOrderDetail,
	useRevealRecipientContact,
} from "./useDelegatedOrders";

/**
 * 代行訂單詳情的內容（mockup §4.8.7）。抽屜與獨立詳情頁共用這一份。
 *
 * ⚠️ mockup 底部畫了「通知賣家」「建立平台異常紀錄」兩顆全寬按鈕，**只落地後者，
 * 且它是一條跨 host 連結而不是一個動作**。兩顆的處置理由不同，不要合併理解：
 *
 * - **平台異常處置（渲染）**：產品需求 §6.7 明寫「平台異常處置會**離開代行工作區並
 *   重新驗治理權限**」，56 號 §2.2 Out-of-scope #5 也把「異常訂單處置」歸給 29 號的
 *   既有 admin 入口。⇒ 它本來就不該在工作區內完成，正解是把人送到治理入口並把
 *   訂單帶過去。**沒有新端點**（29 號自陳「純查詢聚合層，不新增 DB 表、不新增業務
 *   動作」，那裡也沒有「建立」這個動作 —— 所以按鈕字面不照抄 mockup 的「建立」，
 *   見下一段）。
 * - **通知賣家（仍不渲染）**：56 號 §2.2 Out-of-scope **#15** 逐字列出「訂單詳情的
 *   『💬 通知賣家』CTA」本期不做 —— 訂單面唯讀，而 `notifySeller` 是依附於一筆
 *   mutation 的旗標（§8.4 W5 只在 `result='SUCCEEDED'` 時才建 outbox 列），唯讀路徑
 *   沒有可依附的 mutation；做它＝第 10 個 capability 的形狀。實查後端也確實沒有任何
 *   可複用路徑：notification 只有 `POST /admin/notifications/broadcast`（對象是
 *   buyer／seller／all 這個**使用者類型**，打不到單一賣家），IM 沒有任何 admin 入口。
 *
 * 不做成 `disabled` —— `disabled` 答得出「不能點」，答不出「為什麼」，而通知賣家的
 * 「為什麼」是「這個功能在代行下不存在」，那正是禁區語意，禁區的表達方式是不渲染。
 *
 * ⚠️ 收件資訊**預設遮罩**（遮罩在 order-service 做）。完整值只有按「查看完整地址」
 * 才取，那是一個 `risk_level=high` 的獨立稽核事實，且**不通知賣家**。取回來的明文
 * 只放在元件狀態裡，不進 react-query 快取。
 *
 * ⚠️ 呼叫端**必須**以 `key={orderNo}` 掛載本元件（抽屜與獨立頁都已如此）。換一筆
 * 訂單時整個元件重建，上一筆的明文隨狀態一起消失 —— 用 effect 去清會多一個「清得
 * 對不對」的問題，而清錯的後果是 A 單的完整地址顯示在 B 單底下。
 */

export default function DelegatedOrderDetailPanel({
	orderNo,
}: {
	orderNo: string;
}) {
	const { order, loading, error } = useDelegatedOrderDetail(orderNo);
	const { summary, admOrigin } = useDelegatedSession();
	const {
		reveal,
		revealing,
		error: revealError,
		contact,
	} = useRevealRecipientContact();
	const [dialogOpen, setDialogOpen] = React.useState(false);

	if (loading) return <p className="seller-empty">正在載入訂單詳情…</p>;
	if (error || !order) {
		return (
			<p className="seller-empty">
				訂單詳情載入失敗。可能是這筆訂單不屬於本次代行的店家，或工作階段已結束。
			</p>
		);
	}

	const revealed = contact?.recipient ?? null;
	const maskedAddress =
		[order.recipient.city, order.recipient.district].filter(Boolean).join("") ||
		"—";
	// 時間軸是「這筆訂單走到哪」的權威答案（後端保證最後一個節點恆為當下狀態），
	// 所以付款與否直接讀它，不另立第二個來源。
	const paid = order.timeline.some(
		(entry) => entry.status === OrderStatus.PAID,
	);
	const terminal = isTerminalOrderStatus(order.status);

	return (
		<div className="adm-order-detail">
			<div className="adm-order-detail__head">
				<span className="adm-order-detail__no">#{order.orderNo}</span>
				<span
					className={`seller-status seller-status--${orderStatusTone(order.status)}`}
				>
					{orderStatusText(order.status, order.statusLabel)}
				</span>
			</div>

			<Timeline order={order} />

			<section className="adm-order-detail__section">
				<div className="adm-order-detail__section-head">
					<h3 className="adm-order-detail__section-title">收件人與配送資訊</h3>
					<button
						type="button"
						className="adm-order-detail__reveal"
						onClick={() => setDialogOpen(true)}
						disabled={revealing || revealed !== null}
					>
						{revealed ? "已揭露完整地址" : "查看完整地址"}
					</button>
				</div>
				<div className="adm-order-detail__lines">
					<span>
						{revealed?.name || order.recipient.name || "—"}
						{"　"}
						{revealed?.phone || order.recipient.maskedPhone || ""}
					</span>
					<span>
						{revealed
							? [revealed.zipCode, revealed.fullAddress]
									.filter(Boolean)
									.join(" ")
							: maskedAddress}
					</span>
					{shippingMethodText(order.shippingMethod) ? (
						<span>{shippingMethodText(order.shippingMethod)}</span>
					) : null}
					{order.shippingStatusLabel ? (
						<span>物流狀態：{order.shippingStatusLabel}</span>
					) : null}
					{order.shippingNote ? (
						<span>配送備註：{order.shippingNote}</span>
					) : null}
					{contact ? (
						// 稽核列 id 是這次揭露唯一可追溯的憑據，讓操作者當場看得到。
						<span className="adm-order-timeline__time">
							已寫入代行紀錄（稽核編號 {contact.auditLogId}）
						</span>
					) : null}
				</div>
			</section>

			<section className="adm-order-detail__section">
				<h3 className="adm-order-detail__section-title">
					商品明細（共 {order.itemCount || order.items.length} 件）
				</h3>
				<ul className="adm-order-items">
					{order.items.map((item) => {
						const src = buildImageUrl(item.imageKey, "thumb");
						return (
							<li className="adm-order-items__item" key={item.skuId}>
								{src ? (
									// biome-ignore lint/performance/noImgElement: 動態 CDN URL，next/image 的網域未設定
									<img
										className="adm-order-items__thumb"
										src={src}
										alt=""
										loading="lazy"
										// 取不到的圖留一個空位，不留瀏覽器的破圖圖示 —— 與
										// `SellerAvatarImage` 同一條既有慣例：賣家面的介面永遠不替
										// 後端選定的圖片補一張佔位圖。
										onError={(event) => {
											event.currentTarget.style.visibility = "hidden";
										}}
									/>
								) : null}
								<span className="adm-order-items__body">
									<span className="adm-order-items__name">
										{item.productName}
									</span>
									<span className="adm-order-items__meta">
										{item.skuName ? `${item.skuName}　` : ""}
										{formatOrderAmount(item.unitPrice)} × {item.quantity}
									</span>
								</span>
							</li>
						);
					})}
				</ul>
			</section>

			<section className="adm-order-detail__section">
				<h3 className="adm-order-detail__section-title">金額與付款</h3>
				<div className="adm-order-detail__row">
					<span>付款狀態</span>
					{/* 由時間軸推導而非另開一個欄位：後端的時間軸已經是「這筆訂單走到哪」的
					    權威答案，再送一個 paymentStatus 就多出一個會與它不一致的來源。 */}
					<strong>{paid ? "已付款" : "未付款"}</strong>
				</div>
				{paymentMethodText(order.paymentMethod) ? (
					<div className="adm-order-detail__row">
						<span>付款方式</span>
						<strong>{paymentMethodText(order.paymentMethod)}</strong>
					</div>
				) : null}
				{order.itemsSubtotal ? (
					<div className="adm-order-detail__row">
						<span>商品小計</span>
						<strong>{formatOrderAmount(order.itemsSubtotal)}</strong>
					</div>
				) : null}
				<div className="adm-order-detail__row">
					<span>運費</span>
					<strong>{formatOrderAmount(order.shippingFee)}</strong>
				</div>
				<div className="adm-order-detail__row">
					<span>折扣</span>
					<strong>{formatOrderAmount(order.discountAmount)}</strong>
				</div>
				<div className="adm-order-detail__row">
					<span>訂單金額</span>
					<strong>{formatOrderAmount(order.totalAmount)}</strong>
				</div>
				{order.orderRemark ? (
					<div className="adm-order-detail__lines">
						<span>訂單備註：{order.orderRemark}</span>
					</div>
				) : null}
			</section>

			{/* 這三行有機械保證撐著（層 2 路由禁字 boot 斷言 ＋ 層 3a IsSeller(ADMIN)
			    為 false ＋ 層 3b proto 上根本沒有那些欄位），所以可以照 mockup 照抄。

			    ⚠️ 但**只對還走得下去的訂單照抄**。對一筆終態訂單說「出貨、取號、叫件…
			    僅能由賣家帳號執行」是在宣告一個這裡永遠不會有的功能——而且那句話對賣家
			    本人也不成立（已取消的訂單沒有人出得了貨），正是 §8.1.1「不要在畫面上宣告
			    這裡有出貨功能只是你沒權限」要防的同一件事。終態改說它自己的事實。 */}
			<section className="adm-order-lock">
				{/* 終態不用鎖也不用綠勾：鎖是「你沒權限」、綠勾是「成功了」，兩個都答錯了
				    問題。已取消與已完成共用同一張卡，只有中性的說明圖示兩邊都成立。 */}
				<AppIcon name={terminal ? "info" : "lock"} size={22} />
				{terminal ? (
					<>
						<span className="adm-order-lock__title">
							此訂單已結案，沒有待處理的履約操作
						</span>
						<span className="adm-order-lock__body">
							狀態為「{orderStatusText(order.status, order.statusLabel)}
							」，不會再有出貨或取件流程
						</span>
					</>
				) : (
					<>
						<span className="adm-order-lock__title">履約操作已鎖定</span>
						<span className="adm-order-lock__body">
							出貨、取號、叫件與確認寄出僅能由賣家帳號執行
							<br />
							即使直接呼叫 API 也會拒絕
						</span>
					</>
				)}
			</section>

			{/* 產品需求 §6.7 的第二顆「安全動作」。三件事刻意都不是 mockup 的字面：
			    ① **不叫「建立」** —— 目的地是 29 號的異常監控，它自陳「純查詢聚合層，
			       不新增 DB 表、不新增業務動作」，那裡沒有任何「建立異常紀錄」的動作
			       可按。照抄「建立」＝宣告一個不存在的功能，與同頁「查看權限說明」外連
			       因目的地不存在而不渲染是同一條判準。
			    ② **是 `<a href>` 不是按鈕** —— §6.7 明寫它會「離開代行工作區並重新驗
			       治理權限」。沿用 `AdmRail` 兩個入口的既有形狀：純連結、跨 host、
			       **不產生任何 session 狀態變更**（不 end、不 revoke、不動 revision）。
			    ③ **拿不到 storeId 就整顆不渲染** —— 少了 store_id 的連結會把人丟到全
			       平台的異常清單，那與沒有這顆鈕一樣，只是多騙一次點擊。 */}
			{summary?.target.storeId ? (
				<a
					className="adm-order-handoff"
					href={delegatedAnomalyUrl(admOrigin, {
						storeId: summary.target.storeId,
						orderNo: order.orderNo,
						status: order.status,
					})}
				>
					<span className="adm-order-handoff__label">
						<AppIcon name="danger" size={18} aria-hidden="true" />
						前往平台異常處置
					</span>
					<span className="adm-order-handoff__note">
						將離開代行工作區，並在 ADM 重新驗證治理權限
					</span>
				</a>
			) : null}

			<RevealRecipientDialog
				open={dialogOpen}
				orderNo={order.orderNo}
				submitting={revealing}
				errorMessage={revealError ? revealError.message : null}
				onClose={() => setDialogOpen(false)}
				onSubmit={(operationReason) => {
					reveal({ orderNo: order.orderNo, operationReason });
					setDialogOpen(false);
				}}
			/>
		</div>
	);
}

/**
 * 狀態時間軸。
 *
 * ⚠️ mockup 畫了四個節點、其中兩個是「尚未發生」的空心點。後端**只送真的發生過的
 * 節點**，前端不得為了湊出四個而補上未發生的節點 —— 那會渲染成一個這筆訂單從來不曾
 * 處於的狀態，而且沒有任何資料可以說它「將會」發生。
 *
 * ⚠️ **最後一個節點恆為訂單當下的狀態**（後端 `buildOrderTimeline` 的不變式），所以
 * 這裡把它標成「目前」：時間軸與標題那顆 chip 是同一件事的兩種畫法，兩者說出不同的
 * 狀態就是這一頁最嚴重的缺陷（曾經一筆已取消的訂單在這裡顯示「配送中」）。
 *
 * ⚠️ 送達／完成／取消三個結案節點帶自己的時間欄；其餘沒有專屬時間欄的狀態（待出貨、
 * 物流異常、部分／全額退款…）後端送空字串，這裡就不渲染時間那一行 —— 不補 `updated_at`
 * 也不留一個空白的位置。
 *
 * ⚠️ 「沒有時間」與「還在載入」不會被誤讀成同一件事：載入中的時候整個面板被
 * `正在載入訂單詳情…` 取代，時間軸根本不會掛載，所以這裡的「少一行時間」只有一種
 * 意思 —— 這個狀態沒有權威時間戳。
 */
function Timeline({ order }: { order: DelegatedOrderDetail }) {
	if (order.timeline.length === 0) return null;
	const lastIndex = order.timeline.length - 1;
	const cancelNote =
		order.status === OrderStatus.CANCELLED
			? formatCancelNote(
					order.cancelByRole,
					order.cancelReason,
					order.cancelRemark,
				)
			: "";
	return (
		<section className="adm-order-detail__section">
			<h3 className="adm-order-detail__section-title">訂單時間軸</h3>
			<ol className="adm-order-timeline">
				{order.timeline.map((entry, index) => {
					const current = index === lastIndex;
					const time = formatOrderDateTime(entry.occurredAt);
					return (
						<li
							className={[
								"adm-order-timeline__item",
								current ? "adm-order-timeline__item--current" : "",
								current ? `is-${orderStatusTone(entry.status)}` : "",
							]
								.filter(Boolean)
								.join(" ")}
							key={entry.status}
							aria-current={current ? "step" : undefined}
						>
							<span className="adm-order-timeline__label">
								{orderStatusText(entry.status, entry.statusLabel)}
								{current ? (
									<span className="adm-order-timeline__now">目前狀態</span>
								) : null}
							</span>
							{time ? (
								<span className="adm-order-timeline__time">{time}</span>
							) : null}
							{current && cancelNote ? (
								<span className="adm-order-timeline__note">{cancelNote}</span>
							) : null}
						</li>
					);
				})}
			</ol>
		</section>
	);
}
