import type { SeverityChange } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { violationSeverityText } from "@/app/admin/merchants/[id]/measure";

/** 改嚴重度時寫的那則註記的標頭：「改嚴重度：{舊} → {新}」。 */
export function severityChangeText(
	change: Pick<SeverityChange, "fromSeverity" | "toSeverity">,
): string {
	return `改嚴重度：${violationSeverityText(change.fromSeverity)} → ${violationSeverityText(change.toSeverity)}`;
}
