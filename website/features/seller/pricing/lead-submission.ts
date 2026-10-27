import type { PublicBillingCycle, PublicSellerPlanId } from "./public-data";

export type SellerLeadPlanId = PublicSellerPlanId | "undecided";
export type SellerLeadServiceInterest = "none" | "managed";

export interface SellerLeadSubmission {
	brandName: string;
	contactName: string;
	phone: string;
	email: string;
	productType: string;
	plan: SellerLeadPlanId;
	billingCycle: PublicBillingCycle;
	serviceInterest: SellerLeadServiceInterest;
}

export type SellerLeadTrackingContext = Pick<
	SellerLeadSubmission,
	"plan" | "billingCycle" | "serviceInterest"
>;

interface SellerLeadResponse {
	ok: boolean;
	json: () => Promise<unknown>;
}

export interface SubmitSellerLeadDeps {
	post: (submission: SellerLeadSubmission) => Promise<SellerLeadResponse>;
	onAccepted: (context: SellerLeadTrackingContext) => void;
}

/**
 * Submit one seller lead and reveal only non-PII categorical context to
 * analytics after the server explicitly accepts it.
 */
export async function submitSellerLeadRequest(
	submission: SellerLeadSubmission,
	deps: SubmitSellerLeadDeps,
): Promise<void> {
	const response = await deps.post(submission);
	const result = (await response.json().catch(() => null)) as {
		ok?: boolean;
	} | null;
	if (!response.ok || result?.ok !== true) {
		throw new Error("seller lead was not accepted");
	}
	try {
		deps.onAccepted({
			plan: submission.plan,
			billingCycle: submission.billingCycle,
			serviceInterest: submission.serviceInterest,
		});
	} catch {
		// Analytics must not turn an accepted lead into a visible submission error
		// or encourage the user to send the same contact details twice.
	}
}
