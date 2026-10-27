"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import {
	type AiDraftPatch,
	streamAiListingChat,
} from "@/lib/seller/ai-listing-chat";

/** 完成度不足，暫時對 user 隱藏入口；功能補完後改回 true。 */
export const AI_LISTING_ASSISTANT_ENABLED = false;

interface Props {
	open: boolean;
	onClose: () => void;
	/**
	 * Called every time the AI sends a draft_patch event. Parent merges
	 * scalar fields and replaces array fields, mirroring the mobile draft
	 * controller's `mergePatch` semantics.
	 */
	onPatch: (patch: AiDraftPatch) => void;
}

interface Message {
	id: string;
	role: "user" | "assistant" | "system";
	text: string;
	patches?: number; // count of patches received in this turn
}

/**
 * AI 上架小助手 — slide-out drawer that streams chat over SSE and pipes
 * draft_patch events back into the product form. UI intentionally
 * minimal (web v1) compared with mobile's bottom-sheet:
 *   - Drawer pinned to right edge, full height
 *   - Conversation transcript (user / assistant bubbles)
 *   - Single text input + Enter to send
 *   - Session continuation via `session_id` returned in `done`
 *
 * The drawer keeps the form fully visible to the left so the seller can
 * watch fields populate as the AI replies.
 */
export default function AiAssistantPanel({ open, onClose, onPatch }: Props) {
	const [messages, setMessages] = React.useState<Message[]>([]);
	const [input, setInput] = React.useState("");
	const [streaming, setStreaming] = React.useState(false);
	const [sessionId, setSessionId] = React.useState<string | null>(null);
	const transcriptRef = React.useRef<HTMLDivElement>(null);
	const abortRef = React.useRef<AbortController | null>(null);

	// Auto-scroll the transcript on new content.
	React.useEffect(() => {
		const el = transcriptRef.current;
		if (el) el.scrollTop = el.scrollHeight;
	}, []);

	// Cancel an in-flight stream when the drawer closes.
	React.useEffect(() => {
		if (!open && abortRef.current) {
			abortRef.current.abort();
			abortRef.current = null;
			setStreaming(false);
		}
	}, [open]);

	const send = async () => {
		const text = input.trim();
		if (!text || streaming) return;

		const userMsg: Message = {
			id: `u-${Date.now()}`,
			role: "user",
			text,
		};
		const assistantId = `a-${Date.now()}`;
		const assistantStub: Message = {
			id: assistantId,
			role: "assistant",
			text: "",
			patches: 0,
		};
		setMessages((prev) => [...prev, userMsg, assistantStub]);
		setInput("");
		setStreaming(true);

		const ctrl = new AbortController();
		abortRef.current = ctrl;

		try {
			await streamAiListingChat(
				{ message: text, sessionId, signal: ctrl.signal },
				{
					onText: (chunk) => {
						setMessages((prev) =>
							prev.map((m) =>
								m.id === assistantId ? { ...m, text: m.text + chunk } : m,
							),
						);
					},
					onDraftPatch: (patch) => {
						onPatch(patch);
						setMessages((prev) =>
							prev.map((m) =>
								m.id === assistantId
									? { ...m, patches: (m.patches ?? 0) + 1 }
									: m,
							),
						);
					},
					onDone: (sid) => {
						if (sid) setSessionId(sid);
					},
					onError: (msg) => {
						setMessages((prev) => [
							...prev,
							{ id: `e-${Date.now()}`, role: "system", text: msg },
						]);
					},
				},
			);
		} catch (e) {
			if ((e as Error)?.name !== "AbortError") {
				setMessages((prev) => [
					...prev,
					{
						id: `e-${Date.now()}`,
						role: "system",
						text: e instanceof Error ? e.message : "AI 對話失敗",
					},
				]);
			}
		} finally {
			abortRef.current = null;
			setStreaming(false);
		}
	};

	const reset = () => {
		setMessages([]);
		setSessionId(null);
		setInput("");
	};

	if (!open) return null;

	return (
		<>
			<button
				type="button"
				className="ai-panel__backdrop"
				aria-label="關閉 AI 助手"
				onClick={onClose}
			/>
			<aside className="ai-panel" role="dialog" aria-label="AI 上架助手">
				<header className="ai-panel__head">
					<div>
						<strong>AI 上架小助手</strong>
						<small>幫你補齊商品名、產地、描述、分類、規格</small>
					</div>
					<div className="ai-panel__head-actions">
						{messages.length > 0 && (
							<Button
								variant="bordered"
								color="default"
								size="sm"
								onClick={reset}
								disabled={streaming}
							>
								清空對話
							</Button>
						)}
						<button
							type="button"
							className="ai-panel__close"
							onClick={onClose}
							aria-label="關閉"
						>
							<AppIcon name="close" size={20} />
						</button>
					</div>
				</header>

				<div className="ai-panel__transcript" ref={transcriptRef}>
					{messages.length === 0 ? (
						<div className="ai-panel__empty">
							<p>
								例：「我要上架埔里溫室小番茄，600g 一盒，280 元，限量 50 盒」
							</p>
							<p>例：「幫我寫一個有機青花菜的商品描述，強調冷鏈宅配」</p>
						</div>
					) : (
						messages.map((m) => (
							<div
								key={m.id}
								className={`ai-panel__msg ai-panel__msg--${m.role}`}
							>
								<div className="ai-panel__bubble">
									{m.role === "system" ? (
										<AppIcon
											name="warning"
											size={14}
											className="inline-block align-[-0.15em] mr-1"
										/>
									) : null}
									{m.text || (m.role === "assistant" ? "…" : "")}
								</div>
								{m.role === "assistant" && (m.patches ?? 0) > 0 && (
									<small className="ai-panel__patches">
										<AppIcon name="check" size={13} fill />
										已自動填入 {m.patches} 項表單欄位
									</small>
								)}
							</div>
						))
					)}
				</div>

				<form
					className="ai-panel__compose"
					onSubmit={(e) => {
						e.preventDefault();
						void send();
					}}
				>
					<textarea
						value={input}
						onChange={(e) => setInput(e.target.value)}
						placeholder="跟 AI 描述你的商品…（Shift+Enter 換行）"
						rows={2}
						onKeyDown={(e) => {
							if (e.key === "Enter" && !e.shiftKey) {
								e.preventDefault();
								void send();
							}
						}}
						disabled={streaming}
					/>
					<Button type="submit" disabled={!input.trim() || streaming}>
						{streaming ? "回覆中…" : "送出"}
					</Button>
				</form>
			</aside>
		</>
	);
}
