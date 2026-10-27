import { type Timestamp, timestampDate } from "@bufbuild/protobuf/wkt";
import { formatIsoDate } from "@/lib/format";

/** 建立時間 → 台北的日期 `2026-09-27`；沒有值時是 `-`。 */
export function formatCreateDate(ts: Timestamp | undefined): string {
	return ts ? formatIsoDate(timestampDate(ts)) : "-";
}
