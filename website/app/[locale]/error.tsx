"use client";

import ErrorState from "@/components/shared/ErrorState";

/**
 * Segment error boundary for the whole app (below the root layout, so the
 * navbar/footer chrome stays up). Catches production rethrows from server
 * fetches — e.g. the gateway dropping connections during a backend deploy
 * (backend-payuni#939) — and gives the buyer a way back instead of Next's
 * default error screen.
 */
export default function GlobalSegmentError({
	error,
	reset,
}: {
	error: Error & { digest?: string };
	reset: () => void;
}) {
	return <ErrorState error={error} reset={reset} />;
}
