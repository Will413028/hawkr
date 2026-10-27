import type {
	CategoryResolutionData,
	PricebookSelection,
} from "@/lib/pricebook/repo";

export type CategoryResolutionOutcome =
	| {
			kind: "resolved";
			selection: PricebookSelection;
			label: string;
			signature: string;
	  }
	| { kind: "unmapped" }
	| { kind: "ambiguous" }
	| { kind: "invalid" };

/** Derive the UI-facing outcome from the backend's category-resolution
 *  response. `resolved` without a usable `groupCode` is still a contract
 *  violation, not a value to guess from — it fails closed to `invalid` the
 *  same way an unmapped/ambiguous status does, instead of ever applying a
 *  partial selection. */
export function resolveCategoryResolutionOutcome(
	data: CategoryResolutionData | null,
): CategoryResolutionOutcome {
	if (!data) return { kind: "unmapped" };
	if (data.status === "unmapped") return { kind: "unmapped" };
	if (data.status === "ambiguous") return { kind: "ambiguous" };

	if (data.groupCode) {
		const varieties = data.cropVarieties ?? [];
		if (varieties.length === 1) {
			const variety = varieties[0];
			return {
				kind: "resolved",
				selection: { kind: "crop", cropCode: variety.cropCode },
				label: variety.variantName?.trim() || variety.name,
				signature: `crop:${variety.cropCode}`,
			};
		}
		return {
			kind: "resolved",
			selection: { kind: "group", groupCode: data.groupCode },
			label: data.label ?? data.groupCode,
			signature: `group:${data.groupCode}`,
		};
	}
	return { kind: "invalid" };
}
