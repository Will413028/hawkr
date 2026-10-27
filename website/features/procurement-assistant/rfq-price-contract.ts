import { z } from "zod";

const MAX_SIGNED_INT64 = BigInt("9223372036854775807");
const POSITIVE_INT64_PATTERN = /^[1-9]\d{0,18}$/;

export const idSchema = z
	.string()
	.refine(
		(value) =>
			POSITIVE_INT64_PATTERN.test(value) && BigInt(value) <= MAX_SIGNED_INT64,
		"id must be a positive decimal string that fits in a signed int64",
	);
export const payloadHashSchema = z
	.string()
	.regex(/^[a-f0-9]{64}$/i, "payload hash must be 64 hexadecimal characters");
const buyerIntegerAmountSchema = z
	.string()
	.regex(/^[1-9]\d{0,11}$/, "amount must be a positive TWD integer");

function parseFixedAmount(value: string): bigint | null {
	const match = /^(0|[1-9]\d{0,11})(?:\.(\d{1,4}))?$/.exec(value);
	if (!match) return null;
	const scaled =
		BigInt(match[1]) * BigInt(10_000) + BigInt((match[2] ?? "").padEnd(4, "0"));
	return scaled > BigInt(0) ? scaled : null;
}

const serverAmountSchema = z
	.string()
	.refine(
		(value) => parseFixedAmount(value) !== null,
		"amount must be a positive canonical fixed-decimal TWD string",
	);

const BPS_INTEGER_PATTERN = /^(0|[1-9]\d{0,4})$/;

const serverBpsSchema = z
	.string()
	.refine(
		(value) => BPS_INTEGER_PATTERN.test(value) && Number(value) <= 10_000,
		"bps must be a canonical non-negative integer string no greater than 10000",
	);

function isAscending(values: string[]): boolean {
	const parsed = values.map(parseFixedAmount);
	if (parsed.some((value) => value === null)) return false;
	return values.every(
		(_value, index) =>
			index === 0 || (parsed[index - 1] as bigint) <= (parsed[index] as bigint),
	);
}

export const rfqPricingModeSchema = z.enum([
	"PRICEBOOK_GUIDED",
	"BUYER_ENTERED_NO_REFERENCE",
]);

export type RfqPricingMode = z.infer<typeof rfqPricingModeSchema>;

export const rfqBuyerRequestedRangeSchema = z
	.object({
		currency: z.literal("TWD"),
		priceComponent: z.literal("GOODS_ONLY"),
		lowerBoundAmount: serverAmountSchema,
		upperBoundAmount: serverAmountSchema,
		basisQuantity: z.string().max(64),
		basisUnit: z.string().max(32),
	})
	.superRefine((range, context) => {
		if (!isAscending([range.lowerBoundAmount, range.upperBoundAmount])) {
			context.addIssue({
				code: "custom",
				message: "buyer range must be ascending",
				path: ["upperBoundAmount"],
			});
		}
	});

export type RfqBuyerRequestedRange = z.infer<
	typeof rfqBuyerRequestedRangeSchema
>;

export const rfqMarketPriceBasisSchema = z.object({
	scope: z.string().max(16).default(""),
	priceLayer: z.string().max(32).default(""),
	regionName: z.string().max(64).default(""),
	supportingMarketCount: z
		.string()
		.regex(/^\d{1,9}$/, "supporting market count must be a decimal string"),
	sourceDataAsOf: z.string().min(1).nullable(),
	freshnessStatus: z.enum(["fresh", "stale", "unavailable"]).or(z.literal("")),
});

export type RfqMarketPriceBasis = z.infer<typeof rfqMarketPriceBasisSchema>;

export const rfqQuoteTotalRangeSchema = z
	.object({
		currency: z.literal("TWD"),
		lowerBoundAmount: serverAmountSchema,
		upperBoundAmount: serverAmountSchema,
		quantity: z.string().regex(/^[1-9]\d{0,18}$/, "quantity must be positive"),
		quantityUnit: z.string().min(1).max(32),
	})
	.superRefine((range, context) => {
		if (!isAscending([range.lowerBoundAmount, range.upperBoundAmount])) {
			context.addIssue({
				code: "custom",
				message: "quote total range must be ascending",
				path: ["upperBoundAmount"],
			});
		}
	});

export type RfqQuoteTotalRange = z.infer<typeof rfqQuoteTotalRangeSchema>;

export const rfqDealTemperatureSchema = z.discriminatedUnion("status", [
	z
		.object({
			status: z.literal("UNKNOWN"),
			reasonCode: z.string().max(128),
		})
		.strict(),
	z
		.object({
			status: z.literal("AVAILABLE"),
			reasonCode: z.string().max(128),
			probabilityLowerBps: serverBpsSchema,
			probabilityUpperBps: serverBpsSchema,
			band: z.enum(["COLD", "WARM", "HOT"]),
			outcomeDefinition: z.string().min(1).max(128),
			outcomeDefinitionVersion: z.string().min(1).max(16),
			policyVersion: z.string().min(1).max(128),
			bandVersion: z.string().min(1).max(128),
		})
		.strict(),
]);

export type RfqDealTemperature = z.infer<typeof rfqDealTemperatureSchema>;

export const rfqPricePreviewSchema = z
	.object({
		pricePreviewId: idSchema,
		version: idSchema,
		pricingMode: rfqPricingModeSchema,
		qualityKnowledgeState: z.string().default(""),
		referenceAvailability: z.enum(["AVAILABLE", "UNAVAILABLE"]),
		referencePriceRange: z
			.object({
				lowerBoundAmount: serverAmountSchema.or(z.literal("")),
				midpointAmount: serverAmountSchema,
				upperBoundAmount: serverAmountSchema.or(z.literal("")),
				basisQuantity: z.string().max(64).default(""),
				basisUnit: z.string().max(32).default(""),
			})
			.nullable(),
		buyerRequestedRange: rfqBuyerRequestedRangeSchema.nullable(),
		dealTemperature: rfqDealTemperatureSchema,
		payloadHash: payloadHashSchema,
		marketPriceBasis: rfqMarketPriceBasisSchema.nullable().default(null),
		quoteTotalRange: rfqQuoteTotalRangeSchema.nullable().default(null),
	})
	.superRefine((preview, context) => {
		const hasReference = preview.referencePriceRange !== null;
		if ((preview.referenceAvailability === "AVAILABLE") !== hasReference) {
			context.addIssue({
				code: "custom",
				message: "reference availability and range disagree",
				path: ["referencePriceRange"],
			});
		}
		if ((preview.marketPriceBasis !== null) !== hasReference) {
			context.addIssue({
				code: "custom",
				message: "market price basis must accompany a reference range",
				path: ["marketPriceBasis"],
			});
		}
		if (
			preview.pricingMode === "PRICEBOOK_GUIDED" &&
			preview.referenceAvailability !== "AVAILABLE"
		) {
			context.addIssue({
				code: "custom",
				message: "guided pricing requires an available reference",
				path: ["referenceAvailability"],
			});
		}
		if (
			preview.pricingMode === "BUYER_ENTERED_NO_REFERENCE" &&
			preview.referenceAvailability !== "UNAVAILABLE"
		) {
			context.addIssue({
				code: "custom",
				message: "manual pricing must not claim a Pricebook reference",
				path: ["referenceAvailability"],
			});
		}
		const manualRange =
			preview.pricingMode === "BUYER_ENTERED_NO_REFERENCE"
				? preview.buyerRequestedRange
				: null;
		if (
			preview.pricingMode === "BUYER_ENTERED_NO_REFERENCE" &&
			(manualRange === null ||
				!buyerIntegerAmountSchema.safeParse(manualRange.lowerBoundAmount)
					.success ||
				!buyerIntegerAmountSchema.safeParse(manualRange.upperBoundAmount)
					.success)
		) {
			context.addIssue({
				code: "custom",
				message: "manual pricing amounts must remain positive TWD integers",
				path: ["buyerRequestedRange"],
			});
		}
		const referenceHasBand = Boolean(
			preview.referencePriceRange?.lowerBoundAmount &&
				preview.referencePriceRange?.upperBoundAmount,
		);
		if (
			preview.referencePriceRange &&
			referenceHasBand &&
			!isAscending([
				preview.referencePriceRange.lowerBoundAmount,
				preview.referencePriceRange.midpointAmount,
				preview.referencePriceRange.upperBoundAmount,
			])
		) {
			context.addIssue({
				code: "custom",
				message: "reference range must be ascending",
				path: ["referencePriceRange"],
			});
		}
	});

export type RfqPricePreview = z.infer<typeof rfqPricePreviewSchema>;

export const rfqConfirmationSchema = z.object({
	confirmedRfqSnapshotId: idSchema,
	payloadHash: payloadHashSchema,
	expiresAt: z.string().default(""),
});

export type RfqConfirmation = z.infer<typeof rfqConfirmationSchema>;

export interface RfqPricePreviewCommand {
	expectedRevisionId: string;
	clientRequestId: string;
	pricingMode: RfqPricingMode;
	deliveryCity: string;
	buyerRequestedRange?: {
		currency: "TWD";
		priceComponent: "GOODS_ONLY";
		lowerBoundAmount: string;
		upperBoundAmount: string;
	};
}

export interface RfqConfirmationCommand {
	clientRequestId: string;
	expectedRevisionId: string;
	pricePreviewId: string;
	pricePreviewVersion: string;
	payloadHash: string;
	buyerNote: string;
}

export const BUYER_NOTE_MAX_LENGTH = 200;

export type RfqCommandFailureCode =
	| "MAPPING_MISSING"
	| "PRICE_BASIS_UNAVAILABLE"
	| "STALE_REVISION"
	| "PRICE_PREVIEW_STALE"
	| "IDEMPOTENCY_KEY_CONFLICT"
	| "REQUEST_NOT_FOUND"
	| "RESOURCE_NOT_FOUND"
	| "RATE_LIMITED"
	| "UNAUTHORIZED"
	| "FORBIDDEN"
	| "INVALID_RESPONSE"
	| "STALE_CONTEXT"
	| "NETWORK_ERROR"
	| "SERVER_ERROR"
	| "UNKNOWN_ERROR";

export interface RfqCommandFailure {
	code: RfqCommandFailureCode;
	detail: string;
	retryable: boolean;
}

export type RfqCommandResult<T> =
	| { ok: true; data: T }
	| { ok: false; error: RfqCommandFailure };

export function validateBuyerDesiredRange(
	lower: string,
	upper: string,
): string | null {
	const normalizedLower = lower.trim();
	const normalizedUpper = upper.trim();
	if (!normalizedLower || !normalizedUpper) {
		return "請完整填寫希望下限與上限。";
	}
	if (!/^[1-9]\d{0,11}$/.test(normalizedLower)) {
		return "希望下限須為大於 0 的新台幣整數。";
	}
	if (!/^[1-9]\d{0,11}$/.test(normalizedUpper)) {
		return "希望上限須為大於 0 的新台幣整數。";
	}
	if (BigInt(normalizedLower) > BigInt(normalizedUpper)) {
		return "希望上限不可低於下限。";
	}
	return null;
}

export function priceBasisOf(preview: RfqPricePreview): {
	basisQuantity: string;
	basisUnit: string;
} {
	const source = preview.buyerRequestedRange ?? preview.referencePriceRange;
	return {
		basisQuantity: source?.basisQuantity ?? "",
		basisUnit: source?.basisUnit ?? "",
	};
}

export function hasConfirmablePriceBasis(
	preview: RfqPricePreview,
	expectedBasisUnit?: string,
): boolean {
	const { basisQuantity, basisUnit } = priceBasisOf(preview);
	const isCanonical =
		((basisUnit === "KG" || basisUnit === "CATTY") && basisQuantity === "1") ||
		(basisUnit === "GRAM" && basisQuantity === "1000");
	return (
		isCanonical &&
		(expectedBasisUnit === undefined || basisUnit === expectedBasisUnit)
	);
}
