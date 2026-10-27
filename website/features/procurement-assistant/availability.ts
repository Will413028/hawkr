import { env } from "@/lib/config/env";

/**
 * Top-level Hawkr Agent availability gate.
 *
 * The feature stays available on DEV for product work, while production fails
 * closed unless it is explicitly re-enabled. This is intentionally separate
 * from the capability flags: those flags select Agent sub-features after the
 * workspace is available; this gate controls whether any public entry or route
 * exists at all.
 *
 * The environment value is resolved centrally in `lib/config/env.ts`, so Server
 * Components, client navigation, and Next config all share one build-time truth.
 * Re-enabling it needs a deployment; this is not an instant runtime kill switch.
 */
export const HAWKR_AGENT_ENABLED = env.hawkrAgentEnabled;
