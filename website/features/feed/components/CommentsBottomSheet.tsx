"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import PostComments from "./PostComments";

interface Props {
	postId: number | string | null;
	open: boolean;
	onClose: () => void;
}

const DISMISS_RATIO = 0.25;
const DISMISS_VELOCITY = 0.6;

/**
 * Mobile bottom sheet for feed comments — mirrors IG's reels comment UX.
 * The pager surface stays mounted underneath; the sheet's full-bleed
 * scrim swallows pointer events so the pager doesn't react to drags
 * happening inside the sheet.
 */
export default function CommentsBottomSheet({ postId, open, onClose }: Props) {
	const [mounted, setMounted] = React.useState(false);
	const [dragY, setDragY] = React.useState(0);
	const [dragging, setDragging] = React.useState(false);
	const panelRef = React.useRef<HTMLDivElement>(null);
	const gestureRef = React.useRef<{
		pointerId: number;
		startY: number;
		startTime: number;
		height: number;
	} | null>(null);

	React.useEffect(() => {
		setMounted(true);
	}, []);

	React.useEffect(() => {
		if (!open) return;
		setDragY(0);
		setDragging(false);
	}, [open]);

	const onPointerDown = (e: React.PointerEvent<HTMLButtonElement>) => {
		if (e.button !== 0 && e.button !== undefined) return;
		const height = panelRef.current?.clientHeight ?? 1;
		gestureRef.current = {
			pointerId: e.pointerId,
			startY: e.clientY,
			startTime: performance.now(),
			height,
		};
		setDragging(true);
		try {
			e.currentTarget.setPointerCapture(e.pointerId);
		} catch {
			// older WebKit: ignore
		}
	};

	const onPointerMove = (e: React.PointerEvent<HTMLButtonElement>) => {
		const g = gestureRef.current;
		if (!g || e.pointerId !== g.pointerId) return;
		// Only allow drag DOWN; ignore drag-up (no expand-beyond-sheet).
		setDragY(Math.max(0, e.clientY - g.startY));
	};

	const onPointerEnd = (e: React.PointerEvent<HTMLButtonElement>) => {
		const g = gestureRef.current;
		if (!g || e.pointerId !== g.pointerId) return;
		gestureRef.current = null;
		setDragging(false);
		try {
			if (e.currentTarget.hasPointerCapture(e.pointerId)) {
				e.currentTarget.releasePointerCapture(e.pointerId);
			}
		} catch {
			// noop
		}
		const dy = Math.max(0, e.clientY - g.startY);
		const dt = Math.max(performance.now() - g.startTime, 1);
		const velocity = dy / dt;
		if (dy > g.height * DISMISS_RATIO || velocity > DISMISS_VELOCITY) {
			onClose();
		} else {
			setDragY(0);
		}
	};

	if (!open || !mounted || postId === null) return null;

	const panelStyle: React.CSSProperties = {
		transform: dragY > 0 ? `translateY(${dragY}px)` : undefined,
		transition: dragging ? "none" : undefined,
	};

	const node = (
		<div
			className="feed-comments-sheet"
			role="dialog"
			aria-modal="true"
			aria-label="留言"
		>
			<button
				type="button"
				className="feed-comments-sheet__scrim"
				aria-label="關閉留言"
				onClick={onClose}
			/>
			<div
				className="feed-comments-sheet__panel"
				ref={panelRef}
				style={panelStyle}
			>
				<button
					type="button"
					className="feed-comments-sheet__handle"
					aria-label="拖曳或點擊以關閉留言"
					onClick={onClose}
					onPointerDown={onPointerDown}
					onPointerMove={onPointerMove}
					onPointerUp={onPointerEnd}
					onPointerCancel={onPointerEnd}
				>
					<span className="feed-comments-sheet__grabber" aria-hidden="true" />
				</button>
				<div className="feed-comments-sheet__body">
					<PostComments postId={postId} />
				</div>
			</div>
		</div>
	);

	return createPortal(node, document.body);
}
