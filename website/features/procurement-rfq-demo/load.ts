import {
	priceBasisOf,
	type RfqConfirmationCommand,
	type RfqPricePreviewCommand,
	rfqConfirmationSchema,
	rfqPricePreviewSchema,
} from "@/features/procurement-assistant/rfq-price-contract";
import {
	normalizeRawProcurementSlots,
	type RawProcurementSlots,
} from "@/features/procurement-assistant/session-logic";
import { apiClient } from "@/lib/api/client";
import type { ProcurementCandidateSearch } from "@/lib/api/openapi-types";
import type {
	DemoCandidate,
	DemoContext,
	DemoMarketReference,
	DemoPreviewIdentity,
	DemoQuoteTotalRange,
	DemoRequirement,
} from "./types";

interface SnapshotResponse {
	requestId: string;
	revisionId: string;
	slots: RawProcurementSlots;
	runs?: unknown;
}

export class DemoRequirementIncompleteError extends Error {
	readonly missing: string[];
	constructor(missing: string[]) {
		super(`demo requirement incomplete: ${missing.join(", ")}`);
		this.name = "DemoRequirementIncompleteError";
		this.missing = missing;
	}
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function toRequirement(snapshot: SnapshotResponse): DemoRequirement {
	const slots = normalizeRawProcurementSlots(snapshot.slots) ?? {};
	return {
		requestId: snapshot.requestId,
		revisionId: snapshot.revisionId,
		item: slots.item ?? null,
		quantity: slots.quantity ?? null,
		unit: slots.unit ?? null,
		packaging: slots.packaging ?? null,
		deliveryCity: slots.delivery_city ?? null,
		deliveryDate: slots.delivery_date ?? null,
		budgetPerUnit: slots.budget_per_unit ?? null,
	};
}

function latestCandidateSearchRunId(runs: unknown): string | null {
	if (!Array.isArray(runs)) return null;
	for (const entry of runs) {
		if (!isRecord(entry)) continue;
		if (entry.runType !== "CANDIDATE_SEARCH") continue;
		const runId = entry.runId;
		if (typeof runId === "string" && runId !== "") return runId;
	}
	return null;
}

async function fetchSelectedCandidates(
	requestId: string,
	runId: string,
): Promise<DemoCandidate[]> {
	const env = await apiClient.authed.get<ProcurementCandidateSearch>(
		`/buyers/me/procurement/requests/${encodeURIComponent(requestId)}/candidate-searches/${encodeURIComponent(runId)}`,
	);
	const all = [
		...(env.data.candidates?.platform ?? []),
		...(env.data.candidates?.external ?? []),
	];
	return all
		.filter((candidate) => candidate.selected)
		.map((candidate) => ({
			candidateId: candidate.candidateId,
			displayName: candidate.displayName,
			lane: candidate.lane,
			sellerName: candidate.sellerName ?? null,
			matchedFactCount: candidate.matchedFacts?.length ?? 0,
			unverifiableFactCount: candidate.unverifiableFacts?.length ?? 0,
		}));
}

export interface PricePreviewOutcome {
	market: DemoMarketReference | null;
	quoteTotalRange: DemoQuoteTotalRange | null;
	preview: DemoPreviewIdentity | null;
	reason: string | null;
}

export async function runPricePreview(
	requirement: DemoRequirement,
	buyerRange?: { lower: number; upper: number },
): Promise<PricePreviewOutcome> {
	const empty: PricePreviewOutcome = {
		market: null,
		quoteTotalRange: null,
		preview: null,
		reason: null,
	};
	if (!requirement.deliveryCity) {
		return { ...empty, reason: "MISSING_DELIVERY_CITY" };
	}
	const command: RfqPricePreviewCommand = {
		expectedRevisionId: requirement.revisionId,
		clientRequestId: `rfq-demo-${requirement.revisionId}-${Date.now()}`,
		pricingMode: "PRICEBOOK_GUIDED",
		deliveryCity: requirement.deliveryCity,
		...(buyerRange
			? {
					buyerRequestedRange: {
						currency: "TWD" as const,
						priceComponent: "GOODS_ONLY" as const,
						lowerBoundAmount: String(buyerRange.lower),
						upperBoundAmount: String(buyerRange.upper),
					},
				}
			: {}),
	};
	try {
		const env = await apiClient.authed.post<unknown>(
			`/buyers/me/procurement/requests/${encodeURIComponent(requirement.requestId)}/rfq-price-previews`,
			{ json: command },
		);
		const parsed = rfqPricePreviewSchema.safeParse(env.data);
		if (!parsed.success) return { ...empty, reason: "INVALID_RESPONSE" };
		const preview = parsed.data;
		const identity: DemoPreviewIdentity = {
			pricePreviewId: preview.pricePreviewId,
			version: preview.version,
			payloadHash: preview.payloadHash,
		};
		const totals: DemoQuoteTotalRange | null = preview.quoteTotalRange
			? {
					lower: Number(preview.quoteTotalRange.lowerBoundAmount),
					upper: Number(preview.quoteTotalRange.upperBoundAmount),
					quantity: preview.quoteTotalRange.quantity,
					quantityUnit: preview.quoteTotalRange.quantityUnit,
				}
			: null;
		if (
			preview.referenceAvailability !== "AVAILABLE" ||
			!preview.referencePriceRange ||
			!preview.marketPriceBasis
		) {
			return {
				market: null,
				quoteTotalRange: totals,
				preview: identity,
				reason: preview.dealTemperature.reasonCode || "UNAVAILABLE",
			};
		}
		return {
			market: {
				lower: Number(preview.referencePriceRange.lowerBoundAmount),
				midpoint: Number(preview.referencePriceRange.midpointAmount),
				upper: Number(preview.referencePriceRange.upperBoundAmount),
				basisQuantity: priceBasisOf(preview).basisQuantity,
				basisUnit: priceBasisOf(preview).basisUnit,
				regionName: preview.marketPriceBasis.regionName,
				sourceDataAsOf: preview.marketPriceBasis.sourceDataAsOf,
				freshnessStatus: preview.marketPriceBasis.freshnessStatus,
			},
			quoteTotalRange: totals,
			preview: identity,
			reason: null,
		};
	} catch {
		return { ...empty, reason: "PREVIEW_REJECTED" };
	}
}

export async function confirmRfqSnapshot(
	requirement: DemoRequirement,
	preview: DemoPreviewIdentity,
): Promise<{ confirmedSnapshotId: string; payloadHash: string }> {
	const command: RfqConfirmationCommand = {
		clientRequestId: `rfq-demo-confirm-${preview.pricePreviewId}-${preview.version}`,
		expectedRevisionId: requirement.revisionId,
		pricePreviewId: preview.pricePreviewId,
		pricePreviewVersion: preview.version,
		payloadHash: preview.payloadHash,
		buyerNote: "",
	};
	const env = await apiClient.authed.post<unknown>(
		`/buyers/me/procurement/requests/${encodeURIComponent(requirement.requestId)}/rfq-confirmations`,
		{ json: command },
	);
	const parsed = rfqConfirmationSchema.safeParse(env.data);
	if (!parsed.success) throw new Error("confirmation response invalid");
	return {
		confirmedSnapshotId: parsed.data.confirmedRfqSnapshotId,
		payloadHash: parsed.data.payloadHash,
	};
}

export async function loadDemoContext(requestId: string): Promise<DemoContext> {
	const env = await apiClient.authed.get<SnapshotResponse>(
		`/buyers/me/procurement/requests/${encodeURIComponent(requestId)}/snapshot`,
	);
	const requirement = toRequirement(env.data);

	const missing: string[] = [];
	if (!requirement.item) missing.push("品項");
	if (requirement.quantity === null) missing.push("數量");
	if (!requirement.unit) missing.push("單位");
	if (missing.length > 0) throw new DemoRequirementIncompleteError(missing);

	const runId = latestCandidateSearchRunId(env.data.runs);
	let candidates: DemoCandidate[] = [];
	let candidatesUnavailableReason: string | null = null;
	if (!runId) {
		candidatesUnavailableReason = "NO_CANDIDATE_SEARCH_RUN";
	} else {
		try {
			candidates = await fetchSelectedCandidates(requirement.requestId, runId);
			if (candidates.length === 0) {
				candidatesUnavailableReason = "NO_SELECTED_CANDIDATE";
			}
		} catch {
			candidatesUnavailableReason = "CANDIDATE_FETCH_FAILED";
		}
	}

	const outcome = await runPricePreview(requirement);
	return {
		requirement,
		candidates,
		candidatesUnavailableReason,
		market: outcome.market,
		quoteTotalRange: outcome.quoteTotalRange,
		preview: outcome.preview,
		marketUnavailableReason: outcome.reason,
	};
}
