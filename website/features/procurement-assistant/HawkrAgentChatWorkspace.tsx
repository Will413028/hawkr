"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import SendButton from "@/features/messages/components/SendButton";
import { AiStreamTimeoutError } from "@/lib/ai-chat/stream-client";
import {
	type SellerCandidate,
	safeSellerWebsiteUrl,
	streamSellerMatchChat,
} from "@/lib/buyer/seller-match-chat";
import { formatTaipeiTime } from "@/lib/date/taipei";
import { useKeyboardInset } from "@/lib/hooks/useKeyboardInset";
import Link from "@/lib/intl/native-link";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import HawkrAgentMark from "./HawkrAgentMark";
import {
	consumeHawkrAgentHandoff,
	type HawkrAgentHandoffSource,
} from "./handoff";
import { ASSISTANT_PROCUREMENT_PATH } from "./routes";

type TurnStatus = "streaming" | "complete" | "stopped" | "error";

interface HawkrAgentTurn {
	id: string;
	role: "buyer" | "agent";
	text: string;
	status: TurnStatus;
	createdAt: number;
	candidates?: SellerCandidate[];
	candidateResultReceived?: boolean;
}

const SUGGESTED_REQUESTS = [
	"找企業送禮水果盒",
	"找餐飲用冷凍水產",
	"找中部有機蔬菜",
] as const;

const HANDOFF_SOURCE_LABEL: Record<HawkrAgentHandoffSource, string> = {
	"home-search": "首頁搜尋",
	"search-page": "商品搜尋",
	"search-empty": "搜尋結果",
};

function createTurnId(prefix: "buyer" | "agent") {
	try {
		return `${prefix}-${globalThis.crypto.randomUUID()}`;
	} catch {
		return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2)}`;
	}
}

function updateTurn(
	turns: HawkrAgentTurn[],
	id: string,
	updater: (turn: HawkrAgentTurn) => HawkrAgentTurn,
) {
	return turns.map((turn) => (turn.id === id ? updater(turn) : turn));
}

function websiteLabel(href: string) {
	try {
		return new URL(href).hostname.replace(/^www\./, "");
	} catch {
		return "公開網站";
	}
}

function AgentAvatar({ intro = false }: { intro?: boolean }) {
	return (
		<span
			className={
				intro
					? "thread-intro__avatar hawkr-agent-thread__intro-avatar"
					: "thread-page__head-avatar hawkr-agent-thread__head-avatar"
			}
		>
			<HawkrAgentMark label="Hawkr Agent，AI 找貨協作" />
		</span>
	);
}

function CandidateCards({ candidates }: { candidates: SellerCandidate[] }) {
	return (
		<>
			{candidates.map((candidate, index) => {
				const website = safeSellerWebsiteUrl(candidate.publicWebsite);
				const title = candidate.brandName.trim() || `候選業者 ${index + 1}`;
				const metadata = [candidate.category, candidate.region]
					.map((value) => value.trim())
					.filter(Boolean)
					.join(" · ");
				const evidence = [
					candidate.productSummary,
					candidate.positioning,
					candidate.researchSummary,
				]
					.map((value) => value.trim())
					.filter(
						(value, itemIndex, values) =>
							Boolean(value) && values.indexOf(value) === itemIndex,
					);

				return (
					<li
						className="thread-page__bubble thread-page__bubble--card"
						data-candidate-artifact
						key={candidate.id || `${title}-${index}`}
					>
						<article
							className="chat-card hawkr-agent-candidate"
							aria-label={`${title}，公開開發名單，待驗證`}
						>
							<div className="chat-card__body">
								<span
									className="chat-card__media hawkr-agent-candidate__initial"
									aria-hidden="true"
								>
									{title.slice(0, 1)}
								</span>
								<div className="chat-card__info">
									<p className="chat-card__title">{title}</p>
									<p className="hawkr-agent-candidate__meta">
										{metadata || "類別與地區待確認"}
									</p>
									<span className="chat-card__status">開發名單 · 待驗證</span>
								</div>
							</div>
							<div className="hawkr-agent-candidate__evidence">
								{evidence.length > 0 ? (
									evidence.slice(0, 2).map((item) => <p key={item}>{item}</p>)
								) : (
									<p>來源沒有提供更多比對依據，仍需人工確認。</p>
								)}
							</div>
							{website ? (
								<div className="chat-card__actions">
									<a
										className="chat-card__btn chat-card__btn--solid hawkr-agent-candidate__link"
										href={website}
										target="_blank"
										rel="noreferrer noopener"
									>
										<span>{websiteLabel(website)}</span>
										<span className="sr-only">（另開新分頁）</span>
										<AppIcon name="arrow-right" size={15} aria-hidden="true" />
									</a>
								</div>
							) : null}
						</article>
					</li>
				);
			})}
		</>
	);
}

export default function HawkrAgentChatWorkspace() {
	const router = useObservedRouter("hawkr_agent_thread");
	useKeyboardInset();
	const [turns, setTurns] = React.useState<HawkrAgentTurn[]>([]);
	const [input, setInput] = React.useState("");
	const [sessionId, setSessionId] = React.useState<string | null>(null);
	const [streaming, setStreaming] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	const [lastFailedQuery, setLastFailedQuery] = React.useState<string | null>(
		null,
	);
	const [handoffSource, setHandoffSource] =
		React.useState<HawkrAgentHandoffSource | null>(null);
	const textareaRef = React.useRef<HTMLTextAreaElement | null>(null);
	const transcriptRef = React.useRef<HTMLDivElement | null>(null);
	const abortRef = React.useRef<AbortController | null>(null);
	const stoppedRef = React.useRef(false);
	const requestInFlightRef = React.useRef(false);
	const consumedHandoffRef = React.useRef(false);
	const stickToBottomRef = React.useRef(true);
	const lastTurn = turns.at(-1);
	const transcriptContentVersion = `${turns.length}:${lastTurn?.text.length ?? 0}:${lastTurn?.candidates?.length ?? 0}:${error ?? ""}:${handoffSource ?? ""}`;

	React.useEffect(() => {
		if (consumedHandoffRef.current) return;
		consumedHandoffRef.current = true;
		const handoff = consumeHawkrAgentHandoff();
		if (!handoff) return;
		setInput(handoff.query);
		setHandoffSource(handoff.source);
		requestAnimationFrame(() => textareaRef.current?.focus());
	}, []);

	React.useLayoutEffect(() => {
		if (!transcriptContentVersion || !stickToBottomRef.current) return;
		const transcript = transcriptRef.current;
		if (!transcript) return;
		transcript.scrollTop = transcript.scrollHeight;
	}, [transcriptContentVersion]);

	React.useEffect(
		() => () => {
			abortRef.current?.abort();
		},
		[],
	);

	const latestCandidateTurn = React.useMemo(
		() =>
			[...turns]
				.reverse()
				.find((turn) => turn.role === "agent" && turn.candidateResultReceived),
		[turns],
	);
	const latestCandidates = latestCandidateTurn?.candidates ?? [];
	const latestCandidateTurnId = latestCandidateTurn?.id;
	const latestAgentTurn = React.useMemo(
		() => [...turns].reverse().find((turn) => turn.role === "agent"),
		[turns],
	);

	React.useLayoutEffect(() => {
		if (!latestCandidateTurnId) return;
		const frame = requestAnimationFrame(() => {
			const transcript = transcriptRef.current;
			const artifacts = transcript?.querySelectorAll<HTMLElement>(
				"[data-candidate-artifact]",
			);
			const artifact = artifacts?.item((artifacts.length ?? 0) - 1);
			if (!transcript || !artifact) return;
			stickToBottomRef.current = false;
			const transcriptBox = transcript.getBoundingClientRect();
			const artifactBox = artifact.getBoundingClientRect();
			transcript.scrollTo({
				top: Math.max(
					0,
					transcript.scrollTop + artifactBox.top - transcriptBox.top - 12,
				),
				behavior: "auto",
			});
		});
		return () => cancelAnimationFrame(frame);
	}, [latestCandidateTurnId]);

	const send = React.useCallback(
		async (override?: string) => {
			const message = (override ?? input).trim().slice(0, 500);
			if (!message || streaming || requestInFlightRef.current) return;
			requestInFlightRef.current = true;

			const buyerId = createTurnId("buyer");
			const agentId = createTurnId("agent");
			const createdAt = Date.now();
			const controller = new AbortController();
			let receivedDone = false;
			let reportedError: string | null = null;

			abortRef.current = controller;
			stoppedRef.current = false;
			stickToBottomRef.current = true;
			setStreaming(true);
			setError(null);
			setLastFailedQuery(null);
			setHandoffSource(null);
			setInput("");
			setTurns((current) => [
				...current,
				{
					id: buyerId,
					role: "buyer",
					text: message,
					status: "complete",
					createdAt,
				},
				{
					id: agentId,
					role: "agent",
					text: "",
					status: "streaming",
					createdAt,
				},
			]);

			try {
				await streamSellerMatchChat(
					{ message, sessionId, signal: controller.signal },
					{
						onText: (chunk) => {
							setTurns((current) =>
								updateTurn(current, agentId, (turn) => ({
									...turn,
									text: `${turn.text}${chunk}`,
								})),
							);
						},
						onCandidates: (candidates) => {
							setTurns((current) =>
								updateTurn(current, agentId, (turn) => ({
									...turn,
									candidates,
									candidateResultReceived: true,
								})),
							);
						},
						onDone: (nextSessionId) => {
							receivedDone = true;
							if (nextSessionId) setSessionId(nextSessionId);
						},
						onError: (message) => {
							reportedError = message;
						},
					},
				);

				if (stoppedRef.current) return;
				if (reportedError) throw new Error(reportedError);
				if (!receivedDone) {
					throw new Error("連線提前中斷，尚未完成這次找貨。請重新送出。");
				}
				setTurns((current) =>
					updateTurn(current, agentId, (turn) => ({
						...turn,
						status: "complete",
						text:
							turn.text ||
							(turn.candidateResultReceived &&
							(turn.candidates?.length ?? 0) > 0
								? "我先整理出幾個可能符合的公開候選，請從來源與條件開始核對。"
								: "這次沒有找到可顯示的候選。可以補充品項、地區、數量或用途再找一次。"),
					})),
				);
			} catch (caught) {
				if (stoppedRef.current || controller.signal.aborted) {
					setTurns((current) =>
						updateTurn(current, agentId, (turn) => ({
							...turn,
							status: "stopped",
							text: turn.text || "已停止這次找貨。",
						})),
					);
					setLastFailedQuery(message);
				} else {
					const messageText =
						caught instanceof AiStreamTimeoutError
							? "Hawkr Agent 等候回應逾時，這次沒有自動重送。"
							: caught instanceof Error && caught.message.trim()
								? caught.message
								: "目前無法完成找貨，請稍後再試。";
					setError(messageText);
					setLastFailedQuery(message);
					setTurns((current) =>
						updateTurn(current, agentId, (turn) => ({
							...turn,
							status: "error",
							text: turn.text || "這次找貨沒有完成。",
						})),
					);
				}
			} finally {
				if (abortRef.current === controller) abortRef.current = null;
				requestInFlightRef.current = false;
				setStreaming(false);
			}
		},
		[input, sessionId, streaming],
	);

	const stop = () => {
		stoppedRef.current = true;
		abortRef.current?.abort();
	};

	const handleQuickReply = (text: string) => {
		setInput(text);
		requestAnimationFrame(() => {
			const textarea = textareaRef.current;
			if (!textarea) return;
			textarea.focus();
			textarea.setSelectionRange(text.length, text.length);
		});
	};

	const statusText = streaming
		? "Hawkr Agent 正在搜尋可能符合的公開候選。"
		: error
			? `找貨未完成：${error}`
			: latestAgentTurn
				? `${latestAgentTurn.text || "這次找貨已結束。"}${
						latestCandidateTurn
							? latestCandidates.length > 0
								? ` 已整理 ${latestCandidates.length} 家可能候選。`
								: " 這次沒有找到可顯示的候選。"
							: ""
					}`
				: "準備好開始找貨。";
	const noticeText = handoffSource
		? `已從${HANDOFF_SOURCE_LABEL[handoffSource]}帶入，尚未送出；確認內容後按傳送。重新整理會清除這次對話。`
		: "公開候選需自行核對；Hawkr Agent 不會自動聯絡、詢價或下單。重新整理會清除這次對話。";

	return (
		<section
			className="thread-page hawkr-agent-thread"
			aria-label="Hawkr Agent 找貨對話"
			data-assistant-mode="live"
		>
			<header className="thread-page__head">
				<button
					type="button"
					className="thread-page__back"
					onClick={() => router.push("/account/messages")}
					aria-label="返回對話列表"
				>
					<AppIcon name="arrow-left" size={20} />
				</button>
				<AgentAvatar />
				<div className="thread-page__head-meta">
					<strong>Hawkr Agent</strong>
					<span className="thread-page__head-status" data-state="assistant">
						找貨協作
					</span>
				</div>
			</header>

			{error ? (
				<div
					className="thread-page__alert hawkr-agent-thread__alert"
					role="alert"
				>
					<span>{error}</span>
					{lastFailedQuery ? (
						<button type="button" onClick={() => void send(lastFailedQuery)}>
							重新送出
						</button>
					) : null}
				</div>
			) : null}

			<div
				ref={transcriptRef}
				className="thread-page__body"
				role="log"
				aria-label="Hawkr Agent 對話紀錄"
				aria-live="off"
				onScroll={(event) => {
					const target = event.currentTarget;
					stickToBottomRef.current =
						target.scrollHeight - target.scrollTop - target.clientHeight < 80;
				}}
			>
				{turns.length === 0 ? (
					<div className="thread-intro">
						<AgentAvatar intro />
						<h1 className="thread-intro__name">Hawkr Agent</h1>
						<p className="thread-intro__copy">
							說說你想找什麼，我會整理公開候選，再由你確認合作條件。
						</p>
						<Link
							href={ASSISTANT_PROCUREMENT_PATH}
							className="thread-intro__shop"
						>
							開啟完整採購流程
						</Link>
					</div>
				) : (
					<ul className="thread-page__list">
						<li className="thread-page__separator" role="presentation">
							<span>今天</span>
						</li>
						{turns.map((turn) => (
							<React.Fragment key={turn.id}>
								{turn.role === "agent" &&
								turn.status === "streaming" &&
								!turn.text ? (
									<li
										className="thread-page__typing"
										aria-label="Hawkr Agent 正在輸入"
									>
										<span /> <span /> <span /> Hawkr Agent 正在整理…
									</li>
								) : (
									<li
										className={[
											"thread-page__bubble",
											turn.role === "buyer" ? "is-mine" : "",
										]
											.filter(Boolean)
											.join(" ")}
									>
										<p>{turn.text}</p>
										<small>{formatTaipeiTime(turn.createdAt)}</small>
									</li>
								)}
								{turn.role === "agent" &&
								turn.candidateResultReceived &&
								(turn.candidates?.length ?? 0) > 0 ? (
									<CandidateCards candidates={turn.candidates ?? []} />
								) : null}
							</React.Fragment>
						))}
					</ul>
				)}
			</div>

			<div className="sr-only" role="status" aria-live="polite">
				{statusText}
			</div>

			<p className="thread-page__notice" role="status">
				{noticeText}
			</p>

			{turns.length === 0 && !handoffSource ? (
				<div className="thread-page__chips">
					{SUGGESTED_REQUESTS.map((suggestion) => (
						<button
							type="button"
							className="chat-empty-chip"
							key={suggestion}
							onClick={() => handleQuickReply(suggestion)}
						>
							{suggestion}
						</button>
					))}
				</div>
			) : null}

			<form
				className="thread-page__compose"
				onSubmit={(event) => {
					event.preventDefault();
					void send();
				}}
			>
				<div className="thread-page__field">
					<label htmlFor="hawkr-agent-message" className="sr-only">
						告訴 Hawkr Agent 想找什麼
					</label>
					<textarea
						ref={textareaRef}
						id="hawkr-agent-message"
						value={input}
						onChange={(event) => setInput(event.target.value.slice(0, 500))}
						onKeyDown={(event) => {
							if (
								event.key !== "Enter" ||
								event.shiftKey ||
								event.nativeEvent.isComposing
							)
								return;
							event.preventDefault();
							void send();
						}}
						placeholder="輸入訊息…"
						title="Shift + Enter 可換行"
						rows={1}
						maxLength={500}
					/>
					{streaming ? (
						<button
							type="button"
							className="thread-page__send hawkr-agent-thread__stop"
							onClick={stop}
							aria-label="停止回應"
						>
							<AppIcon name="close" size={20} />
						</button>
					) : (
						<SendButton
							type="submit"
							disabled={!input.trim()}
							ariaLabel="傳送"
						/>
					)}
				</div>
			</form>
		</section>
	);
}
