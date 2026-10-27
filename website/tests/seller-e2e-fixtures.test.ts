import assert from "node:assert/strict";
import test from "node:test";
import { fromJson } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { errorFromJson } from "@connectrpc/connect/protocol-connect";
import {
	classifyError,
	factsFromError,
	userMessage,
} from "@hawkr/api/connect/errors";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import {
	AppealUnavailableReason,
	GetMyKYCResponseSchema,
	GetMyStoreResponseSchema,
	KYCFlowStatus,
	MeasureBasis,
	MeasureKind,
	MeasureRequestCloseCause,
	MeasureRequestKind,
	MeasureRequestState,
	MeasureSchema,
	MeasureState,
	RemediationUnavailableReason,
	RevokeReason,
	RiskGround,
	SellerCapability,
	SellerRequirement,
	StoreStatus,
	TerminationGround,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	ALL_SELLER_CAPABILITIES,
	errorInfoDetail,
	localizedMessageDetail,
	measureJson,
	myStoreJson,
	PENDING_DELETION_SELLER_CAPABILITIES,
	SUSPENDED_SELLER_CAPABILITIES,
} from "./e2e/helpers/mock-gateway";

/**
 * e2e 的 Connect fixture 必須是契約的真實 wire 形狀（website-e2e-verify「Mock 必須是真實的 wire 形狀」）：
 * 這裡把 fixture 交給生成碼的 proto3 JSON 解析器，欄位名或列舉名寫錯會當場丟錯，而不是在瀏覽器裡
 * 悄悄變成空值。
 */

test("myStoreJson parses as a GetMyStoreResponse with every field mapped", () => {
	const message = fromJson(
		GetMyStoreResponseSchema,
		myStoreJson({
			id: "90030001",
			storeCode: "2600000003",
			storeName: "Connect 測試商店",
			description: "原本的介紹",
			contactPhone: "0912345678",
			address: { townCode: "66000290", addressDetail: "東關路一段 100 號" },
			approvedFoodRegistration: { registrationNumber: "A-1" },
			pendingFoodRegistration: {
				state: "FOOD_REGISTRATION_STATE_PENDING",
				registrationNumber: "A-2",
			},
			logoUriTemplate: "https://img.test/{preset}/logo.png",
			etag: "e7",
			missingRequirements: ["KYC_APPROVED", "LOGO"],
		}),
	);

	assert.equal(message.store?.id, "90030001");
	assert.equal(message.store?.storeCode, "2600000003");
	assert.equal(message.store?.address?.townCode, "66000290");
	assert.equal(
		message.store?.approvedFoodRegistration?.registrationNumber,
		"A-1",
	);
	assert.equal(
		message.store?.pendingFoodRegistration?.registrationNumber,
		"A-2",
	);
	assert.equal(
		message.store?.logo?.uriTemplate,
		"https://img.test/{preset}/logo.png",
	);
	assert.equal(message.store?.etag, "e7");
	assert.equal(message.store?.status, StoreStatus.ACTIVE);
	assert.equal(message.store?.onboardingCompleted, true);
	const listing = message.capabilities.find(
		(status) => status.capability === SellerCapability.PUBLIC_LISTING,
	);
	assert.deepEqual(listing?.missing, [
		SellerRequirement.KYC_APPROVED,
		SellerRequirement.LOGO,
	]);
	assert.equal(listing?.active, false);
	const shipping = message.capabilities.find(
		(status) => status.capability === SellerCapability.SHIPPING_SETUP,
	);
	assert.equal(shipping?.active, false);
	assert.deepEqual(shipping?.missing, [SellerRequirement.KYC_APPROVED]);
	assert.equal(message.suspended, false);
});

test("myStoreJson carries `suspended` and pending deletion as the contract names them", () => {
	const suspended = fromJson(
		GetMyStoreResponseSchema,
		myStoreJson({
			id: "1",
			suspended: true,
			capabilities: SUSPENDED_SELLER_CAPABILITIES,
			missingRequirements: ["NOT_SUSPENDED"],
		}),
	);
	assert.equal(suspended.suspended, true);

	const pending = fromJson(
		GetMyStoreResponseSchema,
		myStoreJson({
			id: "1",
			status: "PENDING_DELETION",
			capabilities: PENDING_DELETION_SELLER_CAPABILITIES,
		}),
	);
	assert.equal(pending.store?.status, StoreStatus.PENDING_DELETION);
	const storefront = pending.capabilities.find(
		(status) => status.capability === SellerCapability.STOREFRONT_WRITE,
	);
	assert.equal(storefront?.active, false);
	assert.deepEqual(storefront?.missing, [
		SellerRequirement.NOT_PENDING_DELETION,
	]);
});

test("every capability the fixture names exists in the contract", () => {
	const message = fromJson(
		GetMyStoreResponseSchema,
		myStoreJson({ id: "1", capabilities: ALL_SELLER_CAPABILITIES }),
	);
	assert.equal(message.capabilities.length, ALL_SELLER_CAPABILITIES.length + 1);
	assert.ok(
		message.capabilities.every(
			(status) => status.capability !== SellerCapability.UNSPECIFIED,
		),
	);
	assert.ok(
		message.capabilities.every(
			(status) => status.active === (status.missing.length === 0),
		),
	);
});

test("the KYC flow fixture shape parses as a GetMyKYCResponse", () => {
	const message = fromJson(GetMyKYCResponseSchema, {
		application: {
			id: "1",
			flow: { status: "KYC_FLOW_STATUS_APPROVED", identityLocked: true },
		},
	});
	assert.equal(
		message.application?.flow?.status,
		KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
	);
	assert.equal(message.application?.flow?.identityLocked, true);
});

test("the version-conflict error fixture is classified as VERSION_CONFLICT", () => {
	const err = errorFromJson(
		{
			code: "aborted",
			message: "version conflict",
			details: [errorInfoDetail("VERSION_CONFLICT")],
		},
		undefined,
		new ConnectError("fallback", Code.Unknown),
	);
	const decision = classifyError(factsFromError(err));
	assert.equal(decision.action, "reason");
	assert.equal(decision.reason, ErrorReason.VERSION_CONFLICT);
});

test("the localized-message fixture is what the client shows to the user", () => {
	const err = errorFromJson(
		{
			code: "failed_precondition",
			message: "has open orders",
			details: [localizedMessageDetail("還有未結訂單，無法關閉店家")],
		},
		undefined,
		new ConnectError("fallback", Code.Unknown),
	);
	assert.equal(
		userMessage(err, () => {
			throw new Error("有 LocalizedMessage 時不該用預設文字");
		}),
		"還有未結訂單，無法關閉店家",
	);
});

test("measureJson parses as a Measure with every field mapped", () => {
	const message = fromJson(
		MeasureSchema,
		measureJson({
			id: "m1",
			kind: "SUSPENSION",
			basis: "RISK",
			riskGround: "FOOD_SAFETY",
			statement: "近期通報集中",
			state: "ACTIVE",
			period: { startTime: "2026-10-01T02:00:00Z" },
			requests: [
				{
					id: "r1",
					kind: "APPEAL",
					state: "REJECTED",
					reason: "已改善",
					submitTime: "2026-10-02T02:00:00Z",
					decisionReason: "證據不足",
					decideTime: "2026-10-03T02:00:00Z",
				},
			],
			createTime: "2026-10-01T02:00:00Z",
			etag: "m7",
			canAppeal: false,
		}),
	);
	assert.equal(message.kind, MeasureKind.SUSPENSION);
	assert.equal(message.basis, MeasureBasis.RISK);
	assert.equal(message.riskGround, RiskGround.FOOD_SAFETY);
	assert.equal(message.state, MeasureState.ACTIVE);
	assert.equal(message.period?.endTime, undefined);
	assert.equal(message.requests[0]?.kind, MeasureRequestKind.APPEAL);
	assert.equal(message.requests[0]?.state, MeasureRequestState.REJECTED);
	assert.equal(message.requests[0]?.decisionReason, "證據不足");
	assert.equal(message.etag, "m7");
	assert.equal(message.canAppeal, false);

	const revoked = fromJson(
		MeasureSchema,
		measureJson({
			id: "m2",
			kind: "SUSPENSION",
			basis: "VIOLATION",
			findings: [
				{
					id: "f1",
					category: "FOOD_SAFETY",
					severity: "HIGH",
					createTime: "2026-10-01T02:00:00Z",
				},
			],
			statement: "x",
			state: "REVOKED",
			createTime: "2026-10-01T02:00:00Z",
			canAppeal: false,
			revokeReason: "FINDING_REVOKED",
		}),
	);
	assert.equal(revoked.revokeReason, RevokeReason.FINDING_REVOKED);
	assert.equal(revoked.findings[0]?.id, "f1");
});

test("measureJson omits proto3 default values like the real wire: no empty findings, no empty statement, no false canAppeal", () => {
	const json = measureJson({
		id: "m3",
		kind: "WARNING",
		basis: "RISK",
		riskGround: "FRAUD",
		statement: "",
		state: "ISSUED",
		createTime: "2026-10-01T02:00:00Z",
		canAppeal: false,
	});
	assert.equal("findings" in json, false);
	assert.equal("statement" in json, false);
	assert.equal("canAppeal" in json, false);
	assert.equal("period" in json, false);
	const parsed = fromJson(MeasureSchema, json);
	assert.equal(parsed.findings.length, 0);
	assert.equal(parsed.statement, "");
});

test("measureJson maps remediation fields onto the wire shape the generated Measure parses", () => {
	const parsed = fromJson(
		MeasureSchema,
		measureJson({
			id: "m4",
			kind: "SUSPENSION",
			basis: "RISK",
			riskGround: "SECURITY",
			statement: "x",
			state: "ACTIVE",
			createTime: "2026-10-01T02:00:00Z",
			canAppeal: false,
			canRequestRemediation: true,
			nextRemediationTime: "2026-10-20T02:05:00Z",
			rejectionCooldownSeconds: 14 * 86400,
			remediationUnavailabilities: [
				{
					reason: "COOLDOWN",
					message: "解除請求目前在冷卻期內，2026/10/20 10:06 後可再提出",
				},
				{ reason: "NOT_ACTIVE" },
			],
			requests: [
				{
					id: "q2",
					kind: "REMEDIATION",
					state: "CLOSED",
					reason: "r",
					submitTime: "2026-10-05T02:00:00Z",
					decideTime: "2026-10-06T02:00:00Z",
					closeCause: "ENDED",
				},
				{
					id: "q1",
					kind: "APPEAL",
					state: "REJECTED",
					reason: "r",
					submitTime: "2026-10-02T02:00:00Z",
					decisionReason: "照片不清楚",
					decideTime: "2026-10-03T02:00:00Z",
				},
			],
		}),
	);
	assert.equal(parsed.canRequestRemediation, true);
	assert.equal(parsed.requests.length, 2);
	assert.equal(parsed.requests[0]?.id, "q2");
	assert.equal(parsed.requests[0]?.kind, MeasureRequestKind.REMEDIATION);
	assert.equal(parsed.requests[0]?.closeCause, MeasureRequestCloseCause.ENDED);
	assert.equal(parsed.requests[1]?.kind, MeasureRequestKind.APPEAL);
	assert.equal(parsed.requests[1]?.decisionReason, "照片不清楚");
	assert.deepEqual(
		parsed.remediationUnavailabilities.map((u) => [
			u.reason,
			u.message?.locale,
			u.message?.message,
		]),
		[
			[
				RemediationUnavailableReason.COOLDOWN,
				"zh-TW",
				"解除請求目前在冷卻期內，2026/10/20 10:06 後可再提出",
			],
			[RemediationUnavailableReason.NOT_ACTIVE, undefined, undefined],
		],
	);
	assert.equal(
		parsed.nextRemediationTime?.seconds,
		BigInt(Date.UTC(2026, 9, 20, 2, 5, 0) / 1000),
	);
	assert.equal(parsed.rejectionCooldown?.seconds, BigInt(14 * 86400));
});

test("measureJson maps the termination fields onto the wire shape the generated Measure parses", () => {
	const parsed = fromJson(
		MeasureSchema,
		measureJson({
			id: "t1",
			kind: "TERMINATION",
			basis: "CONTRACT",
			statement: "x",
			state: "SCHEDULED",
			// 生效時間刻意不是 createTime + 30 天（2026-11-04）：client 不能從建立時間推算。
			period: { startTime: "2026-11-01T02:00:00Z" },
			createTime: "2026-10-05T02:00:00Z",
			canAppeal: false,
			terminationGround: "NOTICE",
			appealDeadlineTime: "2027-05-04T02:00:00Z",
			appealUnavailabilities: [
				{ reason: "APPEAL_WINDOW_ENDED", message: "終止合作的申覆期限已過" },
				{
					reason: "REQUEST_PENDING",
					message: "這筆措施有待處理的解除請求，審核完成後才能申覆",
				},
			],
			revokeReason: "SOURCE_REVOKED",
		}),
	);
	assert.equal(parsed.kind, MeasureKind.TERMINATION);
	assert.equal(parsed.basis, MeasureBasis.CONTRACT);
	assert.equal(parsed.state, MeasureState.SCHEDULED);
	assert.equal(parsed.terminationGround, TerminationGround.NOTICE);
	assert.equal(
		parsed.appealDeadlineTime?.seconds,
		BigInt(Date.UTC(2027, 4, 4, 2, 0, 0) / 1000),
	);
	assert.deepEqual(
		parsed.appealUnavailabilities.map((u) => [u.reason, u.message?.message]),
		[
			[AppealUnavailableReason.APPEAL_WINDOW_ENDED, "終止合作的申覆期限已過"],
			[
				AppealUnavailableReason.REQUEST_PENDING,
				"這筆措施有待處理的解除請求，審核完成後才能申覆",
			],
		],
	);
	assert.equal(parsed.revokeReason, RevokeReason.SOURCE_REVOKED);

	const plain = measureJson({
		id: "m5",
		kind: "SUSPENSION",
		basis: "RISK",
		statement: "x",
		state: "ACTIVE",
		createTime: "2026-10-01T02:00:00Z",
		canAppeal: true,
	});
	assert.equal("terminationGround" in plain, false);
	assert.equal("appealDeadlineTime" in plain, false);
	assert.equal("appealUnavailabilities" in plain, false);
});
