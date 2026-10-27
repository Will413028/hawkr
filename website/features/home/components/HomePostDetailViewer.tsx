"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import AppIcon from "@/components/shared/AppIcon";
import BrandLogoLoader from "@/components/shared/BrandLogoLoader";
import {
	type HomePostDetailViewerProps,
	type HomePostDetailViewerResource,
	homePostDetailViewerResource,
} from "@/features/home/components/HomePostDetailViewerLoader";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";

type LoadState =
	| { status: "loading" }
	| {
			status: "ready";
			Component: React.ComponentType<HomePostDetailViewerProps>;
	  }
	| { status: "error" };

interface HomePostDetailViewerBoundaryProps extends HomePostDetailViewerProps {
	resource: HomePostDetailViewerResource;
	returnFocusRef: React.RefObject<HTMLElement | null>;
}

const FOCUSABLE_SELECTOR =
	'button:not([disabled]), a[href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

function focusWithoutScrolling(target: HTMLElement | null): void {
	if (target?.isConnected) target.focus({ preventScroll: true });
}

/**
 * Owns one open session from the lightweight status shell through the real
 * viewer. Keeping focus restoration and scroll locking outside the imported
 * module prevents a cold-load handoff from forgetting the original card.
 */
export function HomePostDetailViewerBoundary({
	resource,
	returnFocusRef,
	...viewerProps
}: HomePostDetailViewerBoundaryProps) {
	const [attempt, setAttempt] = React.useState(0);
	const [returnFocusTarget] = React.useState<HTMLElement | null>(
		() =>
			returnFocusRef.current ??
			(typeof document !== "undefined" &&
			document.activeElement instanceof HTMLElement
				? document.activeElement
				: null),
	);
	const [state, setState] = React.useState<LoadState>(() => {
		const Component = resource.peek();
		return Component ? { status: "ready", Component } : { status: "loading" };
	});

	const restoreOpener = React.useCallback(() => {
		focusWithoutScrolling(returnFocusTarget);
	}, [returnFocusTarget]);

	React.useLayoutEffect(() => {
		// Read the offset before overflow:hidden can clamp the document to 0.
		// A warm viewer takes its own passive-effect lock, so this session must
		// pin the original offset first on both the cold and cached paths.
		const releaseScrollLock = acquireBodyScrollLock({
			preserveScrollPosition: true,
			scrollY: window.scrollY,
		});
		return () => {
			releaseScrollLock();
			restoreOpener();
		};
	}, [restoreOpener]);

	React.useEffect(() => {
		void attempt;
		const cached = resource.peek();
		if (cached) {
			// The imported viewer captures document.activeElement for its own
			// close transition. Put the original card back in focus first, rather
			// than letting it remember the temporary loading-shell close button.
			restoreOpener();
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
				if (!active) return;
				restoreOpener();
				setState({ status: "ready", Component });
			},
			() => {
				if (active) setState({ status: "error" });
			},
		);
		return () => {
			active = false;
		};
	}, [attempt, resource, restoreOpener]);

	if (state.status === "ready") {
		return <state.Component {...viewerProps} />;
	}

	return (
		<HomePostDetailStatusDialog
			state={state.status}
			onClose={viewerProps.onClose}
			onRetry={(retryButton, dialog) => {
				if (document.activeElement === retryButton) {
					dialog.focus({ preventScroll: true });
				}
				setState({ status: "loading" });
				setAttempt((current) => current + 1);
			}}
		/>
	);
}

function HomePostDetailStatusDialog({
	state,
	onClose,
	onRetry,
}: {
	state: "loading" | "error";
	onClose: () => void;
	onRetry: (retryButton: HTMLButtonElement, dialog: HTMLDivElement) => void;
}) {
	const dialogRef = React.useRef<HTMLDivElement>(null);
	const closeButtonRef = React.useRef<HTMLButtonElement>(null);

	React.useEffect(() => {
		const dialog = dialogRef.current;
		if (!dialog) return;
		if (!dialog.contains(document.activeElement)) {
			(closeButtonRef.current ?? dialog).focus({ preventScroll: true });
		}

		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key === "Escape") {
				event.preventDefault();
				onClose();
				return;
			}
			if (event.key !== "Tab") return;

			const focusable = Array.from(
				dialog.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR),
			);
			if (focusable.length === 0) {
				event.preventDefault();
				dialog.focus({ preventScroll: true });
				return;
			}
			const first = focusable[0];
			const last = focusable[focusable.length - 1];
			if (
				!dialog.contains(document.activeElement) ||
				(event.shiftKey && document.activeElement === first) ||
				(!event.shiftKey && document.activeElement === last)
			) {
				event.preventDefault();
				(event.shiftKey ? last : first).focus({ preventScroll: true });
			}
		};

		document.addEventListener("keydown", onKeyDown);
		return () => document.removeEventListener("keydown", onKeyDown);
	}, [onClose]);

	if (typeof document === "undefined") return null;

	return createPortal(
		<div
			ref={dialogRef}
			className="post-viewer grid min-h-dvh place-items-center p-6"
			role="dialog"
			aria-modal="true"
			aria-label="貼文詳情"
			aria-busy={state === "loading" || undefined}
			tabIndex={-1}
			data-home-post-detail-state={state}
		>
			<button
				ref={closeButtonRef}
				type="button"
				className="post-viewer__close"
				onClick={onClose}
				aria-label="關閉"
			>
				<AppIcon name="close" size={22} />
			</button>

			<div className="w-full max-w-sm rounded-[24px] border border-hk-border-soft bg-hk-surface-card p-7 text-center shadow-hk-card">
				{state === "loading" ? (
					<>
						<BrandLogoLoader size={48} className="mx-auto" />
						<p
							className="mb-0 mt-4 text-sm font-bold text-hk-muted"
							role="status"
							aria-live="polite"
						>
							正在打開貼文…
						</p>
					</>
				) : (
					<>
						<div role="alert">
							<h2 className="m-0 text-xl font-black text-hk-heading">
								貼文暫時無法載入
							</h2>
							<p className="mb-0 mt-2 text-sm leading-relaxed text-hk-muted">
								可能是網路暫時不穩，請再試一次。
							</p>
						</div>
						<button
							type="button"
							data-home-post-detail-retry
							className="mt-5 min-h-11 rounded-full bg-hk-primary px-5 text-sm font-black text-hk-inverse transition-opacity hover:opacity-90"
							onClick={(event) => {
								const dialog = dialogRef.current;
								if (dialog) onRetry(event.currentTarget, dialog);
							}}
						>
							重新載入
						</button>
					</>
				)}
			</div>
		</div>,
		document.body,
	);
}

export default function HomePostDetailViewer(
	props: HomePostDetailViewerProps & {
		returnFocusRef: React.RefObject<HTMLElement | null>;
	},
) {
	return (
		<HomePostDetailViewerBoundary
			{...props}
			resource={homePostDetailViewerResource}
		/>
	);
}
