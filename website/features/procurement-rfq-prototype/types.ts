export type RfqPrototypeRole = "buyer" | "seller" | "vendor" | "finance";

export type RfqPrototypeWave = "wave-1" | "later";

export type ExternalVendorAccessStep =
	| "invitation"
	| "exchange"
	| "sign-in"
	| "verify-code"
	| "session-expired"
	| "invite-unavailable"
	| "portal";

export type ExternalVendorTransactionStep =
	| "business-verification"
	| "rfq"
	| "quote"
	| "review"
	| "transaction";

export type ExternalVendorStep =
	| ExternalVendorAccessStep
	| ExternalVendorTransactionStep;

export interface ExternalVendorStepDefinition {
	slug: ExternalVendorStep;
	label: string;
	shortLabel: string;
	description: string;
}

export interface ExternalVendorBuyerOffer {
	id: string;
	status: "buyer-confirmed";
	version: string;
	buyerConfirmedAt: string;
	pricebookAsOf: string;
	pricebookVersion: string;
	pricingRuleVersion: string;
	pricebookFloor: number;
	pricebookCeiling: number;
	buyerMinimumUnitPrice: number;
	buyerMaximumUnitPrice: number;
	quantity: number;
	unit: string;
	transactionUnitPrice: number;
	qualityTarget: "價格優先" | "均衡品質" | "品質優先";
	retailMultiplier: number;
	retailUnitPrice: number;
	freight: number;
	subtotal: number;
	total: number;
	tax: "含稅";
	deliveryAt: string;
	validUntil: string;
}

export type RfqPrototypeScreenSlug =
	| "candidate-handoff"
	| "rfq-compose"
	| "seller-inbox"
	| "seller-response"
	| "quote-compare"
	| "checkout"
	| "atm-payment"
	| "external-vendor"
	| "seller-order"
	| "aftersales"
	| "finance";

export interface RfqPrototypeScreenDefinition {
	slug: RfqPrototypeScreenSlug;
	label: string;
	title: string;
	description: string;
	role: RfqPrototypeRole;
	wave: RfqPrototypeWave;
	targetId?: "B-02" | "B-04/B-05" | "B-06" | "E-03" | "S-01" | "B-08" | "F-01";
}

export interface RfqPrototypeSupplier {
	id: string;
	name: string;
	lane: "平台賣家" | "外部業者";
	selected: boolean;
	matchScore: number;
	status: string;
	facts: string[];
}

export interface RfqPrototypeQuote {
	id: string;
	supplier: string;
	total: number;
	unitPrice: number;
	availableQuantity: number;
	leadTime: string;
	freight: string;
	tax: string;
	validUntil: string;
	submittedAt: string;
	version: string;
	notes: string[];
	shortlisted: boolean;
}
