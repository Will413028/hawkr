"use client";

import { Chip, type ChipColor } from "@/components/ui/Chip";
import { ITEM_STATUS_LABEL, ITEM_STATUS_TONE, type StatusTone } from "../i18n";
import { IMPORT_ITEM_STATUS, type ImportItemStatusCode } from "../types";

interface Props {
	status: ImportItemStatusCode;
	needsFix?: boolean;
	size?: "sm" | "md";
}

const TONE_TO_COLOR: Record<StatusTone, ChipColor> = {
	primary: "primary",
	success: "success",
	warning: "warning",
	danger: "danger",
	default: "default",
};

/** @import import { ImportItemStatusBadge } from '@/features/seller-center/products-import/components/ImportItemStatusBadge'; */
export function ImportItemStatusBadge({
	status,
	needsFix = false,
	size = "sm",
}: Props) {
	const label = ITEM_STATUS_LABEL[status] ?? status;
	const baseTone = ITEM_STATUS_TONE[status] ?? "default";
	const tone: StatusTone =
		needsFix && status === IMPORT_ITEM_STATUS.READY ? "warning" : baseTone;
	const display =
		needsFix && status === IMPORT_ITEM_STATUS.READY ? "需修正" : label;
	return (
		<Chip variant="flat" color={TONE_TO_COLOR[tone]} size={size}>
			{display}
		</Chip>
	);
}
