import {
	Code,
	ConnectError,
	createClient,
	createRouterTransport,
} from "@connectrpc/connect";
import { ErrorInfoSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import {
	AgreementKind,
	type ApplyStoreRequest,
	SellerAppService,
	type SubmitKYCRequest,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it, vi } from "vitest";
import { SELLER_AGREEMENTS } from "@/features/legal/documents/sellerAgreements.gen";
import { acceptancesOf, agreementUrl, isAgreementOutdated } from "./agreements";
import { submitKyc } from "./kyc";

const requests = vi.hoisted(() => ({
	apply: [] as unknown[],
}));

vi.mock("../connect/seller", async () => {
	const { createClient, createRouterTransport } = await import(
		"@connectrpc/connect"
	);
	const { SellerAppService } = await import(
		"@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb"
	);
	return {
		sellerAppClient: () =>
			createClient(
				SellerAppService,
				createRouterTransport((router) =>
					router.service(SellerAppService, {
						applyStore(req) {
							requests.apply.push(req);
							return { store: { id: "1", etag: "e1" } };
						},
					}),
				),
			),
	};
});

describe("acceptancesOf / agreementUrl", () => {
	it("版本與網址取自生成的常數，不是寫死的值", () => {
		expect(
			acceptancesOf([
				AgreementKind.SELLER_TERMS,
				AgreementKind.KYC_PRIVACY_NOTICE,
				AgreementKind.PAYUNI_TERMS,
			]),
		).toEqual([
			{
				kind: AgreementKind.SELLER_TERMS,
				version: SELLER_AGREEMENTS.SELLER_TERMS.version,
			},
			{
				kind: AgreementKind.KYC_PRIVACY_NOTICE,
				version: SELLER_AGREEMENTS.KYC_PRIVACY_NOTICE.version,
			},
			{
				kind: AgreementKind.PAYUNI_TERMS,
				version: SELLER_AGREEMENTS.PAYUNI_TERMS.version,
			},
		]);
		expect(agreementUrl(AgreementKind.PAYUNI_TERMS)).toBe(
			SELLER_AGREEMENTS.PAYUNI_TERMS.url,
		);
	});

	it("UNSPECIFIED 不在 bundle 裡：throw，不送出空版本", () => {
		expect(() => acceptancesOf([AgreementKind.UNSPECIFIED])).toThrow();
		expect(() => agreementUrl(AgreementKind.UNSPECIFIED)).toThrow();
	});
});

describe("isAgreementOutdated", () => {
	const withReason = (reason: ErrorReason, code = Code.FailedPrecondition) =>
		new ConnectError("x", code, undefined, [
			{
				desc: ErrorInfoSchema,
				value: { reason: ErrorReason[reason], domain: "hawkr" },
			},
		]);

	it("failed_precondition 加 AGREEMENT_OUTDATED 才是", () => {
		expect(
			isAgreementOutdated(withReason(ErrorReason.AGREEMENT_OUTDATED)),
		).toBe(true);
	});

	it("其他 reason、其他 code、沒有 reason 都不是", () => {
		expect(isAgreementOutdated(withReason(ErrorReason.VERSION_CONFLICT))).toBe(
			false,
		);
		expect(
			isAgreementOutdated(
				withReason(ErrorReason.AGREEMENT_OUTDATED, Code.InvalidArgument),
			),
		).toBe(false);
		expect(
			isAgreementOutdated(new ConnectError("x", Code.FailedPrecondition)),
		).toBe(false);
		expect(isAgreementOutdated(new Error("boom"))).toBe(false);
	});
});

describe("送出時帶上同意", () => {
	it("applyStore 把 accepted_agreements 放進請求", async () => {
		const { applyStore } = await import("./repo");
		await applyStore({
			storeName: "山漾食品",
			acceptedAgreements: [
				{ kind: AgreementKind.SELLER_TERMS, version: "2031-01-02" },
			],
		});
		const sent = requests.apply[0] as ApplyStoreRequest;
		expect(sent.acceptedAgreements.map((a) => [a.kind, a.version])).toEqual([
			[AgreementKind.SELLER_TERMS, "2031-01-02"],
		]);
	});

	it("submitKyc 把兩份同意放進請求", async () => {
		const sent: SubmitKYCRequest[] = [];
		const transport = createRouterTransport((router) =>
			router.service(SellerAppService, {
				submitKYC(req) {
					sent.push(req);
					return { application: { etag: "e2" } };
				},
			}),
		);
		await submitKyc(
			"e1",
			[
				{ kind: AgreementKind.KYC_PRIVACY_NOTICE, version: "2031-01-03" },
				{ kind: AgreementKind.PAYUNI_TERMS, version: "2031-01-04" },
			],
			() => createClient(SellerAppService, transport),
		);
		expect(sent).toHaveLength(1);
		expect(sent[0].etag).toBe("e1");
		expect(sent[0].acceptedAgreements.map((a) => [a.kind, a.version])).toEqual([
			[AgreementKind.KYC_PRIVACY_NOTICE, "2031-01-03"],
			[AgreementKind.PAYUNI_TERMS, "2031-01-04"],
		]);
	});
});
