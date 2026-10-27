"use client";

import "@/styles/components/chat-drawer-shell.css";
import * as React from "react";
import {
	type ChatDrawerThreadProps,
	type ChatDrawerThreadResource,
	chatDrawerThreadResource,
} from "@/features/messages/components/ChatDrawerThreadLoader";

type LoadState =
	| { status: "loading" }
	| { status: "ready"; Component: React.ComponentType<ChatDrawerThreadProps> }
	| { status: "error" };

interface ChatDrawerThreadBoundaryProps extends ChatDrawerThreadProps {
	resource: ChatDrawerThreadResource;
}

/**
 * The boundary is exported for deterministic loader tests. Production callers
 * use the default wrapper below, which binds the singleton module resource.
 */
export function ChatDrawerThreadBoundary({
	resource,
	...threadProps
}: ChatDrawerThreadBoundaryProps) {
	const [attempt, setAttempt] = React.useState(0);
	const [state, setState] = React.useState<LoadState>(() => {
		const Component = resource.peek();
		return Component ? { status: "ready", Component } : { status: "loading" };
	});

	React.useEffect(() => {
		void attempt;
		const cached = resource.peek();
		if (cached) {
			setState((current) =>
				current.status === "ready" && current.Component === cached
					? current
					: { status: "ready", Component: cached },
			);
			return;
		}

		let active = true;
		setState({ status: "loading" });
		void resource.load().then(
			(Component) => {
				if (active) setState({ status: "ready", Component });
			},
			() => {
				if (active) setState({ status: "error" });
			},
		);
		return () => {
			active = false;
		};
	}, [attempt, resource]);

	if (state.status === "ready") {
		return <state.Component {...threadProps} embedded />;
	}
	if (state.status === "error") {
		return (
			<ChatDrawerLoadError
				onRetry={(retryButton) => {
					if (document.activeElement === retryButton) {
						retryButton
							.closest<HTMLElement>('[role="dialog"]')
							?.focus({ preventScroll: true });
					}
					setState({ status: "loading" });
					setAttempt((current) => current + 1);
				}}
			/>
		);
	}
	return <ChatDrawerLoading />;
}

function ChatDrawerLoading() {
	return (
		<section
			className="thread-page thread-page--embedded"
			aria-label="對話內容"
			aria-busy="true"
		>
			<span className="sr-only" role="status" aria-live="polite">
				載入聊天室中…
			</span>
			<header className="thread-page__head">
				<span className="size-11 [@media(max-width:959px)]:size-[38px] shrink-0 animate-pulse rounded-full bg-[var(--hkui-default-100)] motion-reduce:animate-none" />
				<div className="thread-page__head-meta" aria-hidden="true">
					<strong className="flex h-[1lh] items-center">
						<span className="h-[1em] w-36 max-w-[55%] animate-pulse rounded bg-[var(--hkui-default-100)] motion-reduce:animate-none" />
					</strong>
					<span className="thread-page__head-status h-[1lh] before:invisible">
						<span className="h-[1em] w-16 animate-pulse rounded bg-[var(--hkui-default-100)] motion-reduce:animate-none" />
					</span>
				</div>
			</header>
			<div className="thread-page__body" aria-hidden="true">
				<span className="mt-auto h-11 w-[42%] animate-pulse rounded-[var(--hkui-radius-lg)] bg-[var(--hkui-default-100)] motion-reduce:animate-none" />
				<span className="h-16 w-[58%] self-end animate-pulse rounded-[var(--hkui-radius-lg)] bg-[var(--hkui-default-100)] motion-reduce:animate-none" />
				<span className="h-10 w-[34%] animate-pulse rounded-[var(--hkui-radius-lg)] bg-[var(--hkui-default-100)] motion-reduce:animate-none" />
			</div>
			<div className="thread-page__compose" aria-hidden="true">
				<span className="h-14 flex-1 animate-pulse rounded-[var(--hkui-radius-2xl)] bg-[var(--hkui-default-100)] motion-reduce:animate-none" />
			</div>
		</section>
	);
}

function ChatDrawerLoadError({
	onRetry,
}: {
	onRetry: (retryButton: HTMLButtonElement) => void;
}) {
	return (
		<section
			className="thread-page thread-page--embedded"
			aria-label="對話內容"
		>
			<header className="thread-page__head">
				<div className="thread-page__head-meta">
					<strong>聊天室</strong>
					<span className="thread-page__head-status" data-state="offline">
						暫時無法載入
					</span>
				</div>
			</header>
			<div className="thread-page__body items-center justify-center text-center">
				<div className="max-w-sm">
					<div role="alert">
						<h2 className="m-0 text-[length:var(--hkui-display-sm)] font-semibold text-[var(--hkui-foreground)] text-balance">
							聊天室暫時無法載入
						</h2>
						<p className="mb-0 mt-2 text-[length:var(--hkui-text-small)] leading-[var(--hkui-line-small)] text-[var(--hkui-foreground-muted)] text-pretty">
							可能是網路暫時不穩，請重新載入聊天室。
						</p>
					</div>
					<button
						type="button"
						className="mt-5 inline-flex min-h-11 items-center justify-center rounded-[var(--hkui-radius-md)] bg-[var(--hkui-primary)] px-5 text-[length:var(--hkui-text-small)] font-semibold text-[var(--hkui-primary-foreground)] transition-colors hover:bg-[var(--hkui-primary-600)] focus-visible:!outline-none focus-visible:![box-shadow:var(--hkui-focus-ring)]"
						onClick={(event) => onRetry(event.currentTarget)}
					>
						重新載入
					</button>
				</div>
			</div>
			<div className="thread-page__compose" aria-hidden="true">
				<span className="h-11 flex-1 rounded-[var(--radius-pill)] bg-[var(--hkui-default-100)]" />
			</div>
		</section>
	);
}

export default function ChatDrawerThread(props: ChatDrawerThreadProps) {
	return (
		<ChatDrawerThreadBoundary {...props} resource={chatDrawerThreadResource} />
	);
}
