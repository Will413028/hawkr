"use client";

import * as React from "react";
import {
	acquireChatStream,
	type ChatEventStream,
	type ChatStreamEvent,
} from "./stream";
import type { ChatSide, ConnectionState } from "./types";

/**
 * 訂閱某一方的共用聊天串流（全站一條，引用計數，見 `acquireChatStream`）。
 * `onEvent` 以最新的 callback 呼叫，不因 callback 換新而重新訂閱。
 */
export function useChatStream(
	side: ChatSide,
	enabled: boolean,
	onEvent?: (e: ChatStreamEvent, stream: ChatEventStream) => void,
): { state: ConnectionState; stream: ChatEventStream | null } {
	const [state, setState] = React.useState<ConnectionState>("disconnected");
	const [stream, setStream] = React.useState<ChatEventStream | null>(null);
	const handlerRef = React.useRef(onEvent);
	React.useEffect(() => {
		handlerRef.current = onEvent;
	});
	React.useEffect(() => {
		if (!enabled) {
			setState("disconnected");
			setStream(null);
			return;
		}
		const { stream: s, release } = acquireChatStream(side);
		setStream(s);
		setState(s.state);
		const offState = s.onState(setState);
		const offEvent = s.onEvent((e) => handlerRef.current?.(e, s));
		return () => {
			offEvent();
			offState();
			release();
		};
	}, [side, enabled]);
	return { state, stream };
}
