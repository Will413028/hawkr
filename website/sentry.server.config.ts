import * as Sentry from "@sentry/nextjs";
import { scrubSentryEvent } from "@/lib/sentry/scrub";
import { tagBackendTrace } from "@/lib/sentry/trace-tag";

const isProdFlavor =
	(process.env.NEXT_PUBLIC_APP_ENV ?? process.env.NEXT_PUBLIC_FLAVOR) ===
	"prod";

Sentry.init({
	dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
	enabled: process.env.NODE_ENV !== "development",
	environment:
		process.env.NEXT_PUBLIC_APP_ENV ??
		process.env.NEXT_PUBLIC_FLAVOR ??
		process.env.NODE_ENV,
	tracesSampleRate: isProdFlavor ? 0.05 : 1.0,
	beforeSend: (event, hint) => scrubSentryEvent(tagBackendTrace(event, hint)),
});
