"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import BrandLogo from "@/components/shared/BrandLogo";
import Link from "@/components/shared/Link";
import { Button } from "@/components/ui";
import styles from "@/styles/features/procurement-rfq-prototype/external-vendor-portal.module.css";
import {
	EXTERNAL_VENDOR_BUYER_OFFER,
	EXTERNAL_VENDOR_STEPS,
	EXTERNAL_VENDOR_TRANSACTION_STEPS,
	externalVendorStepPath,
	RFQ_DEMO_SUMMARY,
} from "./data";
import type {
	ExternalVendorStep,
	ExternalVendorTransactionStep,
} from "./types";

type TransactionScenario =
	| "negotiation"
	| "selected"
	| "fulfillment"
	| "settlement";

const TRANSACTION_SCENARIOS: ReadonlyArray<{
	id: TransactionScenario;
	label: string;
	status: string;
	tone: "info" | "warning" | "success" | "neutral";
}> = [
	{
		id: "negotiation",
		label: "異議處理",
		status: "價格異議待回覆",
		tone: "warning",
	},
	{
		id: "selected",
		label: "待付款",
		status: "供應商已接受",
		tone: "info",
	},
	{
		id: "fulfillment",
		label: "履約中",
		status: "已付款｜待出貨",
		tone: "success",
	},
	{
		id: "settlement",
		label: "待結算",
		status: "收貨完成｜待結算",
		tone: "neutral",
	},
];

function money(value: number): string {
	return `NT$${new Intl.NumberFormat("zh-TW", {
		maximumFractionDigits: 0,
	}).format(value)}`;
}

function PortalBadge({
	children,
	tone = "neutral",
}: {
	children: React.ReactNode;
	tone?: "neutral" | "success" | "warning" | "info";
}) {
	return (
		<span className={styles.badge} data-tone={tone}>
			{children}
		</span>
	);
}

function FrontendOnlyNotice() {
	return (
		<div className={styles.frontendNotice} role="note">
			<AppIcon name="info" size={18} />
			<span>
				<strong>前端流程預覽</strong>
				目前不會送出邀請、驗證、詢價回覆、訂單或付款資料。
			</span>
		</div>
	);
}

function PortalProgress({
	current,
}: {
	current: ExternalVendorTransactionStep;
}) {
	const currentIndex = EXTERNAL_VENDOR_TRANSACTION_STEPS.findIndex(
		(step) => step.slug === current,
	);

	return (
		<nav className={styles.progressNav} aria-label="外部供應商交易流程">
			<ol>
				{EXTERNAL_VENDOR_TRANSACTION_STEPS.map((step, index) => (
					<li
						key={step.slug}
						data-current={current === step.slug || undefined}
						data-complete={index < currentIndex || undefined}
					>
						<Link
							href={externalVendorStepPath(step.slug)}
							aria-current={current === step.slug ? "step" : undefined}
						>
							<span className={styles.stepMarker}>
								{index < currentIndex ? (
									<AppIcon name="check" size={17} />
								) : (
									index + 1
								)}
							</span>
							<span>
								<strong>{step.shortLabel}</strong>
								<small>{step.label}</small>
							</span>
						</Link>
					</li>
				))}
			</ol>
		</nav>
	);
}

function PortalCard({
	title,
	description,
	action,
	children,
	className,
}: {
	title: string;
	description?: string;
	action?: React.ReactNode;
	children: React.ReactNode;
	className?: string;
}) {
	return (
		<section className={`${styles.card} ${className ?? ""}`}>
			<header className={styles.cardHeader}>
				<div>
					<h2>{title}</h2>
					{description ? <p>{description}</p> : null}
				</div>
				{action}
			</header>
			{children}
		</section>
	);
}

function RequestFacts({ compact = false }: { compact?: boolean }) {
	return (
		<dl className={compact ? styles.compactFacts : styles.requestFacts}>
			<div>
				<dt>品項</dt>
				<dd>企業活動蔬果箱</dd>
			</div>
			<div>
				<dt>數量</dt>
				<dd>80 箱</dd>
			</div>
			<div>
				<dt>交貨</dt>
				<dd>2026/08/15 上午</dd>
			</div>
			<div>
				<dt>地點</dt>
				<dd>臺北市信義區</dd>
			</div>
			<div>
				<dt>回覆截止</dt>
				<dd>2026/08/12 18:00</dd>
			</div>
			<div>
				<dt>需求版本</dt>
				<dd>v3｜2026/08/10</dd>
			</div>
		</dl>
	);
}

function InvitationView() {
	return (
		<div className={styles.invitationLayout}>
			<section className={styles.invitationHero}>
				<div className={styles.eyebrow}>
					<PortalBadge tone="success">邀請連結有效</PortalBadge>
					<span>有效至 2026/08/12 18:00</span>
				</div>
				<h1>豐田產地直送，您收到一份採購詢價</h1>
				<p>
					青田設計透過 Hawkr
					邀請您提供正式報價。先確認需求，再完成交易資格；接受邀請不代表承諾供貨。
				</p>
				<div className={styles.invitationBuyer}>
					<span className={styles.buyerAvatar}>青</span>
					<div>
						<small>邀請方</small>
						<strong>範例設計股份有限公司</strong>
						<span>企業採購｜Email 與手機已驗證</span>
					</div>
				</div>
				<div className={styles.primaryActions}>
					<Button
						href={externalVendorStepPath("exchange")}
						size="lg"
						endContent={<AppIcon name="arrow-right" size={18} />}
					>
						確認邀請並繼續
					</Button>
					<Button variant="light" color="default" size="lg" disabled>
						婉拒邀請（API 待接）
					</Button>
				</div>
				<p className={styles.actionHint}>
					繼續只會帶您進入驗證頁；在送出正式報價前都不構成交易。
				</p>
			</section>

			<aside className={styles.invitationSummary}>
				<header>
					<span className={styles.documentMark}>
						<AppIcon name="document" size={23} />
					</span>
					<div>
						<small>{RFQ_DEMO_SUMMARY.rfqNo}</small>
						<strong>{RFQ_DEMO_SUMMARY.title}</strong>
					</div>
				</header>
				<RequestFacts compact />
				<div className={styles.trustNote}>
					<AppIcon name="shield" size={20} />
					<span>
						<strong>Hawkr 代收付款</strong>
						成交後由平台建立付款與訂單；買家不會直接取得您的完整收款資料。
					</span>
				</div>
			</aside>
		</div>
	);
}

function AccessCard({
	icon,
	eyebrow,
	title,
	description,
	children,
	aside,
}: {
	icon: React.ComponentProps<typeof AppIcon>["name"];
	eyebrow: string;
	title: string;
	description: string;
	children: React.ReactNode;
	aside?: React.ReactNode;
}) {
	return (
		<div className={styles.accessLayout}>
			<section className={styles.accessCard}>
				<span className={styles.accessIcon}>
					<AppIcon name={icon} size={26} />
				</span>
				<div className={styles.accessHeading}>
					<span>{eyebrow}</span>
					<h1>{title}</h1>
					<p>{description}</p>
				</div>
				{children}
			</section>
			{aside ? <aside className={styles.accessAside}>{aside}</aside> : null}
		</div>
	);
}

function ExchangeView() {
	return (
		<AccessCard
			icon="shield"
			eyebrow="步驟 1／2｜安全邀請"
			title="正在確認這份邀請"
			description="正式版會把網址中的一次性憑證換成受限工作階段，接著立即從網址移除憑證。"
			aside={
				<div className={styles.accessAsideBody}>
					<PortalBadge tone="success">邀請摘要已載入</PortalBadge>
					<h2>只有受邀者能繼續</h2>
					<p>即使連結被轉傳，正式報價、KYC 與收款資料仍需要受邀信箱驗證。</p>
					<dl className={styles.accessFacts}>
						<div>
							<dt>受邀信箱</dt>
							<dd>sa•••@fengtian.example</dd>
						</div>
						<div>
							<dt>邀請期限</dt>
							<dd>2026/08/12 18:00</dd>
						</div>
					</dl>
				</div>
			}
		>
			<ol className={styles.exchangeSteps}>
				<li data-complete>
					<span>
						<AppIcon name="check" size={17} />
					</span>
					<div>
						<strong>確認邀請摘要</strong>
						<small>只顯示買家、品項、截止時間與遮罩信箱。</small>
					</div>
				</li>
				<li data-current>
					<span>
						<AppIcon name="lock" size={17} />
					</span>
					<div>
						<strong>兌換一次性憑證</strong>
						<small>正式版由後端原子消耗；同一連結不能重複兌換。</small>
					</div>
				</li>
				<li>
					<span>3</span>
					<div>
						<strong>驗證受邀信箱</strong>
						<small>建立可回訪的外部供應商身分。</small>
					</div>
				</li>
			</ol>
			<div className={styles.accessActions}>
				<Button
					href={externalVendorStepPath("sign-in")}
					fullWidth
					endContent={<AppIcon name="arrow-right" size={18} />}
				>
					預覽兌換成功並繼續
				</Button>
				<p>前端預覽不會讀取、保存或消耗真實邀請憑證。</p>
			</div>
		</AccessCard>
	);
}

function SignInView() {
	return (
		<AccessCard
			icon="user"
			eyebrow="步驟 2／2｜身分驗證"
			title="使用受邀信箱繼續"
			description="驗證完成後，之後可從固定供應商入口回來，不必再尋找原始邀請連結。"
			aside={
				<div className={styles.securityPromise}>
					<AppIcon name="shield" size={22} />
					<div>
						<strong>為什麼還要驗證信箱？</strong>
						<p>
							邀請 URL 可能被轉傳；信箱驗證可把報價與公司資料綁定到指定窗口。
						</p>
					</div>
				</div>
			}
		>
			<div className={styles.authMethods}>
				<Button
					variant="bordered"
					color="default"
					fullWidth
					disabled
					startContent={<span className={styles.googleMark}>G</span>}
				>
					使用受邀 Google 帳號繼續（API 待接）
				</Button>
				<div className={styles.authDivider}>
					<span>或使用 Email 驗證碼</span>
				</div>
				<label className={styles.accessField}>
					<span>受邀 Email</span>
					<input
						type="email"
						name="email"
						autoComplete="email"
						spellCheck={false}
						value="sales@fengtian.example"
						readOnly
					/>
					<small>受邀信箱不能在 Portal 直接變更，需由 Hawkr 重新邀請。</small>
				</label>
				<Button
					href={externalVendorStepPath("verify-code")}
					fullWidth
					endContent={<AppIcon name="send" size={18} />}
				>
					預覽寄出 6 位數驗證碼
				</Button>
			</div>
			<p className={styles.authFootnote}>
				正式版只會回覆一致訊息，不會向未登入者揭露這個信箱是否存在。
			</p>
		</AccessCard>
	);
}

function VerifyCodeView() {
	const [code, setCode] = React.useState("");
	const [status, setStatus] = React.useState<"idle" | "error" | "success">(
		"idle",
	);
	const [resendCount, setResendCount] = React.useState(0);

	function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setStatus(code === "135790" ? "success" : "error");
	}

	return (
		<AccessCard
			icon={status === "success" ? "shield" : "message"}
			eyebrow="Email 驗證"
			title={status === "success" ? "信箱驗證完成" : "輸入 6 位數驗證碼"}
			description={
				status === "success"
					? "這個前端狀態代表驗證完成；正式版會旋轉 Session 並載入供應商工作台。"
					: "正式版會將驗證碼寄到 sa•••@fengtian.example。可直接貼上完整驗證碼。"
			}
			aside={
				<div className={styles.previewCode}>
					<span>前端測試碼</span>
					<strong>135790</strong>
					<p>輸入其他 6 位數可預覽錯誤狀態；此碼不會傳送到任何後端。</p>
				</div>
			}
		>
			{status === "success" ? (
				<div className={styles.verifiedState} role="status">
					<span>
						<AppIcon name="check" size={22} />
					</span>
					<div>
						<strong>已綁定受邀信箱</strong>
						<p>
							一般瀏覽與草稿可在這個 Session 中繼續；變更收款帳戶仍需再次驗證。
						</p>
					</div>
					<Button
						href={externalVendorStepPath("portal")}
						endContent={<AppIcon name="arrow-right" size={18} />}
					>
						進入供應商工作台
					</Button>
				</div>
			) : (
				<form className={styles.codeForm} onSubmit={handleSubmit} noValidate>
					<label className={styles.accessField}>
						<span>驗證碼</span>
						<input
							className={styles.codeInput}
							name="one-time-code"
							type="text"
							inputMode="numeric"
							autoComplete="one-time-code"
							spellCheck={false}
							maxLength={6}
							placeholder="000000"
							value={code}
							aria-invalid={status === "error" || undefined}
							aria-describedby={
								status === "error" ? "vendor-code-error" : "vendor-code-hint"
							}
							onChange={(event) => {
								setCode(event.target.value.replace(/\D/g, "").slice(0, 6));
								if (status === "error") setStatus("idle");
							}}
						/>
						{status === "error" ? (
							<small
								id="vendor-code-error"
								className={styles.fieldError}
								role="alert"
							>
								驗證碼不正確或已失效，請重新確認或取得新的驗證碼。
							</small>
						) : (
							<small id="vendor-code-hint">
								驗證碼預計 10 分鐘內有效，使用後立即失效。
							</small>
						)}
					</label>
					<Button type="submit" fullWidth disabled={code.length !== 6}>
						確認驗證碼
					</Button>
					<button
						type="button"
						className={styles.textAction}
						onClick={() => {
							setCode("");
							setStatus("idle");
							setResendCount((current) => current + 1);
						}}
					>
						重新取得前端示意碼
					</button>
					<p className={styles.resendStatus} aria-live="polite">
						{resendCount > 0
							? "已重設示意碼輸入；正式版才會寄送 Email。"
							: "沒有收到？60 秒後可要求重新寄送。"}
					</p>
				</form>
			)}
		</AccessCard>
	);
}

function SessionExpiredView() {
	return (
		<AccessCard
			icon="clock"
			eyebrow="需要重新驗證"
			title="這次工作階段已結束"
			description="為保護報價與公司資料，Portal 在閒置 24 小時或登入滿 7 天後會要求重新驗證。"
		>
			<div className={styles.recoveryPanel}>
				<div>
					<AppIcon name="document" size={20} />
					<span>
						<strong>不必重新取得邀請連結</strong>
						<small>用原本受邀信箱登入，即可找回仍有權限的詢價與交易。</small>
					</span>
				</div>
				<Button
					href={externalVendorStepPath("sign-in")}
					fullWidth
					endContent={<AppIcon name="arrow-right" size={18} />}
				>
					重新登入
				</Button>
				<Button
					href={externalVendorStepPath("portal")}
					variant="light"
					color="default"
					fullWidth
				>
					預覽工作台
				</Button>
			</div>
			<p className={styles.authFootnote}>
				正式版的草稿恢復狀態必須由後端回傳；本頁不宣稱已保存資料。
			</p>
		</AccessCard>
	);
}

type InviteUnavailableReason = "used" | "expired" | "revoked";

const INVITE_UNAVAILABLE_COPY: Record<
	InviteUnavailableReason,
	{ label: string; title: string; description: string; action: string }
> = {
	used: {
		label: "已使用",
		title: "這個邀請連結已經使用過",
		description:
			"一次性連結不能重複兌換。若你已建立供應商身分，請使用受邀信箱回到工作台。",
		action: "使用受邀信箱登入",
	},
	expired: {
		label: "已過期",
		title: "這份邀請已超過可回覆期限",
		description:
			"為避免看到已失效的需求，完整詢價與附件不再開放。請聯絡邀請方確認是否重新發送。",
		action: "重新邀請（API 待接）",
	},
	revoked: {
		label: "已撤回",
		title: "邀請方已撤回這份邀請",
		description:
			"目前無法進入詢價或提交報價。若你認為狀態不正確，可聯絡 Hawkr 交易支援。",
		action: "聯絡支援（API 待接）",
	},
};

function InviteUnavailableView() {
	const [reason, setReason] = React.useState<InviteUnavailableReason>("used");
	const copy = INVITE_UNAVAILABLE_COPY[reason];

	return (
		<AccessCard
			icon="shield-fail"
			eyebrow="邀請連結狀態"
			title={copy.title}
			description={copy.description}
			aside={
				<div className={styles.accessAsideBody}>
					<PortalBadge tone={reason === "used" ? "info" : "warning"}>
						{copy.label}
					</PortalBadge>
					<h2>我們不會顯示連結是否對應特定公司</h2>
					<p>
						正式 API
						對不存在、過期與撤回採相同外部錯誤，不讓未登入者枚舉邀請資料。
					</p>
				</div>
			}
		>
			<fieldset className={styles.stateTabs} aria-label="預覽邀請失效狀態">
				{(
					Object.keys(INVITE_UNAVAILABLE_COPY) as InviteUnavailableReason[]
				).map((item) => (
					<button
						key={item}
						type="button"
						aria-pressed={reason === item}
						onClick={() => setReason(item)}
					>
						{INVITE_UNAVAILABLE_COPY[item].label}
					</button>
				))}
			</fieldset>
			<div className={styles.recoveryPanel}>
				{reason === "used" ? (
					<Button href={externalVendorStepPath("sign-in")} fullWidth>
						{copy.action}
					</Button>
				) : (
					<Button fullWidth disabled>
						{copy.action}
					</Button>
				)}
				<Link
					href={externalVendorStepPath("invitation")}
					className={styles.textLink}
				>
					返回邀請摘要
				</Link>
			</div>
		</AccessCard>
	);
}

function PortalHomeView() {
	const entries = [
		{
			status: "待確認",
			tone: "warning" as const,
			title: "企業活動蔬果箱｜買家已確認條件",
			meta: "範例設計股份有限公司｜8/12 18:00 前回覆",
			detail: "採購總額 NT$22,000｜Pricebook 試算 v4",
			href: externalVendorStepPath("quote"),
			action: "確認條件",
		},
		{
			status: "待補件",
			tone: "info" as const,
			title: "企業與收款資格",
			meta: "負責人身分與收款帳戶尚未送審",
			detail: "可先確認詢價條件；付款前必須完成資格驗證",
			href: externalVendorStepPath("business-verification"),
			action: "繼續驗證",
		},
		{
			status: "履約中",
			tone: "success" as const,
			title: "訂單 HK202608150014",
			meta: "已付款｜最晚 8/14 12:00 出貨",
			detail: "發票尚未上傳｜結算尚未開放",
			href: externalVendorStepPath("transaction"),
			action: "追蹤交易",
		},
	];

	return (
		<div className={styles.portalHome}>
			<section className={styles.portalWelcome}>
				<div>
					<span>供應商工作台</span>
					<h1>早安，豐田產地直送</h1>
					<p>
						從固定入口查看所有仍有權限的詢價、報價與交易，不必保留原始邀請連結。
					</p>
				</div>
				<div className={styles.sessionSummary}>
					<AppIcon name="shield" size={22} />
					<span>
						<strong>受邀信箱已驗證</strong>
						<small>sa•••@fengtian.example｜本裝置最長 7 天</small>
					</span>
				</div>
			</section>

			<dl className={styles.portalStats}>
				<div>
					<dt>待確認詢價單</dt>
					<dd>1</dd>
					<span>2 天內截止</span>
				</div>
				<div>
					<dt>已接受條件</dt>
					<dd>0</dd>
					<span>本期尚無</span>
				</div>
				<div>
					<dt>進行中交易</dt>
					<dd>1</dd>
					<span>等待履約資料</span>
				</div>
				<div>
					<dt>待補資格</dt>
					<dd>2</dd>
					<span>影響成交與收款</span>
				</div>
			</dl>

			<div className={styles.portalHomeGrid}>
				<section className={styles.portalInbox}>
					<header>
						<div>
							<span>目前工作</span>
							<h2>需要你處理</h2>
						</div>
						<PortalBadge>共 {entries.length} 項</PortalBadge>
					</header>
					<div className={styles.portalEntryList}>
						{entries.map((entry) => (
							<Link
								key={`${entry.title}-${entry.status}`}
								href={entry.href}
								className={styles.portalEntry}
							>
								<span className={styles.entryIcon}>
									<AppIcon name="document" size={21} />
								</span>
								<span className={styles.entryContent}>
									<span>
										<PortalBadge tone={entry.tone}>{entry.status}</PortalBadge>
										<small>{entry.meta}</small>
									</span>
									<strong>{entry.title}</strong>
									<small>{entry.detail}</small>
								</span>
								<span className={styles.entryAction}>
									{entry.action}
									<AppIcon name="chevron-right" size={17} />
								</span>
							</Link>
						))}
					</div>
				</section>

				<aside className={styles.portalHomeSide}>
					<PortalCard
						title="企業驗證"
						action={<PortalBadge tone="warning">2 項待補</PortalBadge>}
					>
						<ul className={styles.checkList}>
							<li data-complete>
								<AppIcon name="check" size={18} />
								<span>
									<strong>公司與聯絡資料</strong>
									<small>示意為已填寫</small>
								</span>
							</li>
							<li>
								<AppIcon name="clock" size={18} />
								<span>
									<strong>負責人身分證明</strong>
									<small>尚未送審</small>
								</span>
							</li>
							<li>
								<AppIcon name="lock" size={18} />
								<span>
									<strong>收款帳戶</strong>
									<small>需再次驗證後設定</small>
								</span>
							</li>
						</ul>
						<div className={styles.sideActions}>
							<Button
								href={externalVendorStepPath("business-verification")}
								fullWidth
							>
								繼續企業驗證
							</Button>
						</div>
					</PortalCard>
					<div className={styles.sessionNote}>
						<AppIcon name="info" size={19} />
						<p>
							<strong>換新裝置時</strong>重新使用受邀信箱登入即可；正式 v5.0
							會撤銷舊裝置 Session。
						</p>
					</div>
				</aside>
			</div>
		</div>
	);
}

function BusinessVerificationView() {
	const [documents, setDocuments] = React.useState({
		registration: true,
		representative: false,
		bank: false,
	});
	const completed = Object.values(documents).filter(Boolean).length;

	return (
		<div className={styles.contentGrid}>
			<div className={styles.mainColumn}>
				<PortalCard
					title="企業與聯絡資料"
					description="欄位分組顯示，之後由後端驗證統編、負責人與收款主體是否一致。"
					action={<PortalBadge tone="warning">草稿</PortalBadge>}
				>
					<div className={styles.formGrid}>
						<label className={styles.fieldWide}>
							<span>公司／商號名稱</span>
							<input defaultValue="範例產地直送有限公司" />
						</label>
						<label>
							<span>統一編號</span>
							<input inputMode="numeric" defaultValue="53548721" />
							<small>後端需檢核 8 碼格式與商工登記資料。</small>
						</label>
						<label>
							<span>負責人姓名</span>
							<input defaultValue="林豐田" />
						</label>
						<label>
							<span>聯絡手機</span>
							<input type="tel" defaultValue="0912 345 678" />
						</label>
						<label>
							<span>聯絡信箱</span>
							<input type="email" defaultValue="sales@fengtian.example" />
						</label>
						<label className={styles.fieldWide}>
							<span>登記地址</span>
							<input defaultValue="桃園市大園區中正東路 168 號" />
						</label>
					</div>
				</PortalCard>

				<PortalCard
					title="資格文件"
					description="前端先呈現檔案類型、檢核狀態與補件方式；目前不會上傳。"
					action={
						<PortalBadge tone={completed === 3 ? "success" : "info"}>
							{completed}/3 已備妥
						</PortalBadge>
					}
				>
					<div className={styles.documentList}>
						{[
							{
								id: "registration" as const,
								title: "公司／商業登記證明",
								detail: "PDF、JPG 或 PNG，10 MB 以內",
							},
							{
								id: "representative" as const,
								title: "負責人身分證明",
								detail: "正反面清晰彩色影像，資料不得遮蔽",
							},
							{
								id: "bank" as const,
								title: "公司收款帳戶證明",
								detail: "戶名需與公司或負責人資料一致",
							},
						].map((document) => {
							const ready = documents[document.id];
							return (
								<div key={document.id} data-ready={ready || undefined}>
									<span className={styles.documentStatus}>
										<AppIcon
											name={ready ? "check" : "paper-upload"}
											size={20}
										/>
									</span>
									<span className={styles.documentDetails}>
										<strong>{document.title}</strong>
										<small>{ready ? "示意檔案已加入" : document.detail}</small>
									</span>
									<Button
										variant="bordered"
										color="default"
										size="sm"
										onClick={() =>
											setDocuments((current) => ({
												...current,
												[document.id]: !current[document.id],
											}))
										}
									>
										{ready ? "移除示意檔" : "加入示意檔"}
									</Button>
								</div>
							);
						})}
					</div>
				</PortalCard>
			</div>

			<aside className={styles.sideColumn}>
				<PortalCard title="驗證進度" description="實際狀態需由 KYC API 回傳。">
					<ul className={styles.checkList}>
						<li data-complete>
							<AppIcon name="check" size={18} />
							<span>
								<strong>邀請與聯絡方式</strong>
								<small>前端示意為已完成</small>
							</span>
						</li>
						<li>
							<AppIcon name="clock" size={18} />
							<span>
								<strong>企業與負責人</strong>
								<small>尚待後端送審</small>
							</span>
						</li>
						<li>
							<AppIcon name="lock" size={18} />
							<span>
								<strong>收款合作商店</strong>
								<small>KYC 通過後建立</small>
							</span>
						</li>
					</ul>
					<div className={styles.sideActions}>
						<Button fullWidth disabled>
							送出驗證（API 待接）
						</Button>
						<Button
							href={externalVendorStepPath("rfq")}
							variant="bordered"
							color="default"
							fullWidth
						>
							預覽驗證後流程
						</Button>
					</div>
					<p className={styles.apiHint}>此操作只切換頁面，不會建立驗證案件。</p>
				</PortalCard>
			</aside>
		</div>
	);
}

function RfqDetailView() {
	return (
		<div className={styles.contentGrid}>
			<div className={styles.mainColumn}>
				<PortalCard
					title="企業活動蔬果箱"
					description="買家已完成需求與價格確認；請先閱讀完整採購範圍，再查看已計算的報價條件。"
					action={<PortalBadge tone="success">買家已確認</PortalBadge>}
				>
					<RequestFacts />
				</PortalCard>

				<PortalCard title="品項與交付規格">
					<div className={styles.specTableScroll}>
						<table className={styles.specTable}>
							<caption>詢價品項規格</caption>
							<thead>
								<tr>
									<th scope="col">品項</th>
									<th scope="col">規格</th>
									<th scope="col">數量</th>
								</tr>
							</thead>
							<tbody>
								<tr>
									<th scope="row">當季綜合蔬果箱</th>
									<td>每箱 8–10 種、總重 5 公斤以上</td>
									<td>80 箱</td>
								</tr>
								<tr>
									<th scope="row">企業活動分裝</th>
									<td>單箱貼標、附保存說明與品項卡</td>
									<td>80 份</td>
								</tr>
							</tbody>
						</table>
					</div>
					<ul className={styles.requirements}>
						<li>
							<AppIcon name="check" size={18} />
							可提供農產品產銷履歷或來源證明
						</li>
						<li>
							<AppIcon name="check" size={18} />
							全程冷藏配送，指定時段一次送達
						</li>
						<li>
							<AppIcon name="check" size={18} />
							可開立統一發票；買家條件已註明含稅與冷藏運費
						</li>
					</ul>
				</PortalCard>

				<PortalCard
					title="附件與買家備註"
					description="附件下載與已讀狀態之後由 RFQ API 提供。"
				>
					<div className={styles.attachmentList}>
						<div>
							<AppIcon name="paper-download" size={20} />
							<span className={styles.contactDetails}>
								<strong>活動收貨與卸貨說明.pdf</strong>
								<small>1.8 MB｜買家於 8/10 更新</small>
							</span>
							<Button variant="light" color="default" size="sm" disabled>
								下載（API 待接）
							</Button>
						</div>
					</div>
					<blockquote>
						活動當日上午會有專人協助卸貨；若無法符合品項、價格或交期，請使用「提出異議」，不要自行改寫買家已確認的詢價單。
					</blockquote>
				</PortalCard>
			</div>

			<aside className={styles.sideColumn}>
				<PortalCard title="本次回覆">
					<div className={styles.responseSummary}>
						<div>
							<span>回覆截止</span>
							<strong>剩餘 2 天 4 小時</strong>
						</div>
						<div>
							<span>目前版本</span>
							<strong>買家確認 v4</strong>
						</div>
						<div>
							<span>企業驗證</span>
							<strong>待後端審核</strong>
						</div>
					</div>
					<div className={styles.sideActions}>
						<Button
							href={externalVendorStepPath("quote")}
							fullWidth
							endContent={<AppIcon name="arrow-right" size={18} />}
						>
							查看已計算報價
						</Button>
						<Button variant="light" color="default" fullWidth disabled>
							提出問題（API 待接）
						</Button>
					</div>
				</PortalCard>
			</aside>
		</div>
	);
}

function BuyerOfferView() {
	const offer = EXTERNAL_VENDOR_BUYER_OFFER;

	return (
		<div className={styles.quoteLayout}>
			<div className={styles.mainColumn}>
				<section className={styles.offerHero}>
					<div>
						<PortalBadge tone="success">買家已確認 {offer.version}</PortalBadge>
						<h1>青田設計已確認這份採購條件</h1>
						<p>
							價格已由 Pricebook
							試算並經買家確認。你不需要重新填寫報價，只要確認是否能依這份條件供貨。
						</p>
					</div>
					<dl className={styles.offerApproval}>
						<div>
							<dt>買家確認時間</dt>
							<dd>{offer.buyerConfirmedAt}</dd>
						</div>
						<div>
							<dt>詢價單編號</dt>
							<dd>{RFQ_DEMO_SUMMARY.rfqNo}</dd>
						</div>
					</dl>
				</section>

				<PortalCard
					title="買家定稿的報價單"
					description="這是已送達供應商的唯讀版本；價格、數量與運費不能在此頁直接修改。"
					action={<PortalBadge>唯讀文件</PortalBadge>}
				>
					<div className={styles.offerLineItem}>
						<header>
							<div>
								<small>品項 01</small>
								<strong>當季綜合蔬果箱＋企業活動分裝</strong>
							</div>
							<PortalBadge>{offer.quantity} 箱</PortalBadge>
						</header>
						<div className={styles.offerTableScroll}>
							<table>
								<caption>買家已確認的報價明細</caption>
								<thead>
									<tr>
										<th scope="col">數量</th>
										<th scope="col">批發成交單價</th>
										<th scope="col">小計</th>
									</tr>
								</thead>
								<tbody>
									<tr>
										<td>{offer.quantity} 箱</td>
										<td>{money(offer.transactionUnitPrice)}／箱</td>
										<td>{money(offer.subtotal)}</td>
									</tr>
								</tbody>
							</table>
						</div>
					</div>
				</PortalCard>

				<PortalCard
					title="Pricebook 試算依據"
					description="顯示買家確認前使用的價格基準，讓供應商理解這份價格如何形成。"
					action={
						<PortalBadge tone="info">基準日 {offer.pricebookAsOf}</PortalBadge>
					}
				>
					<div className={styles.offerRange}>
						<div>
							<span>買家可接受單價範圍</span>
							<strong>
								{money(offer.buyerMinimumUnitPrice)}–
								{money(offer.buyerMaximumUnitPrice)}／箱
							</strong>
							<small>若要超出範圍，必須提出異議並等待買家重新確認。</small>
						</div>
						<div>
							<span>理想價格與品質取向</span>
							<strong>
								{money(offer.transactionUnitPrice)}／箱｜{offer.qualityTarget}
							</strong>
							<small>兼顧履歷來源、每箱重量與活動分裝品質。</small>
						</div>
					</div>
					<div className={styles.pricebookGrid}>
						<div>
							<span>建議批發成交價</span>
							<strong>{money(offer.transactionUnitPrice)}／箱</strong>
							<small>本次提供給供應商的成交單價</small>
						</div>
						<div>
							<span>零售參考價</span>
							<strong>{money(offer.retailUnitPrice)}／箱</strong>
							<small>批發成交價 × {offer.retailMultiplier}</small>
						</div>
						<div>
							<span>買家確認狀態</span>
							<strong>已鎖定 {offer.version}</strong>
							<small>{offer.buyerConfirmedAt}</small>
						</div>
					</div>
					<div className={styles.pricingFormula}>
						<AppIcon name="chart" size={22} />
						<p>
							<strong>計價規則</strong>此 fixture
							依指定規則計算：零售參考價＝批發成交價 × 2。正式版必須由後端
							Pricebook
							回傳價格、基準日與版本，前端不得自行決定成交價。規則版本：
							{offer.pricingRuleVersion}。
						</p>
					</div>
				</PortalCard>

				<PortalCard title="交付與驗收條件">
					<ul className={styles.offerTerms}>
						<li>
							<AppIcon name="calendar" size={19} />
							<span>
								<strong>指定交貨</strong>
								<small>{offer.deliveryAt}送達臺北市信義區指定地點</small>
							</span>
						</li>
						<li>
							<AppIcon name="document" size={19} />
							<span>
								<strong>品項規格</strong>
								<small>
									每箱 9 種當季蔬果、總重約 5.2 公斤，含活動分裝與品項卡
								</small>
							</span>
						</li>
						<li>
							<AppIcon name="shield" size={19} />
							<span>
								<strong>替代限制</strong>
								<small>
									任何替代品必須先取得買家書面同意，不得由供應商自行更換
								</small>
							</span>
						</li>
					</ul>
				</PortalCard>
			</div>

			<aside className={styles.quoteSummary}>
				<div className={styles.summaryTitle}>
					<span>待供應商回覆</span>
					<PortalBadge tone="warning">{offer.validUntil} 前</PortalBadge>
				</div>
				<dl>
					<div>
						<dt>
							{offer.quantity} 箱 × {money(offer.transactionUnitPrice)}
						</dt>
						<dd>{money(offer.subtotal)}</dd>
					</div>
					<div>
						<dt>冷藏運費</dt>
						<dd>{money(offer.freight)}</dd>
					</div>
					<div>
						<dt>稅額</dt>
						<dd>已含</dd>
					</div>
					<div className={styles.totalRow}>
						<dt>採購總額</dt>
						<dd>{money(offer.total)}</dd>
					</div>
				</dl>
				<div className={styles.priceSignal}>
					<AppIcon name="chart" size={20} />
					<span>
						<strong>價格已由買家確認</strong>
						<small>
							Pricebook {offer.pricebookVersion}｜基準日 {offer.pricebookAsOf}
						</small>
					</span>
				</div>
				<div className={styles.decisionPrompt}>
					<h2>你可以依這份條件供貨嗎？</h2>
					<p>接受前會再顯示一次完整確認頁。</p>
					<Button href={externalVendorStepPath("review")} fullWidth>
						接受這份條件
					</Button>
					<Button variant="bordered" color="default" fullWidth disabled>
						價格或交期需要調整（API 待接）
					</Button>
					<Button variant="light" color="default" fullWidth disabled>
						無法供貨（API 待接）
					</Button>
				</div>
				<p className={styles.apiHint}>前端預覽不會送出任何回覆。</p>
			</aside>
		</div>
	);
}

function ReviewView() {
	const [confirmed, setConfirmed] = React.useState(false);
	const offer = EXTERNAL_VENDOR_BUYER_OFFER;

	return (
		<div className={styles.reviewLayout}>
			<div className={styles.reviewDocument}>
				<header>
					<div>
						<PortalBadge tone="success">買家確認 {offer.version}</PortalBadge>
						<h1>接受前，請確認你能履行每一項條件</h1>
						<p>
							你接受的是買家已定稿的詢價單；若價格、品質或交期不同，應提出異議而不是直接改寫文件。
						</p>
					</div>
					<span className={styles.documentNumber}>
						{RFQ_DEMO_SUMMARY.rfqNo}
					</span>
				</header>

				<section>
					<h2>交易雙方</h2>
					<dl className={styles.reviewRows}>
						<div>
							<dt>買家</dt>
							<dd>範例設計股份有限公司</dd>
						</div>
						<div>
							<dt>供應商</dt>
							<dd>範例產地直送有限公司</dd>
						</div>
						<div>
							<dt>統一編號</dt>
							<dd>53548721</dd>
						</div>
						<div>
							<dt>企業資格</dt>
							<dd>待 KYC API 確認</dd>
						</div>
					</dl>
				</section>

				<section>
					<h2>買家定稿價格</h2>
					<dl className={styles.reviewRows}>
						<div>
							<dt>採購數量</dt>
							<dd>{offer.quantity} 箱</dd>
						</div>
						<div>
							<dt>理想成交單價</dt>
							<dd>
								{money(offer.transactionUnitPrice)}（{offer.tax}）
							</dd>
						</div>
						<div>
							<dt>買家接受範圍</dt>
							<dd>
								{money(offer.buyerMinimumUnitPrice)}–
								{money(offer.buyerMaximumUnitPrice)}／箱
							</dd>
						</div>
						<div>
							<dt>品質取向</dt>
							<dd>{offer.qualityTarget}</dd>
						</div>
						<div>
							<dt>冷藏運費</dt>
							<dd>{money(offer.freight)}</dd>
						</div>
						<div data-total>
							<dt>採購總額</dt>
							<dd>{money(offer.total)}</dd>
						</div>
					</dl>
				</section>

				<section>
					<h2>交付與驗收條件</h2>
					<ul className={styles.reviewBullets}>
						<li>{offer.deliveryAt}送達臺北市信義區指定地點。</li>
						<li>每箱 9 種蔬果、總重約 5.2 公斤，含活動分裝。</li>
						<li>任何替代品都需先取得買家書面同意，不得自行更換。</li>
						<li>供應商回覆期限為 {offer.validUntil}。</li>
					</ul>
				</section>
			</div>

			<aside className={styles.submitPanel}>
				<div className={styles.submitState}>
					<AppIcon name="shield" size={23} />
					<span>
						<strong>接受邊界</strong>
						<small>這個前端版本不會真的通知買家。</small>
					</span>
				</div>
				<ul>
					<li>價格與品質條件由買家確認後送達供應商。</li>
					<li>供應商接受後保留操作者、時間與詢價單版本。</li>
					<li>接受不等於已付款；付款成功後才建立正式訂單。</li>
				</ul>
				<label className={styles.confirmation}>
					<input
						type="checkbox"
						checked={confirmed}
						onChange={(event) => setConfirmed(event.currentTarget.checked)}
					/>
					<span>我有權代表供應商，並確認能依價格、品質、數量與交期履行。</span>
				</label>
				<Button
					href={externalVendorStepPath("transaction")}
					fullWidth
					disabled={!confirmed}
					endContent={<AppIcon name="arrow-right" size={18} />}
				>
					預覽接受後狀態
				</Button>
				<div className={styles.alternativeActions}>
					<Button variant="bordered" color="default" fullWidth disabled>
						提出價格或交期異議（API 待接）
					</Button>
					<Button variant="light" color="default" fullWidth disabled>
						婉拒本次詢價（API 待接）
					</Button>
				</div>
				<p className={styles.apiHint}>不會建立供應商回覆，也不會通知買家。</p>
			</aside>
		</div>
	);
}

function TransactionView() {
	const [scenario, setScenario] =
		React.useState<TransactionScenario>("negotiation");
	const activeScenario =
		TRANSACTION_SCENARIOS.find((item) => item.id === scenario) ??
		TRANSACTION_SCENARIOS[0];
	const activeIndex = TRANSACTION_SCENARIOS.findIndex(
		(item) => item.id === scenario,
	);

	const timeline = [
		{
			title: "買家詢價單已送出",
			detail: "採購條件 v4｜買家確認時間 8/10 15:20",
		},
		{
			title: "供應商回覆",
			detail:
				scenario === "negotiation"
					? "供應商提出價格異議，等待買家確認新版本"
					: "供應商已接受採購條件，等待買家完成付款",
		},
		{
			title: "付款與訂單成立",
			detail:
				activeIndex >= 2
					? "平台已確認入帳，訂單 HK202608150014 成立"
					: "付款成功後才會建立唯一正式訂單",
		},
		{
			title: "備貨、出貨與收貨",
			detail:
				activeIndex >= 3
					? "買家已確認收貨，售後申訴期計算中"
					: "訂單成立後由供應商更新履約狀態",
		},
		{
			title: "發票與結算",
			detail:
				activeIndex >= 3
					? "發票待審核；款項仍未排入匯款批次"
					: "收貨與售後條件滿足後才可進入結算",
		},
	];

	return (
		<div className={styles.transactionStack}>
			<div className={styles.scenarioBar}>
				<div>
					<small>前端情境預覽</small>
					<strong>切換交易階段，檢查每個狀態的資訊與操作</strong>
				</div>
				<div
					className={styles.scenarioTabs}
					role="tablist"
					aria-label="交易情境"
				>
					{TRANSACTION_SCENARIOS.map((item) => (
						<button
							key={item.id}
							type="button"
							role="tab"
							aria-selected={scenario === item.id}
							onClick={() => setScenario(item.id)}
						>
							{item.label}
						</button>
					))}
				</div>
			</div>

			<section className={styles.transactionHero}>
				<div>
					<PortalBadge tone={activeScenario.tone}>
						{activeScenario.status}
					</PortalBadge>
					<h1>
						{scenario === "negotiation"
							? "採購條件 v4｜價格異議處理"
							: "訂單 HK202608150014"}
					</h1>
					<p>
						{scenario === "negotiation"
							? "供應商沒有改寫買家詢價單，而是另行提出異議；買家接受、維持原條件或建立新版都必須留下版本紀錄。"
							: "買家詢價、供應商接受、付款、履約與結算各自保留狀態，不讓同一顆按鈕跨越多個交易責任。"}
					</p>
				</div>
				<div className={styles.transactionAmount}>
					<small>
						{scenario === "negotiation" ? "買家原始採購總額" : "成交總額"}
					</small>
					<strong>{money(22000)}</strong>
					<span>含稅、含冷藏運費</span>
				</div>
			</section>

			<div className={styles.transactionGrid}>
				<PortalCard title="交易進度">
					<ol className={styles.timeline}>
						{timeline.map((item, index) => (
							<li
								key={item.title}
								data-complete={index < activeIndex || undefined}
								data-active={index === activeIndex || undefined}
							>
								<span className={styles.timelineMarker}>
									{index < activeIndex ? (
										<AppIcon name="check" size={16} />
									) : (
										index + 1
									)}
								</span>
								<span>
									<strong>{item.title}</strong>
									<small>{item.detail}</small>
								</span>
							</li>
						))}
					</ol>
				</PortalCard>

				<div className={styles.transactionSide}>
					<PortalCard title="目前可做的事">
						{scenario === "negotiation" ? (
							<>
								<div className={styles.counterOffer}>
									<span>供應商提出的異議</span>
									<strong>希望調整為每箱 NT$275</strong>
									<small>調整後總額 NT$22,800｜仍在買家上限 NT$280 內</small>
								</div>
								<div className={styles.sideActions}>
									<Button fullWidth disabled>
										查看買家回覆（API 待接）
									</Button>
									<Button variant="bordered" color="default" fullWidth disabled>
										補充異議說明（API 待接）
									</Button>
								</div>
							</>
						) : null}

						{scenario === "selected" ? (
							<div className={styles.lockedAction}>
								<AppIcon name="clock" size={22} />
								<strong>等待買家完成付款</strong>
								<p>
									這個階段供應商不能提前出貨，也不能把「已選定」當成已付款訂單。
								</p>
							</div>
						) : null}

						{scenario === "fulfillment" ? (
							<>
								<div className={styles.fulfillmentFacts}>
									<div>
										<span>最晚出貨</span>
										<strong>8/14 12:00</strong>
									</div>
									<div>
										<span>指定送達</span>
										<strong>8/15 09:30</strong>
									</div>
									<div>
										<span>發票</span>
										<strong>尚未上傳</strong>
									</div>
								</div>
								<div className={styles.sideActions}>
									<Button fullWidth disabled>
										開始備貨（API 待接）
									</Button>
									<Button variant="bordered" color="default" fullWidth disabled>
										上傳發票（API 待接）
									</Button>
								</div>
							</>
						) : null}

						{scenario === "settlement" ? (
							<>
								<div className={styles.settlementRows}>
									<div>
										<span>訂單總額</span>
										<strong>{money(22000)}</strong>
									</div>
									<div>
										<span>平台費用</span>
										<strong>待後端計算</strong>
									</div>
									<div>
										<span>預計應付</span>
										<strong>待雙審確認</strong>
									</div>
								</div>
								<div className={styles.lockedAction}>
									<AppIcon name="lock" size={21} />
									<strong>尚未具備可結算證據</strong>
									<p>發票、售後 freeze 與財務雙審 API 完成後才顯示匯款日期。</p>
								</div>
							</>
						) : null}
					</PortalCard>

					<PortalCard title="交易聯絡">
						<div className={styles.contactCard}>
							<span className={styles.contactAvatar}>日</span>
							<span className={styles.contactDetails}>
								<strong>Hawkr 交易支援</strong>
								<small>買賣雙方訊息需保留於交易紀錄</small>
							</span>
							<Button variant="light" color="default" size="sm" disabled>
								聯絡支援
							</Button>
						</div>
					</PortalCard>
				</div>
			</div>
		</div>
	);
}

function StepBody({ step }: { step: ExternalVendorStep }) {
	switch (step) {
		case "invitation":
			return <InvitationView />;
		case "exchange":
			return <ExchangeView />;
		case "sign-in":
			return <SignInView />;
		case "verify-code":
			return <VerifyCodeView />;
		case "session-expired":
			return <SessionExpiredView />;
		case "invite-unavailable":
			return <InviteUnavailableView />;
		case "portal":
			return <PortalHomeView />;
		case "business-verification":
			return <BusinessVerificationView />;
		case "rfq":
			return <RfqDetailView />;
		case "quote":
			return <BuyerOfferView />;
		case "review":
			return <ReviewView />;
		case "transaction":
			return <TransactionView />;
	}
}

export default function ExternalVendorPortal({
	step,
}: {
	step: ExternalVendorStep;
}) {
	const definition = EXTERNAL_VENDOR_STEPS.find((item) => item.slug === step);
	const transactionStep = EXTERNAL_VENDOR_TRANSACTION_STEPS.find(
		(item) => item.slug === step,
	)?.slug;
	const isAuthenticatedSurface = Boolean(transactionStep || step === "portal");

	return (
		<div className={styles.portal} data-vendor-step={step}>
			<header className={styles.topbar}>
				<Link
					href={externalVendorStepPath("invitation")}
					className={styles.brand}
					aria-label="Hawkr 外部供應商入口"
				>
					<BrandLogo
						variant="horizontal"
						size="md"
						surface="plain"
						className={styles.brandLogo}
					/>
					<span className={styles.brandPortalLabel}>
						<strong>供應商報價</strong>
						<small>安全交易入口</small>
					</span>
				</Link>
				<div className={styles.topbarRequest}>
					<span>
						{isAuthenticatedSurface ? "已驗證的供應商入口" : "安全邀請與登入"}
					</span>
					<strong>
						{step === "portal"
							? "豐田產地直送｜工作台"
							: "青田設計｜企業活動蔬果箱"}
					</strong>
				</div>
				{isAuthenticatedSurface ? (
					<div className={styles.vendorAccount}>
						<span className={styles.vendorAvatar}>豐</span>
						<span>
							<strong>豐田產地直送</strong>
							<small>受邀信箱已驗證</small>
						</span>
						<Button variant="light" color="default" size="sm" disabled>
							登出
						</Button>
					</div>
				) : (
					<span className={styles.secureEntry}>
						<AppIcon name="lock" size={17} />
						安全交易入口
					</span>
				)}
			</header>

			{transactionStep ? <PortalProgress current={transactionStep} /> : null}

			<main className={styles.main}>
				<FrontendOnlyNotice />
				{transactionStep ? (
					<header className={styles.pageHeader}>
						<div>
							<span>外部供應商 Portal</span>
							<h1>{definition?.label}</h1>
							<p>{definition?.description}</p>
						</div>
						<PortalBadge tone="warning">尚待 API 串接</PortalBadge>
					</header>
				) : null}
				<StepBody step={step} />
			</main>

			<footer className={styles.footer}>
				<span>Hawkr 外部供應商交易入口</span>
				<span>僅限受邀業者｜示意資料不構成交易</span>
			</footer>
		</div>
	);
}
