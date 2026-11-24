/** Notification preference types (legacy REST, pending the preferences migration). */

/**
 * Per-category preference toggle. Backend (`GET/PUT
 * /app/{buyers|sellers}/me/notification-preferences`) tracks a single
 * `enabled` flag per category — no per-channel (push/email/in_app)
 * granularity is wired through the gateway. `title` + `description`
 * come from the backend so categories can be added server-side without
 * a client redeploy.
 *
 * `forced` flags categories the backend won't let the user disable
 * (notification design doc §3.1 — e.g. `shipment_anomaly` for sellers).
 * The UI renders the toggle locked-on for these rows. Some backend
 * builds may not include the flag yet, so callers should additionally
 * fall back to a hard-coded `FORCED_BY_ROLE` map.
 */
export interface NotificationPreferenceItem {
	category: string;
	title?: string;
	description?: string;
	enabled: boolean;
	forced?: boolean;
}
