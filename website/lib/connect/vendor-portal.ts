import { createClient } from "@connectrpc/connect";
import {
	VendorPortalInviteeService,
	VendorPortalPublicService,
} from "@hawkr/api/gen/hawkr/agent/v1alpha1/agent_pb";
import { vendorPortalTransport } from "./transport";

/** 供應商入口、以 secret 為憑證的兩個方法（Preview、Exchange）；憑證是邀請範圍的 cookie（見 vendorPortalTransport）。 */
export function vendorPortalClient() {
	return createClient(VendorPortalPublicService, vendorPortalTransport());
}

/** 已兌換邀請的供應商（以 cookie 為憑證）：GetSession、VerifySession；同一個 transport。 */
export function vendorPortalInviteeClient() {
	return createClient(VendorPortalInviteeService, vendorPortalTransport());
}
