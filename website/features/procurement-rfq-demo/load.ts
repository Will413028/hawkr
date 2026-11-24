import { commonStoreCardClosedStore } from "@hawkr/ui-text/keys";
import { candidateFromProto } from "@/features/procurement-assistant/proto-map";
import { pricePreviewFromProto } from "@/features/procurement-assistant/rfq-price-contract";
import { fetchProcurementSnapshot } from "@/features/procurement-assistant/snapshot";
import type { ProcurementSnapshot } from "@/features/procurement-assistant/types";
import { buyerProcurementClient } from "@/lib/connect/procurement";
import { uiTextFor } from "@/lib/i18n/ui-text";
import { activeLocale } from "@/lib/intl/active-locale";
import type {
	DemoCandidate,
	DemoContext,
	DemoQuoteTotalRange,
	DemoRequirement,
} from "./types";

export class DemoRequirementIncompleteError extends Error {
	readonly missing: string[];
	constructor(missing: string[]) {
		super(`demo requirement incomplete: ${missing.join(", ")}`);
		this.name = "DemoRequirementIncompleteError";
		this.missing = missing;
	}
}

function toRequirement(snapshot: ProcurementSnapshot): DemoRequirement {
	const slots = snapshot.slots;
	return {
		requestId: snapshot.requestId,
		revisionId: snapshot.revisionId,
		item: slots.item ?? null,
		quantity: slots.quantity ?? null,
		unit: slots.unit ?? null,
		packaging: slots.packaging ?? null,
		deliveryCity: slots.deliveryCity ?? null,
		deliveryDate: slots.deliveryDate ?? null,
		budgetPerUnit: slots.budgetPerUnit ?? null,
	};
}

function latestCandidateSearchRunId(
	snapshot: ProcurementSnapshot,
): string | null {
	const run = snapshot.runs.find(
		(entry) => entry.runType === "CANDIDATE_SEARCH",
	);
	return run?.runId || null;
}

async function fetchSelectedCandidates(
	requestId: string,
	runId: string,
): Promise<DemoCandidate[]> {
	const closedStoreName = uiTextFor(activeLocale())(
		commonStoreCardClosedStore(),
	);
	const res = await buyerProcurementClient().getCandidateSearch({
		requestId,
		runId,
	});
	return res.candidates
		.filter((candidate) => candidate.selected)
		.map((candidate) => {
			const view = candidateFromProto(candidate, closedStoreName);
			return {
				candidateId: view.candidateId,
				displayName: view.displayName,
				matchedFactCount: view.matchedFacts.length,
				unverifiableFactCount: view.unverifiableFacts.length,
			};
		});
}

export interface PricePreviewOutcome {
	quoteTotalRange: DemoQuoteTotalRange | null;
	/** 試算沒有成功的原因；成功時是 null。 */
	reason: string | null;
}

export async function runPricePreview(
	requirement: DemoRequirement,
	buyerRange: { lower: number; upper: number },
): Promise<PricePreviewOutcome> {
	if (!requirement.deliveryCity) {
		return { quoteTotalRange: null, reason: "MISSING_DELIVERY_CITY" };
	}
	try {
		const res = await buyerProcurementClient().previewRfqPrice({
			requestId: requirement.requestId,
			expectedRevisionId: requirement.revisionId,
			buyerRequestedRange: {
				lowerBoundAmount: BigInt(buyerRange.lower),
				upperBoundAmount: BigInt(buyerRange.upper),
			},
		});
		const preview = pricePreviewFromProto(res);
		if (!preview) return { quoteTotalRange: null, reason: "INVALID_RESPONSE" };
		const total = preview.quoteTotalRange;
		return {
			quoteTotalRange: total
				? {
						lower: Number(total.lowerBoundAmount),
						upper: Number(total.upperBoundAmount),
						quantity: total.quantity,
						quantityUnit: total.quantityUnit,
					}
				: null,
			reason: null,
		};
	} catch {
		return { quoteTotalRange: null, reason: "PREVIEW_REJECTED" };
	}
}

export async function loadDemoContext(requestId: string): Promise<DemoContext> {
	const snapshot = await fetchProcurementSnapshot(requestId);
	const requirement = toRequirement(snapshot);

	const missing: string[] = [];
	if (!requirement.item) missing.push("品項");
	if (requirement.quantity === null) missing.push("數量");
	if (!requirement.unit) missing.push("單位");
	if (missing.length > 0) throw new DemoRequirementIncompleteError(missing);

	const runId = latestCandidateSearchRunId(snapshot);
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

	return {
		requirement,
		candidates,
		candidatesUnavailableReason,
		quoteTotalRange: null,
	};
}
