"use client";

import { Chip, type ChipColor } from "@/components/ui/Chip";
import { JOB_STATUS_LABEL, JOB_STATUS_TONE, type StatusTone } from "../i18n";
import { IMPORT_JOB_STATUS, type ImportJobStatusCode } from "../types";

interface Props {
	status: ImportJobStatusCode;
	size?: "sm" | "md";
}

const TONE_TO_COLOR: Record<StatusTone, ChipColor> = {
	primary: "primary",
	success: "success",
	warning: "warning",
	danger: "danger",
	default: "default",
};

export function ImportJobStatusBadge({ status, size = "sm" }: Props) {
	const label = JOB_STATUS_LABEL[status] ?? status;
	const tone = JOB_STATUS_TONE[status] ?? "default";
	const variant = status === IMPORT_JOB_STATUS.PARSING ? "dot" : "flat";
	return (
		<Chip variant={variant} color={TONE_TO_COLOR[tone]} size={size}>
			{label}
		</Chip>
	);
}
