import * as Sentry from "@sentry/nextjs";
import { scrubSentryEvent } from "@/lib/sentry/scrub";
import { tagBackendTrace } from "@/lib/sentry/trace-tag";

// NODE_ENV is always "production" for both the dev and prod deployment
// (both are production builds), so it can't distinguish them. NEXT_PUBLIC_FLAVOR
// can, and is what `environment` below already keys off of.
const isProdFlavor =
	(process.env.NEXT_PUBLIC_APP_ENV ?? process.env.NEXT_PUBLIC_FLAVOR) ===
	"prod";

Sentry.init({
	dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
	// 本機 next dev（NODE_ENV=development）不上報，隔離本機 runtime 噪音；
	// 部署版（含 dev flavor）NODE_ENV 恆為 production，不受影響。
	enabled: process.env.NODE_ENV !== "development",
	environment:
		process.env.NEXT_PUBLIC_APP_ENV ??
		process.env.NEXT_PUBLIC_FLAVOR ??
		process.env.NODE_ENV,
	// Pre-launch, dev traffic is low — sample every transaction to get a
	// statistically meaningful Web Vitals baseline. Dial back once real prod
	// traffic makes 100% too expensive.
	tracesSampleRate: isProdFlavor ? 0.05 : 1.0,
	beforeSend: (event, hint) => scrubSentryEvent(tagBackendTrace(event, hint)),
});
