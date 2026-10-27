import type { ProcurementLane } from "@/lib/api/openapi-types";
import type { PublicNarrativeBlock } from "./narrative";

export type ProcurementStage =
	| "clarifying"
	| "ready"
	| "searching"
	| "candidates";

export interface RecordedField {
	key: string;
	label: string;
	value: string;
}

export interface RejectedField {
	key: string;
	reason: string;
}

export interface ClarifyQuestion {
	slotKey: string;
	prompt: string;
	options: Array<{ optionId: string; label: string; value: string }>;
	unitOptions?: Array<{ optionId: string; label: string; value: string }>;
	allowFreeText: boolean;
	source: "PHRASE_LLM" | "FIXED";
	canSkip: boolean;
}

export interface CropDisambiguationOption {
	optionId: string;
	label: string;
	cropCode: string | null;
	groupCode: string | null;
}

export interface CropDisambiguation {
	prompt: string;
	options: CropDisambiguationOption[];
}

export type PriceAvailability = "MARKET" | "MARKET_STALE" | "UNAVAILABLE";

export interface ProcurementChatTurn {
	id: string;
	role: "buyer" | "assistant";
	text: string;
	blocks?: PublicNarrativeBlock[];
	recordedThisTurn?: RecordedField[];
	rejectedThisTurn?: RejectedField[];
	createdAt: number;
}
