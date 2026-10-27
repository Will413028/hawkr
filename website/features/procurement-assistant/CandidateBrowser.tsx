"use client";

import * as React from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import { Button, Chip, Tabs } from "@/components/ui";
import type {
	ProcurementCandidate,
	ProcurementCandidateSearch,
	ProcurementLane,
} from "@/lib/api/openapi-types";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import Link from "@/lib/intl/native-link";
import {
	type CandidateSellerGroup,
	candidatesToToggleForSeller,
	canGroupBySeller,
	countSelectedRecipients,
	groupPlatformCandidates,
	sellerSelectionState,
} from "./candidate-grouping";
import { hasPartialLaneFailure } from "./session-logic";

/** lane 對買家的說法。**只有這一份**——`AssistantWorkspace` 的候選詳情面板
 * 原本各自寫死一組同義字串，清單改了詳情不會跟著改。 */
export const LANE_LABEL: Record<ProcurementLane, string> = {
	PLATFORM: "平台賣家",
	EXTERNAL: "外部業者",
};

/**
 * 兩條 lane 各自的代號圖示：平台賣家是一間店，站外業者是一只箱子。
 *
 * 兩條 lane 的可信度不同（平台賣家有 KYC，站外業者只有公開商務資訊），而候選列的
 * 其餘欄位對兩者長得一模一樣——沒有這個代號，買家得讀完 badge 文字才分得出來。
 *
 * 字形取自共用 registry 現有的名字。registry 沒有「店面」「紙箱」這兩個字形，
 * 所以這裡不新造字形，取語意最近的
 * 「建物 vs 容器」把兩條 lane 分開。
 */
const LANE_AVATAR_ICON: Record<ProcurementLane, AppIconName> = {
	PLATFORM: "home",
	EXTERNAL: "box",
};

/**
 * `matchedFacts[].key` 逐列對應的圖示。
 *
 * key 的值域是後端的 `domain.SlotKey`（`services/ai-service/internal/procurement/
 * domain/slots.go`），`MatchFactFor` 只會用這些值。整欄共用同一個勾不帶任何資訊——
 * 逐列給字形，買家掃一眼就知道命中的是品項、價格還是配送。
 *
 * ⚠️ 值域外的 key 退回勾，不猜也不隱藏：後端日後擴充 slot 時，多出來的那一列仍要
 * 列得出來（B2 的 evidence contract 升格就會加 key）。
 */
const FACT_GLYPH_BY_SLOT: Record<string, AppIconName> = {
	item: "leaf",
	item_resolution: "leaf",
	quantity: "cubes",
	unit: "scan",
	packaging: "box",
	budget_per_unit: "tag",
	delivery_city: "location",
	delivery_date: "calendar",
	exact_match_required: "shield",
};

function factGlyph(key: string): AppIconName {
	return FACT_GLYPH_BY_SLOT[key] ?? "check";
}

/** 候選列的身分區塊：代號圖示＋名稱。兩種列（單一候選／收斂後的商家）共用同一份，
 * 只有圖示來源不同——商家有 logo 就用 logo，其餘一律用 lane 代號。 */
function CandidateIdentity({
	lane,
	name,
	logoUrl,
}: {
	lane: ProcurementLane;
	name: string;
	logoUrl?: string | null;
}) {
	return (
		<span className="assistant-candidate-identity" data-lane={lane}>
			<span className="assistant-candidate-identity__avatar" aria-hidden="true">
				{logoUrl ? (
					<AppImage
						src={logoUrl}
						alt=""
						width={32}
						height={32}
						preset="avatar"
					/>
				) : (
					<AppIcon name={LANE_AVATAR_ICON[lane]} size={19} />
				)}
			</span>
			<strong>{name}</strong>
		</span>
	);
}

/** 比對分數方塊。窄版靠 CSS 貼到卡片右上角自成一塊，所以「比對」與「94 分」要是
 * 兩個具名子元素——留成裸文字節點時，grid 會把它們拆成各自佔一列的匿名 item，
 * 「94」與「分」會上下分家。 */
function CandidateScore({ score }: { score: number }) {
	return (
		<span className="assistant-candidate-row__score">
			<span className="assistant-candidate-row__score-label">比對</span>
			<span className="assistant-candidate-row__score-value">
				<b>{score}</b> 分
			</span>
		</span>
	);
}

interface CandidateBrowserProps {
	/**
	 * 直接用 ProcurementCandidateSearch，不再手寫一份結構相同的 inline 型別。
	 *
	 * 原本這裡寫的是 `status: string`，於是下面兩處 `candidateSet.status === "..."`
	 * 的字面值比較在 tsc 眼中永遠合法——run 狀態值域改名時，這兩處是全 repo 最容易漏、
	 * 且漏了完全沒有訊號的地方（`string === "PARTIAL"` 不是型別錯誤，只是恆為 false）。
	 * 呼叫端本來傳的就是完整的 ProcurementCandidateSearch，這個寬型別沒有換到任何東西。
	 */
	candidateSet: ProcurementCandidateSearch;
	candidatesStale: boolean;
	pendingSelectionCandidateIds: ReadonlySet<string>;
	onToggleSelection: (candidate: ProcurementCandidate) => void;
	onOpenCandidate: (candidate: ProcurementCandidate) => void;
	onRetrySearch: () => void;
	onStartRfqDraft: (candidates: ProcurementCandidate[]) => void;
	/** 價格試算 capability 尚未開啟時，不顯示會進入 RFQ 草稿的新 CTA。 */
	rfqDraftEnabled: boolean;
	headingRef?: React.Ref<HTMLHeadingElement>;
}

function formatFactsAsOf(iso: string): string {
	return formatTaipeiDateTime(iso) || iso;
}

function CandidateRow({
	candidate,
	disabled,
	pending,
	onToggle,
	onOpen,
}: {
	candidate: ProcurementCandidate;
	disabled: boolean;
	pending: boolean;
	onToggle: () => void;
	onOpen: () => void;
}) {
	const checkboxId = `procurement-candidate-${candidate.candidateId}`;
	// 這個工作台只靠 server 已提供的 public facts 說真話；既然沒有可打開的
	// typed seller artifact，就不能再用卡片摘要的 slice 隱藏其餘查核結果。
	const matchedFacts = candidate.matchedFacts;
	const unverifiableFacts = candidate.unverifiableFacts;
	return (
		<tr
			className="assistant-candidate-row"
			data-selected={candidate.selected || undefined}
			aria-busy={pending || undefined}
		>
			<td className="assistant-candidate-row__select">
				<label
					className="assistant-candidate-row__select-control"
					htmlFor={checkboxId}
				>
					<input
						id={checkboxId}
						type="checkbox"
						checked={candidate.selected}
						disabled={disabled || pending}
						onChange={onToggle}
						aria-label={`${candidate.selected ? "取消選取" : "選取"} ${candidate.displayName}`}
					/>
					<span
						className="assistant-candidate-row__selection-mark"
						data-selected={candidate.selected || undefined}
						aria-hidden="true"
					>
						{candidate.selected ? <AppIcon name="check" size={17} /> : null}
					</span>
					<span className="assistant-candidate-row__select-text">
						{pending ? "更新中" : candidate.selected ? "已選" : "選取"}
					</span>
				</label>
			</td>

			<td className="assistant-candidate-row__identity">
				<CandidateIdentity
					lane={candidate.lane}
					name={candidate.displayName}
					logoUrl={candidate.brandLogoUrl}
				/>
				<span className="assistant-candidate-row__meta">
					<span
						className="assistant-candidate-row__lane"
						data-lane={candidate.lane}
					>
						{LANE_LABEL[candidate.lane]}
					</span>
					<CandidateScore score={candidate.score} />
				</span>
			</td>

			<td className="assistant-candidate-row__actions">
				<button
					type="button"
					className="assistant-candidate-row__details"
					onClick={onOpen}
					aria-label={`查看 ${candidate.displayName} 詳情`}
				>
					查看詳情
					<AppIcon name="chevron-right" size={17} aria-hidden="true" />
				</button>
			</td>

			<td className="assistant-candidate-row__facts">
				<span className="assistant-candidate-row__mobile-label">符合條件</span>
				{matchedFacts.length > 0 ? (
					<ul>
						{matchedFacts.map((fact) => (
							<li key={fact.key}>
								<AppIcon
									name={factGlyph(fact.key)}
									size={15}
									aria-hidden="true"
								/>
								<span>{fact.label}</span>
							</li>
						))}
					</ul>
				) : (
					<span className="assistant-candidate-row__empty">尚無已查核條件</span>
				)}
			</td>

			<td className="assistant-candidate-row__pending">
				<span className="assistant-candidate-row__mobile-label">待確認</span>
				{unverifiableFacts.length > 0 ? (
					<ul>
						{unverifiableFacts.map((fact) => (
							<li key={fact.key}>
								<AppIcon name="warning" size={15} aria-hidden="true" />
								<span>{fact.reason}</span>
							</li>
						))}
					</ul>
				) : (
					<span className="assistant-candidate-row__empty">
						沒有額外待確認項目
					</span>
				)}
			</td>

			<td className="assistant-candidate-row__date">
				<span className="assistant-candidate-row__mobile-label">資料時間</span>
				<time dateTime={candidate.factsAsOf}>
					{formatFactsAsOf(candidate.factsAsOf)}
				</time>
			</td>

			<td className="assistant-candidate-row__status">
				<span className="assistant-candidate-row__mobile-label">狀態</span>
				<Chip variant="flat" color="warning" size="sm">
					待進一步查核
				</Chip>
			</td>
		</tr>
	);
}

/**
 * 一家商家一列。
 *
 * 候選的單位是**收件人**不是品項：買家在這一步要決定的是「找哪幾家問」，所以同一家
 * 商家底下的多個符合商品收斂成一列，只顯示筆數，不列出商品。詳細理由見
 * `candidate-grouping.ts` 的檔頭。
 *
 * 勾選這一列＝勾選它底下的全部候選（詢價單一家只需要問一次）。部分勾選以
 * `indeterminate` 表達，不用第三個圖示——那是 checkbox 本來就有的語意。
 *
 * 刻意不用 `SellerBusinessCard`：那個元件吃 `SellerShareCardData`（10 個欄位，含封面／
 * 描述／分享文案），候選 DTO 只帶得出其中 3 個，硬接會逼出一輪額外請求，而這個工作台
 * 要的只是「這是哪一家」。完整名片留在商家頁。
 */
function SellerRow({
	group,
	disabled,
	pending,
	onToggle,
}: {
	group: CandidateSellerGroup;
	disabled: boolean;
	pending: boolean;
	onToggle: () => void;
}) {
	const checkboxId = `procurement-seller-${group.storeId}`;
	const selection = sellerSelectionState(group);
	const checked = selection === "all";
	const inputRef = React.useRef<HTMLInputElement>(null);

	// indeterminate 只有 DOM property，沒有對應的 HTML attribute，React 也不會幫忙設，
	// 只能在 ref 上寫。漏掉的話「部分勾選」會顯示成完全未選，使用者以為自己沒選到。
	React.useEffect(() => {
		if (inputRef.current) {
			inputRef.current.indeterminate = selection === "partial";
		}
	}, [selection]);

	return (
		<tr
			className="assistant-candidate-row"
			data-selected={checked || undefined}
			aria-busy={pending || undefined}
		>
			<td className="assistant-candidate-row__select">
				<label
					className="assistant-candidate-row__select-control"
					htmlFor={checkboxId}
				>
					<input
						ref={inputRef}
						id={checkboxId}
						type="checkbox"
						checked={checked}
						disabled={disabled || pending}
						onChange={onToggle}
						aria-label={`${checked ? "取消選取" : "選取"} ${group.sellerName}`}
					/>
					<span
						className="assistant-candidate-row__selection-mark"
						data-selected={checked || undefined}
						data-partial={selection === "partial" || undefined}
						aria-hidden="true"
					>
						{checked ? <AppIcon name="check" size={17} /> : null}
					</span>
					<span className="assistant-candidate-row__select-text">
						{pending
							? "更新中"
							: selection === "all"
								? "已選"
								: selection === "partial"
									? "部分已選"
									: "選取"}
					</span>
				</label>
			</td>

			<td className="assistant-candidate-row__identity">
				<CandidateIdentity
					lane="PLATFORM"
					name={group.sellerName}
					logoUrl={group.logoUrl}
				/>
				<span className="assistant-candidate-row__meta">
					<span className="assistant-candidate-row__lane" data-lane="PLATFORM">
						{LANE_LABEL.PLATFORM}
					</span>
					<CandidateScore score={group.score} />
					<span className="assistant-candidate-row__match-count">
						{group.candidates.length} 項符合的商品
					</span>
				</span>
				{/* storeCode 缺值時不給連結——`/sellers/{storeId}` 那條路徑正在被收掉，
				    用 storeId 頂替會產生一個會壞掉的連結。 */}
			</td>

			<td className="assistant-candidate-row__actions">
				{group.storeCode ? (
					<Link
						className="assistant-candidate-row__details"
						href={`/sellers/${group.storeCode}`}
					>
						查看商家
						<AppIcon name="chevron-right" size={17} aria-hidden="true" />
					</Link>
				) : null}
			</td>

			<td className="assistant-candidate-row__facts">
				<span className="assistant-candidate-row__mobile-label">符合條件</span>
				{group.matchedFacts.length > 0 ? (
					<ul>
						{group.matchedFacts.map((fact) => (
							<li key={fact.key}>
								<AppIcon
									name={factGlyph(fact.key)}
									size={15}
									aria-hidden="true"
								/>
								<span>{fact.label}</span>
							</li>
						))}
					</ul>
				) : (
					<span className="assistant-candidate-row__empty">尚無已查核條件</span>
				)}
			</td>

			<td className="assistant-candidate-row__pending">
				<span className="assistant-candidate-row__mobile-label">待確認</span>
				{group.unverifiableFacts.length > 0 ? (
					<ul>
						{group.unverifiableFacts.map((fact) => (
							<li key={fact.key}>
								<AppIcon name="warning" size={15} aria-hidden="true" />
								<span>{fact.reason}</span>
							</li>
						))}
					</ul>
				) : (
					<span className="assistant-candidate-row__empty">
						沒有額外待確認項目
					</span>
				)}
			</td>

			<td className="assistant-candidate-row__date">
				<span className="assistant-candidate-row__mobile-label">資料時間</span>
				<time dateTime={group.factsAsOf}>
					{formatFactsAsOf(group.factsAsOf)}
				</time>
			</td>

			<td className="assistant-candidate-row__status">
				<span className="assistant-candidate-row__mobile-label">狀態</span>
				<Chip variant="flat" color="warning" size="sm">
					待進一步查核
				</Chip>
			</td>
		</tr>
	);
}

function LanePanel({
	lane,
	candidates,
	disabled,
	pendingSelectionCandidateIds,
	onToggleSelection,
	onOpenCandidate,
	query,
	onQueryChange,
}: {
	lane: ProcurementLane;
	candidates: ProcurementCandidate[];
	disabled: boolean;
	pendingSelectionCandidateIds: ReadonlySet<string>;
	onToggleSelection: (candidate: ProcurementCandidate) => void;
	onOpenCandidate: (candidate: ProcurementCandidate) => void;
	query: string;
	onQueryChange: (value: string) => void;
}) {
	const normalizedQuery = query.trim().toLocaleLowerCase("zh-Hant");
	const visibleCandidates = normalizedQuery
		? candidates.filter((candidate) =>
				[candidate.displayName, ...candidate.matchedFacts.map((f) => f.label)]
					.join(" ")
					.toLocaleLowerCase("zh-Hant")
					.includes(normalizedQuery),
			)
		: candidates;
	const selectedCount = candidates.filter((c) => c.selected).length;
	// EXTERNAL lane 的候選來自站外名單，永遠沒有平台賣家可分組——這個判斷由
	// canGroupBySeller 的資料判準自然涵蓋，不需要另外寫 lane 的特例。
	const grouped = canGroupBySeller(visibleCandidates)
		? groupPlatformCandidates(visibleCandidates)
		: null;

	const renderRow = (candidate: ProcurementCandidate) => (
		<CandidateRow
			key={candidate.candidateId}
			candidate={candidate}
			disabled={disabled}
			pending={pendingSelectionCandidateIds.has(candidate.candidateId)}
			onToggle={() => onToggleSelection(candidate)}
			onOpen={() => onOpenCandidate(candidate)}
		/>
	);

	// 勾選一家商家＝逐筆切換它底下需要改變狀態的候選。這一層刻意不新增「批次 toggle」
	// 的 API：勾選的持久化契約是 per-candidate 的
	// （POST .../selections 帶單一 candidateId），前端自己展開才不會在 wire 上多開一條
	// 語意重複的路徑。
	const onToggleSeller = (group: CandidateSellerGroup) => {
		for (const candidate of candidatesToToggleForSeller(group).candidates) {
			onToggleSelection(candidate);
		}
	};

	return (
		<div className="assistant-lane" data-lane={lane}>
			<div className="assistant-lane__intro">
				<div>
					{/* 收斂成商家之後，「幾筆」講的必須是商家數——沿用候選數會讓標題與
					    畫面上看得到的列數對不起來。 */}
					<strong>
						{grouped
							? `${grouped.groups.length + grouped.ungrouped.length} 家商家`
							: `${candidates.length} 筆候選`}
					</strong>
					<span>已選 {selectedCount} 筆</span>
				</div>
				<p>
					{lane === "PLATFORM"
						? "候選不是供貨承諾；正式報價才代表賣家願意承接。"
						: "外部資料只顯示公開商務資訊；完整聯絡資料不會出現在這個工作台。"}
				</p>
			</div>
			<div className="assistant-lane__filter">
				<AppIcon name="search" size={18} aria-hidden="true" />
				<input
					type="search"
					value={query}
					onChange={(event) => onQueryChange(event.currentTarget.value)}
					placeholder={`搜尋${LANE_LABEL[lane]}候選或條件`}
					aria-label={`搜尋${LANE_LABEL[lane]}候選`}
				/>
				{query ? (
					<button type="button" onClick={() => onQueryChange("")}>
						清除
					</button>
				) : null}
			</div>

			{visibleCandidates.length > 0 ? (
				<table
					className="assistant-candidate-table"
					aria-label={`${LANE_LABEL[lane]}候選資料`}
				>
					<thead className="assistant-candidate-table__head">
						<tr>
							<th scope="col">選取</th>
							<th scope="col">候選</th>
							<th scope="col">符合條件</th>
							<th scope="col">待確認</th>
							<th scope="col">資料時間</th>
							<th scope="col">狀態</th>
							<th scope="col">操作</th>
						</tr>
					</thead>
					{/* 收斂成商家是資料驅動的（見 candidate-grouping.ts）：候選實際帶得出
					    商家身分才收斂，帶不出就是原本這張候選平表。沒有任何模式開關，
					    所以不會出現「看起來有、點進去空」的分頁。 */}
					<tbody className="assistant-candidate-table__body">
						{grouped
							? [
									...grouped.groups.map((group) => (
										<SellerRow
											key={`seller-${group.storeId}`}
											group={group}
											disabled={disabled}
											pending={group.candidates.some((c) =>
												pendingSelectionCandidateIds.has(c.candidateId),
											)}
											onToggle={() => onToggleSeller(group)}
										/>
									)),
									// 部署窗口內混到的舊候選：照樣完整列出，不靜默丟掉。
									...grouped.ungrouped.map(renderRow),
								]
							: visibleCandidates.map(renderRow)}
					</tbody>
				</table>
			) : (
				<div className="assistant-lane__empty" role="status">
					<AppIcon name="search" size={24} aria-hidden="true" />
					<strong>這個頁籤沒有符合篩選的候選</strong>
					<p>已選名單不受影響；清除搜尋即可查看全部候選。</p>
					<Button
						variant="bordered"
						color="default"
						onClick={() => onQueryChange("")}
					>
						清除搜尋
					</Button>
				</div>
			)}
		</div>
	);
}

export default function CandidateBrowser({
	candidateSet,
	candidatesStale,
	pendingSelectionCandidateIds,
	onToggleSelection,
	onOpenCandidate,
	onRetrySearch,
	onStartRfqDraft,
	rfqDraftEnabled,
	headingRef,
}: CandidateBrowserProps) {
	const [activeLane, setActiveLane] =
		React.useState<ProcurementLane>("PLATFORM");
	const [laneQueries, setLaneQueries] = React.useState<
		Record<ProcurementLane, string>
	>({
		PLATFORM: "",
		EXTERNAL: "",
	});

	if (candidateSet.status === "FAILED") {
		return (
			<section
				className="assistant-artifact assistant-candidates"
				aria-labelledby="candidate-title"
			>
				<header className="assistant-artifact__head">
					<h3 id="candidate-title" ref={headingRef} tabIndex={-1}>
						候選搜尋失敗
					</h3>
				</header>
				<div className="assistant-invalidated" role="alert">
					<AppIcon name="danger" size={20} aria-hidden="true" />
					<div>
						{/* failureReason 是機器可讀的失敗原因；缺值時仍要有可讀的替代文案，
						    否則這塊 alert 會整個空白。 */}
						<strong>{candidateSet.failureReason || "搜尋失敗"}</strong>
						<p>請重新搜尋一次候選。</p>
					</div>
				</div>
				<Button onClick={onRetrySearch}>重新搜尋</Button>
			</section>
		);
	}

	const { platform, external } = candidateSet.candidates;
	const selectedCandidates = [...platform, ...external].filter(
		(candidate) => candidate.selected,
	);
	const selectedCount = selectedCandidates.length;
	// 行動列講「家」，勾選契約講「筆」：同一家商家底下勾了三個候選，畫面上是一列、
	// 打出去是三次 POST .../selections。兩個數字都要有——`selectedCount` 管按鈕能不能按
	// （一筆都沒有就不能），`selectedRecipientCount` 管說出來的話跟看到的列數對得上。
	const selectedRecipientCount = countSelectedRecipients(selectedCandidates);

	return (
		<section
			className="assistant-artifact assistant-candidates"
			aria-labelledby="candidate-title"
		>
			<header className="assistant-artifact__head">
				<div>
					<h3 id="candidate-title" ref={headingRef} tabIndex={-1}>
						候選名單
					</h3>
					<p>勾選加入比較清單，不會通知商家。</p>
				</div>
				{/* 契約凍結後 run 狀態不再帶「部分成功」這個資訊：判準是「run 成功但有 lane
				    失敗」，那正是新值域對部分成功的定義。 */}
				{hasPartialLaneFailure(candidateSet) ? (
					<Chip variant="flat" color="warning" size="sm">
						部分來源未完成
					</Chip>
				) : null}
			</header>

			{candidatesStale ? (
				<div className="assistant-invalidated" role="alert">
					<AppIcon name="warning" size={20} aria-hidden="true" />
					<div>
						<strong>候選資料已更新</strong>
						<p>請重新搜尋候選後再勾選。</p>
					</div>
					<Button variant="bordered" color="default" onClick={onRetrySearch}>
						重新搜尋
					</Button>
				</div>
			) : null}

			{/* 實心膠囊而不是底線分頁：這兩個頁籤是「要看哪一批候選」的主要切換，
			    在窄面板裡底線的命中區與選中感都太弱。文字用 LANE_LABEL，不再另寫
			    一組同義字（兩顆分頁原本各自寫死一份）——那正是本檔頂端那條註解在防的漂移。 */}
			<Tabs
				aria-label="候選來源"
				variant="solid"
				fullWidth
				selectedKey={activeLane}
				onSelectionChange={(key) => setActiveLane(key as ProcurementLane)}
				items={[
					{
						key: "PLATFORM",
						label: (
							<span className="assistant-tab-label">
								{LANE_LABEL.PLATFORM} <b>{platform.length}</b>
							</span>
						),
						content: (
							<LanePanel
								lane="PLATFORM"
								candidates={platform}
								disabled={candidatesStale}
								pendingSelectionCandidateIds={pendingSelectionCandidateIds}
								onToggleSelection={onToggleSelection}
								onOpenCandidate={onOpenCandidate}
								query={laneQueries.PLATFORM}
								onQueryChange={(value) =>
									setLaneQueries((current) => ({ ...current, PLATFORM: value }))
								}
							/>
						),
					},
					{
						key: "EXTERNAL",
						label: (
							<span className="assistant-tab-label">
								{LANE_LABEL.EXTERNAL} <b>{external.length}</b>
							</span>
						),
						content: (
							<LanePanel
								lane="EXTERNAL"
								candidates={external}
								disabled={candidatesStale}
								pendingSelectionCandidateIds={pendingSelectionCandidateIds}
								onToggleSelection={onToggleSelection}
								onOpenCandidate={onOpenCandidate}
								query={laneQueries.EXTERNAL}
								onQueryChange={(value) =>
									setLaneQueries((current) => ({ ...current, EXTERNAL: value }))
								}
							/>
						),
					},
				]}
			/>

			<div className="assistant-rfq-action-bar">
				<div className="assistant-rfq-action-bar__count" aria-live="polite">
					<span>
						<AppIcon name="users" size={19} aria-hidden="true" />
					</span>
					<div>
						<strong>
							{selectedCount > 0
								? `已選 ${selectedRecipientCount} 家`
								: "尚未選擇商家"}
						</strong>
						<small>
							{rfqDraftEnabled
								? "下一步可整理詢價內容，現在不會通知商家。"
								: "詢價草稿尚未開放；選取候選不會建立或通知任何人。"}
						</small>
					</div>
				</div>
				{rfqDraftEnabled ? (
					<Button
						disabled={
							selectedCount === 0 ||
							candidatesStale ||
							pendingSelectionCandidateIds.size > 0
						}
						onClick={() => onStartRfqDraft(selectedCandidates)}
						endContent={
							<AppIcon name="arrow-right" size={18} aria-hidden="true" />
						}
					>
						建立詢價草稿
					</Button>
				) : null}
			</div>
		</section>
	);
}
