import { buyerProcurementClient } from "@/lib/connect/procurement";
import { snapshotFromProto } from "./proto-map";
import type { ProcurementSnapshot } from "./types";

export async function fetchProcurementSnapshot(
	requestId: string,
): Promise<ProcurementSnapshot> {
	return snapshotFromProto(
		await buyerProcurementClient().getSnapshot({ requestId }),
	);
}
