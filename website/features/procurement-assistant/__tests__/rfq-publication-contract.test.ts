import { describe, expect, it } from "vitest";
import {
	publicationExecuteBody,
	publicationPreviewBody,
	rfqPublicationApprovalSchema,
	rfqPublicationResultSchema,
} from "../rfq-publication-contract";

const APPROVAL = {
	approvalId: "1234567890123456803",
	approvalVersion: "1",
	audienceHash: "a".repeat(64),
	sellerFacingPayloadHash: "b".repeat(64),
	sellerFacingPayloadJson: '{"item":"蘋果"}',
	expiresAt: "2026-08-22T12:36:00Z",
	audiencePolicyId: "WISH_POOL_ALL_ACTIVE_SELLERS_V1",
	audiencePolicyVersion: "1",
	recipientCount: "0",
};

const BASE = {
	previewClientRequestId: "preview-key-1",
	expectedRevisionId: "1234567890123456791",
	confirmedRfqSnapshotId: "1234567890123456802",
};

describe("RFQ publication preview body", () => {
	// docx V.7.5：Wish Pool 不得用空 recipient list 假裝定向邀請。Gateway 對
	// 「許願池帶了候選」與「指定對象沒選候選」都回 400，所以模式與名單必須是
	// 互斥的 discriminator，而不是兩個各自可填的欄位。
	it("sends no candidate set and no recipient list for the wish pool", () => {
		const body = publicationPreviewBody({
			...BASE,
			audience: { mode: "WISH_POOL" },
		});

		expect(body.audienceMode).toBe("WISH_POOL");
		expect(body.candidateSetId).toBe("");
		expect(body.selectedCandidateIds).toEqual([]);
	});

	// audienceHash 涵蓋這個順序，換順序等於換受眾（#2450 Scope 2）。
	it("preserves the selected recipient order for direct recipients", () => {
		const body = publicationPreviewBody({
			...BASE,
			audience: {
				mode: "DIRECT_RECIPIENTS",
				candidateSetId: "1234567890123456790",
				selectedCandidateIds: ["seller-b", "seller-a"],
			},
		});

		expect(body.audienceMode).toBe("DIRECT_RECIPIENTS");
		expect(body.candidateSetId).toBe("1234567890123456790");
		expect(body.selectedCandidateIds).toEqual(["seller-b", "seller-a"]);
	});

	// ⛔ audienceHash 由 Server compile 並回傳，Client 不得提供或猜測（docx V.7.5）。
	it("never carries a client-side audience hash", () => {
		const body = publicationPreviewBody({
			...BASE,
			audience: { mode: "WISH_POOL" },
		});

		expect(Object.keys(body)).not.toContain("audienceHash");
	});

	// Gateway 的 responseDeadline 是 *string + omitempty：送空字串會被讀成
	// 「有給截止日而且是空的」並寫進賣家看到的內容，所以沒填就整個不送這個 key。
	it("omits the response deadline key when the buyer set none", () => {
		const body = publicationPreviewBody({
			...BASE,
			audience: { mode: "WISH_POOL" },
		});

		expect("responseDeadline" in body).toBe(false);
	});
});

describe("RFQ publication approval contract", () => {
	// approvalId ＋ approvalVersion 是 execute 的 CAS token，而這支回應是唯一
	// 讀得到 approvalVersion 的地方。缺一個就沒有可執行的核准，必須 fail closed。
	it("fails closed when the approval version is missing", () => {
		const { approvalVersion: _omitted, ...withoutVersion } = APPROVAL;

		expect(rfqPublicationApprovalSchema.safeParse(withoutVersion).success).toBe(
			false,
		);
	});

	// WISH_POOL 沒有名單是該模式的定義，recipientCount 恆為 "0"——不能套用
	// 「正整數 id」那條規則把它當成壞資料。
	it("accepts the wish pool's zero recipient count", () => {
		const parsed = rfqPublicationApprovalSchema.parse(APPROVAL);

		expect(parsed.recipientCount).toBe("0");
		expect(parsed.audiencePolicyId).toBe("WISH_POOL_ALL_ACTIVE_SELLERS_V1");
	});
});

const RESULT = {
	publicationId: "1234567890123456804",
	publicationVersion: "1",
	state: "PUBLISHED",
	audienceMode: "WISH_POOL",
	audienceHash: "a".repeat(64),
	sellerFacingPayloadHash: "b".repeat(64),
	publishedAt: "2026-08-22T12:40:00Z",
	replayed: false,
};

describe("RFQ publication execute body", () => {
	it("carries only the five reference and CAS fields", () => {
		const body = publicationExecuteBody({
			clientRequestId: "execute-key-1",
			expectedRevisionId: BASE.expectedRevisionId,
			approval: APPROVAL,
		});

		expect(Object.keys(body).sort()).toEqual([
			"approvalId",
			"approvalVersion",
			"clientRequestId",
			"expectedRevisionId",
			"payloadHash",
		]);
	});

	it("takes the payload hash from the approval's seller-facing payload", () => {
		const body = publicationExecuteBody({
			clientRequestId: "execute-key-1",
			expectedRevisionId: BASE.expectedRevisionId,
			approval: APPROVAL,
		});

		expect(body.payloadHash).toBe(APPROVAL.sellerFacingPayloadHash);
		expect(body.approvalId).toBe(APPROVAL.approvalId);
		expect(body.approvalVersion).toBe(APPROVAL.approvalVersion);
	});
});

describe("RFQ publication result contract", () => {
	it("fails closed when the state is missing", () => {
		const { state: _omitted, ...withoutState } = RESULT;

		expect(rfqPublicationResultSchema.safeParse(withoutState).success).toBe(
			false,
		);
	});

	it("accepts a published wish pool result", () => {
		const parsed = rfqPublicationResultSchema.parse(RESULT);

		expect(parsed.state).toBe("PUBLISHED");
		expect(parsed.audienceMode).toBe("WISH_POOL");
		expect(parsed.replayed).toBe(false);
	});

	it("keeps the idempotent replay flag", () => {
		const parsed = rfqPublicationResultSchema.parse({
			...RESULT,
			replayed: true,
		});

		expect(parsed.replayed).toBe(true);
	});
});
