"use client";

import { type Timestamp, timestampDate } from "@bufbuild/protobuf/wkt";
import { Chip } from "@/components/ui";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useStoreCardViews } from "@/lib/seller/useStoreCardViews";

export function formatReportTime(time: Timestamp | undefined): string {
	return time ? formatTaipeiDateTime(timestampDate(time)) : "";
}

export function ReportOutcomeChip({ text }: { text: string }) {
	return (
		<Chip size="sm" variant="flat" color="default">
			{text}
		</Chip>
	);
}

/** 店名：卡片還沒讀到或沒有卡片時不顯示文字（只留佔位，不退回快照）。 */
export function StoreName({ storeId }: { storeId: string }) {
	const { views } = useStoreCardViews([storeId]);
	const name = views.get(storeId)?.name;
	return name ? (
		<span>{name}</span>
	) : (
		<span aria-hidden className="inline-block h-4 w-24 rounded bg-content2" />
	);
}
