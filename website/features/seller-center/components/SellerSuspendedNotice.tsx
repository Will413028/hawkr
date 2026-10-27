"use client";

/**
 * 限制中店家的公告頁 — full-page takeover shown inside the seller-center shell (nav stays
 * visible so the sections that remain open stay reachable via the sidebar too)。兩種情境共用：
 *   - 停權：生效中的每一筆措施一張卡（依據、理由、期間、申覆進度），連到該措施
 *     （/seller/measures/{id}，帳號層級的措施與申覆頁，停權期間仍開放）；申覆入口一律顯示（`canAppeal` 為真連到該措施的申覆頁，否則反灰並附伺服器原因）；RISK 措施一律有解除請求入口（不能用時反灰並附原因），按下連到該措施的解除請求頁。
 *   - 隨帳號刪除、等待未結訂單完成：只能處理訂單與售後。
 * 仍可使用的功能依 `GetMyStore.capabilities` 中啟用的能力列出。
 */

import {
	MeasureKind,
	MeasureState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { commonUnknown, sellerMeasureState } from "@hawkr/ui-text/keys";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { useSellerIdentity } from "@/components/shared/useSellerIdentity";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import AppealEntry from "@/features/seller/measures/AppealEntry";
import { measureNotes } from "@/features/seller/measures/MeasureCard";
import {
	MEASURES_PATH,
	measureHref,
	measureMeta,
	measureStatusChips,
	measureTitle,
} from "@/features/seller/measures/measureDisplay";
import RemediationEntry, {
	MEASURE_ACTION_CLASS,
} from "@/features/seller/measures/RemediationEntry";
import { useSellerMeasures } from "@/lib/hooks/useSellerMeasures";
import { useUiText } from "@/lib/i18n/ui-text";
import type { SellerMeasure } from "@/lib/seller/measures";
import { activeCapabilities } from "@/lib/seller/store";
import { isPageAvailable } from "./pageCapabilities";

const SUSPENSION_NOTICE = "帳號已被平台停權";
/** 措施還沒讀到（讀取中或失敗）時的中性狀態文字：還不知道是停權還是終止，不能說停權。 */
const UNRESOLVED_STATUS = "商店已暫停營業";
const LOADING_TEXT = "載入中…";

/** 終止公告頁的疑義入口（文字與 seller-app 的 `suspension_notice_screen.dart` 相同）。 */
const TERMINATION_SUPPORT_HEADING = "對終止決定有疑義？";
const SUPPORT_BODY =
	"到措施與申覆查看措施內容與期間，並可對仍可申覆的措施提出申覆。";
const SUPPORT_ACTION = "措施與申覆";
/** 終止公告頁列出生效中終止措施的區段標題。 */
export const TERMINATION_REASON_HEADING = "終止原因";

/** 終止合作生效後的公告頁副標（標題「合作已終止」取自共用文案目錄的 MeasureState.ACTIVE）。 */
export const TERMINATION_SUBTITLE =
	"店家已暫停營業，已成立的訂單仍需完成；訂單完成後店家會關閉。終止生效後 6 個月內申覆成立，可恢復同一家店。";

/** 等待刪除的提示文字。 */
export const PENDING_DELETION_NOTICE =
	"帳號刪除處理中，等待未結訂單完成；期間只能處理訂單與售後";

const WHITELIST_LINKS = [
	{
		href: "/seller-center/me",
		title: "商店設定",
		description: "檢視商店資料；需要時可以在這裡關閉商店。",
	},
	{
		href: "/seller-center/orders",
		title: "訂單管理",
		description: "既有訂單的出貨、狀態更新與售後處理，仍可正常使用。",
	},
	{
		href: "/seller-center/notifications",
		title: "通知",
		description: "查看訂單與平台通知，掌握必要動態。",
	},
	// 稅務身分＝補齊或重新送出稅務件，正是解除停權的路徑；買家申訴＝逾期未回覆會
	// 再累積違規，鎖住等於讓停權自我延長。
	{
		href: "/seller-center/invoices/tax-status",
		title: "稅務身分",
		description: "查看稅務身分的審核進度與補件備註，仍可辦理。",
	},
	{
		href: "/seller-center/invoices/appeals",
		title: "買家發票申訴",
		description: "回覆買家提出的未收到發票申訴，逾期未回覆會再累積違規。",
	},
] as const;

export default function SellerSuspendedNotice() {
	const { isPendingDeletion, isSuspended } = useSellerIdentity();
	// 等待刪除不讀措施：只有停權（含終止合作）才有生效中的措施。
	return isPendingDeletion && !isSuspended ? (
		<NoticeFrame pendingOnly terminated={false} measures={null} />
	) : (
		<SuspendedNotice />
	);
}

/** 停權或終止合作：讀生效中的措施，有生效中的終止合作時公告頁用終止的標題與副標。 */
function SuspendedNotice() {
	const measures = useSellerMeasures(MeasureState.ACTIVE);
	const terminated = measures.records.some(
		(m) => m.kind === MeasureKind.TERMINATION,
	);
	return (
		<NoticeFrame
			pendingOnly={false}
			terminated={terminated}
			measures={measures}
		/>
	);
}

interface ActiveMeasuresState {
	records: SellerMeasure[];
	loading: boolean;
	error: string | null;
}

function NoticeFrame({
	pendingOnly,
	terminated,
	measures,
}: {
	pendingOnly: boolean;
	terminated: boolean;
	measures: ActiveMeasuresState | null;
}) {
	const { loading, seller } = useSellerIdentity();
	const ut = useUiText();
	const terminationTitle = ut(
		sellerMeasureState(MeasureState.ACTIVE, {
			kind: MeasureKind.TERMINATION,
		}) ?? commonUnknown(),
	);
	// 措施讀取中或失敗：還不知道有沒有終止，停權專屬文字都不顯示。
	const unresolved =
		!pendingOnly &&
		!terminated &&
		measures !== null &&
		(measures.loading || !!measures.error);
	const capabilities = activeCapabilities(seller);
	const links = WHITELIST_LINKS.filter((link) =>
		isPageAvailable(link.href, capabilities),
	);

	return (
		<SellerPageShell
			title={
				pendingOnly
					? "帳號刪除處理中"
					: terminated
						? terminationTitle
						: "商店已暫停營業"
			}
			subtitle={
				pendingOnly
					? undefined
					: unresolved
						? undefined
						: terminated
							? TERMINATION_SUBTITLE
							: "你的商家帳號目前處於停權狀態，商品已從市集下架、無法新增或變更上架商品。已成立的訂單仍可依下方白名單功能繼續處理。"
			}
		>
			<Card className="flex flex-col gap-4 p-6 sm:flex-row sm:items-center">
				<div
					className="flex size-16 shrink-0 items-center justify-center"
					aria-hidden="true"
				>
					<AppIcon name="danger" size={64} />
				</div>
				<div className="flex flex-col gap-1.5">
					<span className="inline-flex w-fit text-[0.8125rem] font-bold uppercase tracking-normal text-[var(--hkui-danger)]">
						帳號狀態
					</span>
					<strong className="text-lg text-[var(--hkui-foreground)]">
						{loading || (unresolved && measures?.loading)
							? LOADING_TEXT
							: pendingOnly
								? PENDING_DELETION_NOTICE
								: unresolved
									? UNRESOLVED_STATUS
									: terminated
										? terminationTitle
										: SUSPENSION_NOTICE}
					</strong>
				</div>
			</Card>

			{measures && (
				<ActiveMeasures
					{...measures}
					heading={terminated ? TERMINATION_REASON_HEADING : "生效中的措施"}
				/>
			)}

			<section className="flex flex-col gap-3">
				<h2 className="m-0 text-base font-extrabold text-[var(--hkui-foreground)]">
					{pendingOnly || terminated || unresolved
						? "目前仍可使用"
						: "停權期間仍可使用"}
				</h2>
				<div className="grid gap-3 sm:grid-cols-2">
					{links.map((link) => (
						<Card
							key={link.href}
							as={Link}
							href={link.href}
							isHoverable
							className="flex flex-col gap-1.5 p-5 no-underline"
						>
							<strong className="text-[0.9375rem] text-[var(--hkui-foreground)]">
								{link.title}
							</strong>
							<span className="text-[0.8125rem] leading-[1.6] text-[var(--hkui-foreground-soft)]">
								{link.description}
							</span>
						</Card>
					))}
				</div>
			</section>
			{terminated && <TerminationSupport />}
		</SellerPageShell>
	);
}

/** 終止公告頁的疑義入口：到措施與申覆（申覆期限與可否申覆由各措施卡呈現）。 */
function TerminationSupport() {
	return (
		<section className="flex flex-col gap-3">
			<h2 className="m-0 text-base font-extrabold text-[var(--hkui-foreground)]">
				{TERMINATION_SUPPORT_HEADING}
			</h2>
			<Card className="flex flex-col gap-3 p-5">
				<p className="m-0 text-sm leading-[1.6] text-[var(--hkui-foreground-soft)]">
					{SUPPORT_BODY}
				</p>
				<Link
					href={MEASURES_PATH}
					className="w-fit text-sm font-bold text-[var(--hkui-primary)]"
				>
					{SUPPORT_ACTION}
				</Link>
			</Card>
		</section>
	);
}

/**
 * 生效中的停權措施（`ListMyMeasures(state=ACTIVE)`）：每筆一張卡，連到該措施；能不能申覆讀伺服器的
 * `canAppeal`，與措施是管理員或違規認定建立的無關。
 */
function ActiveMeasures({
	records,
	loading,
	error,
	heading,
}: ActiveMeasuresState & { heading: string }) {
	return (
		<section className="flex flex-col gap-3">
			<div className="flex flex-wrap items-center justify-between gap-3">
				<h2 className="m-0 text-base font-extrabold text-[var(--hkui-foreground)]">
					{heading}
				</h2>
				<Link
					href={MEASURES_PATH}
					className="text-sm font-bold text-[var(--hkui-primary)]"
				>
					查看全部措施與申覆
				</Link>
			</div>
			{loading ? (
				<Card className="p-6 text-sm text-[var(--hkui-foreground-soft)]">
					載入中…
				</Card>
			) : error ? (
				<Card className="p-6 text-sm text-[var(--hkui-foreground-soft)]">
					{error}
				</Card>
			) : (
				records.map((measure) => (
					<ActiveMeasureCard key={measure.id} measure={measure} />
				))
			)}
		</section>
	);
}

function ActiveMeasureCard({ measure }: { measure: SellerMeasure }) {
	const ut = useUiText();
	const chips = measureStatusChips(ut, measure);
	const meta = measureMeta(ut, measure);
	return (
		<Card className="flex flex-col gap-3 p-5">
			<div className="flex flex-wrap items-start justify-between gap-3">
				<div className="flex flex-col gap-1">
					<strong className="text-[0.9375rem] text-[var(--hkui-foreground)]">
						{measureTitle(ut, measure)}
					</strong>
					{meta && (
						<span className="text-xs text-[var(--hkui-foreground-muted)]">
							{meta}
						</span>
					)}
				</div>
				{chips.length > 0 && (
					<div className="flex flex-wrap items-center gap-2">
						{chips.map((chip) => (
							<Chip
								key={chip.label}
								variant="flat"
								color={chip.color}
								size="sm"
							>
								{chip.label}
							</Chip>
						))}
					</div>
				)}
			</div>
			{measure.statement && (
				<p className="m-0 text-sm leading-[1.6] text-[var(--hkui-foreground-soft)]">
					{measure.statement}
				</p>
			)}
			{measureNotes(ut, measure).map((note) => (
				<p
					key={note}
					className="m-0 text-xs leading-[1.6] text-[var(--hkui-foreground-muted)]"
				>
					{note}
				</p>
			))}
			<div className="flex flex-wrap items-center justify-end gap-3">
				<Link
					href={measureHref(measure)}
					className="text-sm font-bold text-[var(--hkui-primary)]"
				>
					查看措施
				</Link>
				<RemediationEntry
					measure={measure}
					buttonClassName={MEASURE_ACTION_CLASS}
				/>
				<AppealEntry measure={measure} />
			</div>
		</Card>
	);
}
