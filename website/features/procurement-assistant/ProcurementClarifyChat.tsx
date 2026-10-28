"use client";

import * as React from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import Button from "@/components/ui/Button";
import SendButton from "@/features/messages/components/SendButton";
import { useAuth } from "@/lib/auth/context";
import { useComposerScroll } from "@/lib/hooks/useComposerScroll";
import ClarifyQuestionCard from "./ClarifyQuestionCard";
import CropDisambiguationCard from "./CropDisambiguationCard";
import HawkrAgentTypingRow from "./HawkrAgentTypingRow";
import { legacyReplyToBlocks, narrativeIcon } from "./narrative";
import { RUN_CANCEL_COPY } from "./runtime-snapshot-contract";
import { slotLabel } from "./session-logic";
import { groupLabel, shouldStartGroup } from "./transcript-groups";
import type {
	ClarifyQuestion,
	CropDisambiguation,
	CropDisambiguationOption,
	ProcurementChatTurn,
} from "./types";
import type { RunCancelState } from "./useProcurementSession";

const MAX_MESSAGE_LENGTH = 2000;
/** 稿 01 的 composer 停用態逐字文案：只說「不能輸入」不夠，要說為什麼。 */
const PENDING_PLACEHOLDER = "執行中，暫時無法輸入訊息...";

/**
 * 停止鈕按不下去時的說明。
 *
 * 取消命令要帶這一輪 run 的 CAS token，而那個 token 只有伺服器的 runtime
 * snapshot 給得出來；讀不到（端點還沒上線、這一輪還沒寫進快照、或現在跑的是
 * 不可取消的候選搜尋）時按鈕就該按不下去。這句話講的是**現在停不了**，不是
 * 「功能沒做」——寫成後者會在功能已經上線時繼續誤導。
 */
const STOP_UNAVAILABLE_HINT = "目前無法確認這一輪的執行狀態，暫時不能停止";

/**
 * 「已記下」是不可互動的欄位摘要，不是下一步 action。圖示只幫助掃讀；
 * 文字仍保留完整 label/value，未知欄位也只落到中性的文件圖示。
 */
const RECORDED_FIELD_ICON: Readonly<Record<string, AppIconName>> = {
	item: "goods",
	quantity: "cubes",
	unit: "reorder",
	packaging: "box",
	budgetPerUnit: "wallet",
	budget_per_unit: "wallet",
	deliveryCity: "location",
	delivery_city: "location",
	deliveryDate: "calendar",
	delivery_date: "calendar",
	exactMatchRequired: "scan",
	exact_match_required: "scan",
	itemResolution: "category",
	item_resolution: "category",
};

function recordedFieldIcon(key: string): AppIconName {
	return RECORDED_FIELD_ICON[key] ?? "paper";
}

/** 保留後端／使用者文字中的換行，不靠 white-space CSS 才看得見。 */
function TextWithLineBreaks({ text }: { text: string }) {
	const children: React.ReactNode[] = [];
	let cursor = 0;
	for (const match of text.matchAll(/\r?\n/g)) {
		const breakAt = match.index;
		children.push(text.slice(cursor, breakAt));
		children.push(<br key={`line-break:${breakAt}`} />);
		cursor = breakAt + match[0].length;
	}
	children.push(text.slice(cursor));
	return <>{children}</>;
}

interface ProcurementClarifyChatProps {
	turns: ProcurementChatTurn[];
	restoredSummary: string | null;
	pending: boolean;
	/** 當前處理列，固定在主對話區，與正式回覆分開。 */
	processing?: React.ReactNode;
	/** #2549 Phase 3：剛完成這一輪的 assistant turn id（見
	 * useProcurementSession.ts 的同名欄位）；只有 id 對得上的那一輪 blocks
	 * 套交錯淡入，歷史／重播的 turn 一律不套動畫。 */
	justCompletedTurnId?: string | null;
	/** 進行中那一輪「現在在做什麼」的那一行字，由 workspace 從 liveActivity／
	 * liveSteps 算好傳進來。缺席＝還沒收到第一步（或這輪走的是拿不到串流的降級
	 * 路徑），等待中就顯示中性的三點指示。
	 *
	 * 未提供 processing 插槽的獨立呼叫仍可使用單行等待。 */
	latestActivityLabel?: string;
	/** 最新一輪的問題卡（B-244 Ask 3）；null＝這輪沒有問題（或後端尚未上線）。 */
	question?: ClarifyQuestion | null;
	onPickOption?: (option: {
		slotKey: string;
		optionId: string;
		optionLabel: string;
	}) => void;
	/** 跳過這題；未傳（必填題）＝不渲染跳過按鈕。 */
	onSkipOption?: (payload: { slotKey: string; skip: true }) => void;
	/** 問題卡內的自由文字答案；走 answers 端點而不是把答案誤送成新一輪 turns。 */
	onSubmitFreeText?: (payload: { slotKey: string; freeText: string }) => void;
	/** quantity 複合問題卡／單位追問卡點選單位。 */
	onPickUnit?: (payload: {
		slotKey: string;
		unitOptionId: string;
		unitLabel: string;
		freeText?: string;
	}) => void;
	/** crop_disambiguation 問句選「D 其他」子表單提交（品項分類消歧
	 * gate）；帶上 question.slotKey 走 answers 端點，同 onSubmitFreeText 的
	 * 傳遞慣例。 */
	onSubmitCropOther?: (payload: {
		slotKey: string;
		description: string;
	}) => void;
	/** 最新一輪的品項消歧問題卡（B-244 Ask 3.5）；null＝這輪沒有待處理的消歧。 */
	cropDisambiguation?: CropDisambiguation | null;
	onChooseCropDisambiguation?: (option: CropDisambiguationOption) => void;
}

/**
 * Transcript half of the clarify stage. Renders `<li>` rows only, so it
 * drops straight into the shared `.thread-page__list` inside the
 * scroller. The composer is a separate export because — exactly as in
 * the Messages thread — it belongs *outside* the scroller as a
 * `.thread-page__compose` sibling; nesting it in a bubble left it
 * outside the compose-bar CSS and scrolling away with the bubbles.
 */
export default function ProcurementClarifyChat({
	turns,
	restoredSummary,
	pending,
	processing,
	justCompletedTurnId = null,
	latestActivityLabel,
	question = null,
	onPickOption,
	onSkipOption,
	onSubmitFreeText,
	onPickUnit,
	onSubmitCropOther,
	cropDisambiguation = null,
	onChooseCropDisambiguation,
}: ProcurementClarifyChatProps) {
	const answeredViaChipRef = React.useRef(false);
	const { identity } = useAuth();
	const buyerName = identity?.nickname?.trim() || undefined;
	const buyerAvatarLabel = buyerName ? `${buyerName} 的大頭貼` : "你的大頭貼";
	// Date.now() 不能直接在 SSR/hydration render 裡呼叫：跨過午夜或只差幾毫秒
	// 都可能讓 server/client 產出不同 separator。首屏用 turn 自己的時間作穩定
	// fallback；hydrate 後再一次校正「今天／日期」。
	const [groupNow, setGroupNow] = React.useState<number | null>(null);
	React.useEffect(() => {
		setGroupNow(Date.now());
	}, []);

	return (
		<>
			{restoredSummary ? (
				<li className="hawkr-agent-phase1__system-note" role="status">
					{restoredSummary}
				</li>
			) : null}

			{turns.length === 0 && !restoredSummary ? (
				<li className="hawkr-agent-narrative">
					<p>
						告訴我你想採購什麼——品項、數量、預算、配送城市與到貨日，想到什麼先說什麼。
					</p>
				</li>
			) : null}

			{turns.map((turn, index) => {
				// 時間戳不再掛在每個氣泡上：只有這一則與上一則隔得夠久（或跨日）
				// 才在前面插一條置中分隔線，同一批連續對話共用一個時間。
				const startsGroup = shouldStartGroup(turn, turns[index - 1]);
				return (
					<React.Fragment key={turn.id}>
						{startsGroup ? (
							<li className="thread-page__separator" role="presentation">
								<span>
									{groupLabel(turn.createdAt, groupNow ?? turn.createdAt)}
								</span>
							</li>
						) : null}
						<li
							className={
								turn.role === "buyer"
									? "hawkr-agent-message-row is-mine"
									: "hawkr-agent-narrative"
							}
						>
							<div
								className={
									turn.role === "buyer" ? "thread-page__bubble is-mine" : ""
								}
							>
								{turn.role === "assistant" ? (
									/* §V.10.6-4：assistant 完成回覆採**無泡泡正文**，每個語意
									 * block 左側一枚 allowlisted AppIcon，且不重複 Agent
									 * avatar。#2549 Phase 2 起優先使用後端真正的 blocks[]；缺席
									 * （pre-Phase-2 回應，或舊資料列重播）時才落回把整包 reply
									 * 包成單一 SUMMARY block 的 legacy 規則，不自己切段猜語意。
									 * #2549 Phase 3：只有 justCompletedTurnId 對得上這一輪
									 * 才套交錯淡入——歷史 turn（reload／scroll 出來的）與 buyer
									 * 自己的 turn 都不套，見 useProcurementSession.ts
									 * justCompletedTurnId 的 doc comment。純前端動畫，blocks[]
									 * 本身沿用既有終態一次性交付，沒有新的 wire 事件。 */
									(turn.blocks ?? legacyReplyToBlocks(turn.text)).map(
										(block, index) => {
											const icon = narrativeIcon(block.kind);
											const isJustCompleted = turn.id === justCompletedTurnId;
											return (
												<div
													// biome-ignore lint/suspicious/noArrayIndexKey: blocks 無唯一 ID，順序穩定
													key={index}
													className={
														isJustCompleted
															? "hawkr-agent-narrative__block hawkr-agent-narrative__block--stagger-in"
															: "hawkr-agent-narrative__block"
													}
													style={
														isJustCompleted
															? { animationDelay: `${index * 120}ms` }
															: undefined
													}
												>
													{icon ? (
														<AppIcon name={icon} size={18} aria-hidden="true" />
													) : (
														// icon 為 null（未知 kind）時仍需佔住 grid 第一欄——容器是
														// grid-template-columns: 18px minmax(0, 1fr)，沒有第一欄
														// 元素時 CSS auto-placement 會把 <p> 擠進 18px 欄，配上
														// overflow-wrap: anywhere 讓整段文字變成逐字換行的豎排。
														<span aria-hidden="true" />
													)}
													<p>
														<TextWithLineBreaks text={block.text} />
													</p>
												</div>
											);
										},
									)
								) : (
									<p>
										<TextWithLineBreaks text={turn.text} />
									</p>
								)}
								{turn.role === "assistant" && turn.recordedThisTurn?.length ? (
									<ul
										className="hawkr-agent-message__facts"
										aria-label="已記下的條件"
									>
										{turn.recordedThisTurn.map((field) => (
											<li key={field.key} data-slot-key={field.key}>
												<span
													className="hawkr-agent-message__fact-icon"
													aria-hidden="true"
												>
													<AppIcon
														name={recordedFieldIcon(field.key)}
														size={18}
													/>
												</span>
												<span className="hawkr-agent-message__fact-copy">
													<span className="hawkr-agent-message__fact-label">
														{field.label}：
													</span>
													<strong className="hawkr-agent-message__fact-value">
														{field.value}
													</strong>
												</span>
											</li>
										))}
									</ul>
								) : null}
								{turn.role === "assistant" && turn.rejectedThisTurn ? (
									<aside
										className="hawkr-agent-message__rejected"
										aria-label="這輪未採用的條件"
									>
										<AppIcon name="info" size={18} aria-hidden="true" />
										<div>
											<strong>有些條件需要重新確認</strong>
											<ul>
												{turn.rejectedThisTurn.map((field) => (
													<li key={`${field.key}-${field.reason}`}>
														<span>{slotLabel(field.key)}</span>
														<span>{field.reason}</span>
													</li>
												))}
											</ul>
										</div>
									</aside>
								) : null}
							</div>
							{turn.role === "buyer" ? (
								<Avatar
									src={identity?.avatarKey}
									name={buyerName}
									alt={buyerAvatarLabel}
									entity="buyer"
									fallback={buyerName ? "initial" : "placeholder"}
									className="hawkr-agent-message-avatar hawkr-agent-message-avatar--buyer"
									loading="eager"
								/>
							) : null}
						</li>
					</React.Fragment>
				);
			})}

			{/* 等待中且已收到即時步驟 → 顯示當下那一步（節奏由伺服器決定）。
			 * 等待中但還沒有任何步驟 → 中性三點指示，不編造階段。
			 * 回覆落地後兩者都消失——思考過程只在思考的時候存在。 */}
			{pending && processing ? (
				<li className="hawkr-agent-processing-row">{processing}</li>
			) : pending ? (
				/* 還沒收到第一個合法 public step ＝ 尚未觀察到伺服器開始處理；這不等於
				 * durable ACK。§V.10.6-2：只顯示中性等待，不編造步驟；§V.10.10-2
				 * 也禁止在這裡循環播放，所以 motion 一律是不循環的 SUBMITTING。
				 *
				 * 顯示收到的步驟不是 §V.10.6-2 的「編造步驟」；這一列也不是
				 * §V.10.6-3 保護的對話紀錄。 */
				<HawkrAgentTypingRow
					label={latestActivityLabel}
					motion="SUBMITTING"
					ariaLabel="Hawkr Agent 正在整理"
				/>
			) : null}

			{!pending && cropDisambiguation && onChooseCropDisambiguation ? (
				<CropDisambiguationCard
					disambiguation={cropDisambiguation}
					pending={pending}
					onChoose={onChooseCropDisambiguation}
				/>
			) : null}

			{!pending && question && onPickOption ? (
				<ClarifyQuestionCard
					key={question.slotKey}
					question={question}
					pending={pending}
					onPick={(option) => {
						answeredViaChipRef.current = true;
						onPickOption(option);
					}}
					onSubmitFreeText={
						onSubmitFreeText
							? (freeText) =>
									onSubmitFreeText({ slotKey: question.slotKey, freeText })
							: undefined
					}
					onSubmitCropOther={
						onSubmitCropOther
							? (payload) =>
									onSubmitCropOther({
										slotKey: question.slotKey,
										description: payload.description,
									})
							: undefined
					}
					onSkip={
						onSkipOption
							? (payload) => {
									answeredViaChipRef.current = true;
									onSkipOption(payload);
								}
							: undefined
					}
					onPickUnit={
						onPickUnit
							? (payload) => {
									answeredViaChipRef.current = true;
									onPickUnit(payload);
								}
							: undefined
					}
					// 上一題是點 chip 答的 → 新卡掛載時把焦點還給第一顆選項，
					// 鍵盤流程不落回 <body>（surface 契約的 focus return）。
					autoFocusFirstOption={answeredViaChipRef.current}
				/>
			) : null}
		</>
	);
}

interface ProcurementClarifyComposerProps {
	pending: boolean;
	/** 目前草稿。**由父層持有**——見下方元件註解的「單一 composer」規則。 */
	value: string;
	onValueChange: (next: string) => void;
	onSubmit: (message: string) => void;
	/** 有 active question 時走 /answers，單一 slot 與後端 domain.MaxTextLen
	 * 同步限制 200 字；一般 /turns 維持 2000 字。 */
	maxLength?: number;
	placeholder?: string;
	validate?: (message: string) => string | null;
	/**
	 * `entry`＝§V.11.0 的大型中央 composer（min-height 160–192px、桌機佔主內容
	 * pane 的 70–82% 且不超過 960px）；`thread`＝首送後移到對話底部的一般尺寸。
	 * 兩者是同一個元件的兩種尺寸，不是兩個 composer。
	 */
	variant?: "entry" | "thread";
	/** 讓 entry 的 chips／起手卡點擊後把焦點移進輸入框（§V.11.0）。 */
	inputRef?: React.RefObject<HTMLTextAreaElement | null>;
	/**
	 * 這一輪拿不拿得到取消命令要的 run 身分與 CAS token。
	 *
	 * false 時停止鈕**看得到但按不下去**：按鈕消失會讓人以為自己漏看了，
	 * 按得下去則會送出一個必然被伺服器以 400 擋下的請求。
	 */
	canStop?: boolean;
	/** 取消命令的結果；由 `useProcurementSession` 持有。 */
	stopState?: RunCancelState;
	/** 按下停止。未提供時按鈕一律不可用。 */
	onStop?: () => void;
}

/**
 * Compose bar for the clarify stage — the same single-pill field as the
 * Messages thread, minus the attach / image buttons (Hawkr Agent takes
 * text only; there is no image upload or order attachment to send it).
 *
 * 草稿是**受控的**，狀態放在 `AssistantWorkspace`。這不是風格選擇，是設計書
 * §V.11.0 的兩條硬要求：
 *
 * 1.「同一 text-only composer 保留 prompt……並移到 thread 底部，不生成第二套
 *    composer」：entry 與 thread 是同一個元件的兩個位置，React 會在位置改變時
 *    重掛，內部 `useState` 的草稿就會蒸發。草稿放父層才真的是同一份。
 * 2.「首送失敗保留 prompt」：舊版在 `onSubmit` 之後立刻清空草稿，送出失敗時
 *    使用者要重打一次。現在由父層等真實 public step 或成功結果後才清。
 */
export function ProcurementClarifyComposer({
	pending,
	value,
	onValueChange,
	onSubmit,
	maxLength = MAX_MESSAGE_LENGTH,
	placeholder = "例如：水果 50 箱，每箱預算 800 元，送到桃園市",
	validate,
	variant = "thread",
	inputRef,
	canStop = false,
	stopState = { status: "idle" },
	onStop,
}: ProcurementClarifyComposerProps) {
	const [validationError, setValidationError] = React.useState<string | null>(
		null,
	);
	const draftLength = Array.from(value).length;
	const overLimit = draftLength > maxLength;
	const hasError = overLimit || Boolean(validationError);
	const canSend = Boolean(value.trim()) && !overLimit && !pending;
	// 按得下去的唯一條件：伺服器給得出這一輪的 CAS token，而且這次還沒收到過
	// 回執。已經 settled／stale 之後再按沒有第二件事可做；`error` 是例外——
	// 那一次命令沒有成立，重試是有意義的。
	const stopAnswered =
		stopState.status === "settled" || stopState.status === "stale";
	const canPressStop =
		Boolean(onStop) &&
		canStop &&
		stopState.status !== "requesting" &&
		!stopAnswered;
	// 四個 disposition 各講各的話：只有 REQUESTED 是「正在停止」，另外三個
	// 講的是這一輪已經到終點、已完成的結果沒有被作廢（RUN_CANCEL_COPY）。
	const stopNotice =
		stopState.status === "settled"
			? RUN_CANCEL_COPY[stopState.disposition]
			: stopState.status === "stale"
				? "這一輪的狀態已經變了，請重新整理確認目前結果。"
				: stopState.status === "error"
					? stopState.detail
					: null;
	const fallbackRef = React.useRef<HTMLTextAreaElement | null>(null);
	const textareaRef = inputRef ?? fallbackRef;
	React.useLayoutEffect(() => {
		void value; // 草稿改變後重新量測換行高度。
		const textarea = textareaRef.current;
		if (!textarea) return;
		const resize = () => {
			if (!textarea.getClientRects().length) return;
			textarea.style.height = "auto";
			const maxHeight =
				Number.parseFloat(getComputedStyle(textarea).maxHeight) || 120;
			textarea.style.height = `${Math.min(textarea.scrollHeight, maxHeight)}px`;
		};
		resize();
		let previousWidth = textarea.clientWidth;
		const observer = new ResizeObserver(() => {
			if (previousWidth === textarea.clientWidth) return;
			previousWidth = textarea.clientWidth;
			resize();
		});
		observer.observe(textarea);
		return () => observer.disconnect();
	}, [textareaRef, value]);
	useComposerScroll(textareaRef, value);

	const submit = () => {
		if (!canSend) return;
		const message = value.trim();
		const error = validate?.(message) ?? null;
		if (error) {
			setValidationError(error);
			return;
		}
		setValidationError(null);
		onSubmit(message);
	};

	return (
		<>
			{hasError ? (
				<p
					id="procurement-clarify-error"
					className="thread-page__notice hawkr-agent-phase1__compose-alert"
					role="alert"
				>
					{overLimit
						? `單則訊息上限 ${maxLength} 字，目前 ${draftLength} 字。`
						: validationError}
				</p>
			) : null}

			{stopNotice ? (
				<p
					id="procurement-clarify-stop-status"
					data-testid="agent-stop-status"
					// 只有真的失敗才用 alert 的 danger 色。四個 disposition 都是成功
					// 回應，把「這一輪已經跑完了」畫成紅字等於把一個正常結果講成錯誤。
					className={
						stopState.status === "error"
							? "thread-page__notice hawkr-agent-phase1__compose-alert"
							: "thread-page__notice"
					}
					role="status"
				>
					{stopNotice}
				</p>
			) : null}

			<form
				className="thread-page__compose"
				data-composer-variant={variant}
				onSubmit={(event) => {
					event.preventDefault();
					submit();
				}}
			>
				<div className="thread-page__field">
					<label htmlFor="procurement-clarify-input" className="sr-only">
						採購需求輸入
					</label>
					<textarea
						id="procurement-clarify-input"
						ref={textareaRef}
						className="hawkr-agent-composer__input"
						wrap="soft"
						enterKeyHint="send"
						value={value}
						onChange={(event) => {
							onValueChange(event.currentTarget.value);
							setValidationError(null);
						}}
						onKeyDown={(event) => {
							if (
								event.key !== "Enter" ||
								event.shiftKey ||
								event.nativeEvent.isComposing
							)
								return;
							event.preventDefault();
							submit();
						}}
						placeholder={pending ? PENDING_PLACEHOLDER : placeholder}
						title="按 Enter 送出，Shift + Enter 換行"
						rows={1}
						maxLength={maxLength}
						aria-invalid={hasError ? "true" : undefined}
						aria-describedby={
							hasError ? "procurement-clarify-error" : undefined
						}
						disabled={pending}
					/>
					{pending ? (
						<>
							<Button
								type="button"
								data-testid="agent-stop-run"
								variant="bordered"
								color="danger"
								size="md"
								disabled={!canPressStop}
								isLoading={stopState.status === "requesting"}
								loadingLabel="停止中"
								onClick={onStop}
								className="min-h-11 min-w-11 shrink-0"
								title={canStop ? undefined : STOP_UNAVAILABLE_HINT}
								aria-describedby={
									canStop ? undefined : "procurement-clarify-stop-hint"
								}
							>
								停止處理
							</Button>
							{canStop ? null : (
								<span id="procurement-clarify-stop-hint" className="sr-only">
									{STOP_UNAVAILABLE_HINT}
								</span>
							)}
						</>
					) : null}
					<SendButton
						type="submit"
						disabled={!canSend}
						loading={pending}
						ariaLabel="送出"
					/>
				</div>
			</form>
		</>
	);
}
