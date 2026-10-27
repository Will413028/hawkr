import * as Sentry from "@sentry/nextjs";
import { scrubSentryEvent } from "@/lib/sentry/scrub";
import { tagBackendTrace } from "@/lib/sentry/trace-tag";

Sentry.init({
	dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
	environment: process.env.NEXT_PUBLIC_APP_ENV ?? process.env.NODE_ENV,
	tracesSampleRate: process.env.NEXT_PUBLIC_APP_ENV === "prod" ? 0.05 : 0,
	tracePropagationTargets: process.env.BACKEND_BASE_URL?.trim()
		? [process.env.BACKEND_BASE_URL.trim()]
		: [],
	beforeSend: (event, hint) => scrubSentryEvent(tagBackendTrace(event, hint)),
});
