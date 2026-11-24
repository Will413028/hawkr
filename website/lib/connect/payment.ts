import { createClient } from "@connectrpc/connect";
import { PaymentAppService } from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import { authedTransport } from "./transport";

/** Browser callers share the generated buyer payment contract. */
export function paymentAppClient() {
	return createClient(PaymentAppService, authedTransport());
}
