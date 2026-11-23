import { MediaDelegatedService } from "@hawkr/api/gen/hawkr/media/v1alpha1/media_pb";
import {
	forwardDelegatedConnect,
	stagedTokenSegment,
} from "../../../../_forward";

/**
 * `POST /adm/api/media/upload_sessions/{stagedToken}/complete` → `MediaDelegatedService.CompleteUpload`。
 * 路徑段是 staged token（JWT 形狀的短期憑證）：不進 log、不進稽核快照。
 */

type RouteContext = { params: Promise<{ sessionId: string }> };

export async function POST(
	request: Request,
	context: RouteContext,
): Promise<Response> {
	const { sessionId } = await context.params;
	return forwardDelegatedConnect(request, () => {
		const stagedToken = stagedTokenSegment(sessionId);
		return {
			service: MediaDelegatedService,
			method: "POST",
			timeout: "mutation",
			build: () => async (client, options) => {
				await client.completeUpload({ stagedToken }, options);
				return {};
			},
		};
	});
}
