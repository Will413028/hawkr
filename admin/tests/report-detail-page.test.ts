import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	Code,
	ConnectError,
	createRouterTransport,
	type Transport,
} from "@connectrpc/connect";
import {
	ReporterContactSchema,
	ReportState,
	ReportSupplementSchema,
	SellerGovernanceAdminService,
	SupplementMatch,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { EvidenceSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import { ViolationSeverity } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { defaultErrorTextZhTW } from "@hawkr/ui-text/zh-TW";
import {
	bodyText,
	buttonByText,
	byId,
	click,
	dom,
	maybeById,
	mount,
	navigation,
	React,
	requireFromTest,
	setValue,
	until,
} from "./fixtures/dom-harness";
import {
	outcome,
	REPORT_ID,
	report,
	T0,
	target,
	ts,
	versionConflict,
	withMessage,
} from "./fixtures/report-fixtures";

/**
 * 通報詳情頁（/admin/violation-reports/[id]）實際掛進 DOM 後的行為：讀取中與失敗、內容與補充與檢舉人與佐證與處理、
 * 按鈕只看伺服器的 can_*（不能操作就隱藏，不顯示原因）、三個對話框的送出與版本衝突（重讀、保留輸入、顯示衝突句）。
 */

const Page = requireFromTest("../src/app/admin/violation-reports/[id]/page")
	.default as unknown;
const h = React.createElement;

type Req = Record<string, unknown>;
type Handler = (req: Req) => unknown;

function transportWith(handlers: Record<string, Handler>): Transport {
	return createRouterTransport(({ service }) => {
		service(SellerGovernanceAdminService, handlers as never);
	});
}

function failingFirst(inner: Transport, times: number): Transport {
	let remaining = times;
	return {
		...inner,
		unary: ((...args: Parameters<Transport["unary"]>) => {
			if (remaining > 0) {
				remaining--;
				return Promise.reject(
					ConnectError.from(new TypeError("Failed to fetch")),
				);
			}
			return inner.unary(...args);
		}) as Transport["unary"],
		stream: inner.stream.bind(inner),
	};
}

let current: { unmount: () => Promise<void> } | undefined;

/** 掛載頁面；同一個測試開第二個頁面時，前一個先卸載（畫面上同時只有一個頁面）。 */
async function openPage(transport: Transport, opts: { settle?: boolean } = {}) {
	await current?.unmount();
	navigation.params = { id: REPORT_ID };
	const m = mount(h(Page), transport);
	current = m;
	await m.render(opts);
	return m;
}

const buttons = () =>
	[...dom.window.document.querySelectorAll(".header-actions button")].map(
		(b) => b.textContent,
	);

const ACTIONS = ["分案", "設定結果", "移到其他案件", "查看案件"];

test("讀取中顯示「載入通報中...」；讀取失敗顯示原因並可重試（網路層失敗是「通報讀取失敗」）", async () => {
	await openPage(transportWith({ getReport: () => new Promise(() => {}) }), {
		settle: false,
	});
	assert.match(bodyText(), /載入通報中\.\.\./u);
});

test("讀取失敗：Connect 錯誤照伺服器的句子，網路層失敗是「通報讀取失敗」，都有重試", async () => {
	let calls = 0;
	await openPage(
		transportWith({
			getReport: () => {
				calls++;
				if (calls === 1) throw withMessage(Code.NotFound, "找不到這筆通報。");
				return { report: report({}) };
			},
		}),
	);
	assert.match(bodyText(), /找不到這筆通報。/u);
	await click("重試");
	await until(() => bodyText().includes("通報詳情"), "重試後出現內容");

	await openPage(
		failingFirst(
			transportWith({ getReport: () => ({ report: report({}) }) }),
			1,
		),
	);
	assert.match(bodyText(), /通報讀取失敗/u);
	assert.ok(buttonByText("重試"));
});

test("按鈕只看 can_*：待分案的通報有分案與設定結果；沒有的就隱藏，不顯示原因", async () => {
	await openPage(
		transportWith({
			getReport: () => ({
				report: report({ canTriage: true, canResolve: true }),
			}),
		}),
	);
	assert.deepEqual(buttons(), ["分案", "設定結果"]);
});

test("按鈕只看 can_*：案件裡的通報有設定結果、移到其他案件與查看案件，沒有分案", async () => {
	await openPage(
		transportWith({
			getReport: () => ({
				report: report({
					state: ReportState.IN_CASE,
					caseId: "c-1",
					canResolve: true,
					canMove: true,
				}),
			}),
		}),
	);
	assert.deepEqual(buttons(), ["設定結果", "移到其他案件", "查看案件"]);
	await click("查看案件");
	assert.deepEqual(navigation.pushed, ["/admin/violation-reports/cases/c-1"]);
});

test("按鈕只看 can_*：什麼都不能做的通報沒有操作按鈕，也沒有任何「為什麼不能」的說明", async () => {
	await openPage(
		transportWith({
			getReport: () => ({
				report: report({
					state: ReportState.RESOLVED,
					outcome: ReportOutcome.NO_VIOLATION,
				}),
			}),
		}),
	);
	for (const label of ACTIONS) {
		assert.ok(buttonByText(label) === undefined, `${label} 不該出現`);
	}
	assert.doesNotMatch(bodyText(), /不能|無法/u);
});

test("返回鈕回到清單", async () => {
	await openPage(transportWith({ getReport: () => ({ report: report({}) }) }));
	const back = dom.window.document.querySelector(
		'button[aria-label="返回列表"]',
	);
	assert.ok(back);
	await React.act(async () => {
		(back as HTMLElement).click();
	});
	assert.deepEqual(navigation.pushed, ["/admin/violation-reports"]);
});

test("通報內容、檢舉人、佐證與處理：登入的檢舉人是買家 ID，沒填的欄位是「-」，沒有佐證說「沒有佐證」", async () => {
	await openPage(
		transportWith({
			getReport: () => ({
				report: report({
					reporterBuyerId: "50007",
					productId: "",
					orderNo: "",
					description: "吃了便當之後腹瀉",
				}),
			}),
		}),
	);
	const text = bodyText();
	assert.match(
		text,
		/查看店家・通報 0190b000-0000-7000-8000-000000000001・建立於 2026\/10\/18 10:00/u,
	);
	assert.match(text, /通報內容/u);
	assert.match(text, /檢舉人選的類別食品安全/u);
	assert.match(text, /商品 ID-/u);
	assert.match(text, /訂單編號-/u);
	assert.match(text, /吃了便當之後腹瀉/u);
	assert.match(text, /買家 ID 50007/u);
	assert.match(text, /沒有佐證/u);
	assert.match(text, /狀態待分案/u);
	assert.match(text, /平台判定類別-/u);
	assert.match(text, /給檢舉人的結果說明沒有說明/u);
	assert.doesNotMatch(text, /補充/u, "沒有補充就沒有補充區塊");
});

test("匿名的檢舉人列出有提供的聯絡方式；補充標示比對方式、列出丟掉的次數；佐證分原通報與補充", async () => {
	const evidence = (position: number) =>
		create(EvidenceSchema, {
			position,
			viewUri: `/api/files/admin/seller/reports/${REPORT_ID}/evidence/${position}`,
		});
	await openPage(
		transportWith({
			getReport: () => ({
				report: report({
					reporterBuyerId: "",
					contact: create(ReporterContactSchema, {
						phone: "0912345678",
						name: "王小明",
					}),
					supplements: [
						create(ReportSupplementSchema, {
							id: "s1",
							description: "隔天仍不舒服",
							createTime: ts(T0),
							matchedBy: SupplementMatch.PHONE,
							evidencePositions: [1],
						}),
						create(ReportSupplementSchema, {
							id: "s2",
							description: "附上診斷書",
							createTime: ts(T0),
							matchedBy: SupplementMatch.BOTH,
						}),
					],
					supplementCount: 2,
					discardedSubmissionCount: 3,
					evidence: [evidence(0), evidence(1)],
				}),
			}),
		}),
	);
	const text = bodyText();
	assert.match(text, /匿名/u);
	assert.match(text, /電話0912345678/u);
	assert.match(text, /姓名王小明/u);
	assert.doesNotMatch(text, /聯絡信箱/u, "沒提供的欄位不列");
	assert.match(text, /補充 1・2026\/10\/18 10:00/u);
	assert.match(
		text,
		/以電話比對到這筆通報的匿名送出，送出者不一定是原檢舉人。/u,
	);
	assert.match(text, /補充 2・2026\/10\/18 10:00/u);
	assert.equal(
		text.match(/比對到這筆通報的匿名送出/gu)?.length,
		1,
		"電話與聯絡信箱都相同時不提醒",
	);
	assert.match(text, /另有 3 次重複送出因補充已達上限，內容沒有存下。/u);
	const links = [
		...dom.window.document.querySelectorAll(
			'section[aria-labelledby="report-evidence-title"] .card-body a',
		),
	].map((a) => a.textContent);
	assert.deepEqual(links, ["佐證 1", "補充 1 的佐證 1"]);
});

test("聯絡方式已清除時說明清除的時間", async () => {
	await openPage(
		transportWith({
			getReport: () => ({
				report: report({ reporterBuyerId: "", contactEraseTime: ts(T0) }),
			}),
		}),
	);
	assert.match(bodyText(), /聯絡方式已於 2026\/10\/18 10:00 依保留期限清除/u);
});

test("待分案的通報顯示分案期限：日期時間加相對時間，逾期時是「已逾期」", async () => {
	const due = new Date(Date.now() - 90 * 60 * 1000);
	await openPage(
		transportWith({
			getReport: () => ({
				report: report({
					createTime: ts(new Date(due.getTime() - 4 * 3600 * 1000)),
					triageDeadline: {
						startTime: ts(new Date(due.getTime() - 4 * 3600 * 1000)),
						dueTime: ts(due),
					},
				}),
			}),
		}),
	);
	assert.match(
		bodyText(),
		/分案期限：\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}（已逾期 1 小時 30 分）/u,
	);
	assert.ok(dom.window.document.querySelector(".adm-sla--danger"));
});

test("分案：送出請求帶讀到的 etag，成功後關閉對話框並重讀通報，成功時沒有另外的提示", async () => {
	let current = report({
		canTriage: true,
		etag: "r-1",
		triageTargets: [target(PolicyCategory.FRAUD)],
	});
	let reads = 0;
	const triaged: Req[] = [];
	await openPage(
		transportWith({
			getReport: () => {
				reads++;
				return { report: current };
			},
			triageReport: (req) => {
				triaged.push(req);
				current = report({
					state: ReportState.IN_CASE,
					caseId: "c-new",
					assessedCategory: PolicyCategory.FRAUD,
					etag: "r-2",
				});
				return { report: current };
			},
		}),
	);
	await click("分案");
	await until(() => maybeById("triage-report-category") !== null, "對話框");
	await React.act(async () => {
		setValue(byId("triage-report-category"), String(PolicyCategory.FRAUD));
	});
	await React.act(async () => {
		setValue(byId("triage-report-severity"), String(ViolationSeverity.MEDIUM));
	});
	await click("確認分案");
	await until(() => maybeById("triage-report-category") === null, "對話框關閉");
	assert.equal(triaged.length, 1);
	assert.equal(triaged[0].etag, "r-1");
	assert.equal(triaged[0].assessedCategory, PolicyCategory.FRAUD);
	const sent = triaged[0].target as {
		target: { case: string; value: { severity: number } };
	};
	assert.equal(sent.target.case, "openNew");
	assert.equal(
		sent.target.value.severity,
		ViolationSeverity.MEDIUM,
		"去處是伺服器給的 triage_targets：這個類別沒有進行中的案件，所以開新案件並送嚴重度",
	);
	await until(
		() => reads >= 2 && buttonByText("分案") === undefined,
		"重讀後分案鈕消失",
	);
	assert.doesNotMatch(bodyText(), /已分案|成功/u);
});

test("版本衝突：重讀通報、對話框留在原地保留輸入並顯示衝突句；伺服器的句子照原樣顯示", async () => {
	let reads = 0;
	let mode: "conflict" | "message" = "conflict";
	await openPage(
		transportWith({
			getReport: () => {
				reads++;
				return {
					report: report({
						canTriage: true,
						etag: `r-${reads}`,
						triageTargets: [target(PolicyCategory.FRAUD)],
					}),
				};
			},
			triageReport: () => {
				throw mode === "conflict"
					? versionConflict()
					: withMessage(Code.FailedPrecondition, "這筆通報已經分案。");
			},
		}),
	);
	await click("分案");
	await until(() => maybeById("triage-report-category") !== null, "對話框");
	await React.act(async () => {
		setValue(byId("triage-report-category"), String(PolicyCategory.FRAUD));
	});
	await React.act(async () => {
		setValue(byId("triage-report-severity"), String(ViolationSeverity.LOW));
	});
	await click("確認分案");
	await until(
		() =>
			bodyText().includes(
				"這筆通報剛被更新，頁面已重新載入；請確認最新狀態後再操作。",
			),
		"衝突句",
	);
	assert.equal(reads, 2, "衝突時重讀一次");
	assert.equal(
		byId<HTMLSelectElement>("triage-report-category").value,
		String(PolicyCategory.FRAUD),
	);
	mode = "message";
	await click("確認分案");
	await until(() => bodyText().includes("這筆通報已經分案。"), "伺服器的句子");
});

test("通報有還沒看過的補充：頁面顯示補充時帶讀到的 etag 記下已讀一次，之後不再呼叫；沒有未看過的補充就不呼叫", async () => {
	const marked: Req[] = [];
	let seen = false;
	await openPage(
		transportWith({
			getReport: () => ({
				report: report({
					etag: "r-1",
					supplementCount: 1,
					hasUnseenSupplements: !seen,
					supplements: [
						{ id: "s-1", description: "又吃到一次", createTime: ts(T0) },
					],
				}),
			}),
			markReportSupplementsSeen: (req) => {
				marked.push(req);
				seen = true;
				return { report: report({ etag: "r-1" }) };
			},
		}),
	);
	await until(() => marked.length === 1, "記下已讀");
	assert.equal(marked[0].id, REPORT_ID);
	assert.equal(marked[0].etag, "r-1");
	await until(() => seen, "已讀記下之後通報重讀");
	await React.act(async () => {
		await new Promise((r) => setTimeout(r, 50));
	});
	assert.equal(marked.length, 1, "已讀之後不重複呼叫");

	marked.length = 0;
	await openPage(
		transportWith({
			getReport: () => ({
				report: report({ supplementCount: 1, hasUnseenSupplements: false }),
			}),
			markReportSupplementsSeen: (req) => {
				marked.push(req);
				return { report: report({}) };
			},
		}),
	);
	assert.equal(marked.length, 0, "沒有未看過的補充不呼叫");
});

test("記下已讀遇到版本衝突（讀到之後又有新的補充）：重讀通報，用新的 etag 再記一次", async () => {
	const marked: Req[] = [];
	let reads = 0;
	await openPage(
		transportWith({
			getReport: () => {
				reads++;
				return {
					report: report({
						etag: reads === 1 ? "r-1" : "r-2",
						supplementCount: reads,
						hasUnseenSupplements: marked.length < 2,
					}),
				};
			},
			markReportSupplementsSeen: (req) => {
				marked.push(req);
				if (req.etag === "r-1") throw versionConflict("report");
				return { report: report({ etag: "r-2" }) };
			},
		}),
	);
	await until(() => marked.length === 2, "用新的 etag 再記一次");
	assert.deepEqual(
		marked.map((m) => m.etag),
		["r-1", "r-2"],
	);
});

test("分案要開新案件而類別剛有進行中的案件（conflictTarget 是 category_case_opened）時說類別剛有案件，要加入的案件過時（target_case）說要移入的案件，通報自己過時才說通報", async () => {
	let target_: "category_case_opened" | "target_case" | "report" =
		"category_case_opened";
	await openPage(
		transportWith({
			getReport: () => ({
				report: report({
					canTriage: true,
					triageTargets: [target(PolicyCategory.FRAUD)],
				}),
			}),
			triageReport: () => {
				throw versionConflict(target_);
			},
		}),
	);
	await click("分案");
	await until(() => maybeById("triage-report-category") !== null, "對話框");
	await React.act(async () => {
		setValue(byId("triage-report-category"), String(PolicyCategory.FRAUD));
	});
	await React.act(async () => {
		setValue(byId("triage-report-severity"), String(ViolationSeverity.LOW));
	});
	await click("確認分案");
	await until(
		() => bodyText().includes("這個類別剛有進行中的案件，請重新選擇。"),
		"類別剛有案件的衝突句",
	);
	assert.doesNotMatch(
		bodyText(),
		/這筆通報剛被更新|這個案件剛被更新|要移入的案件剛被更新/u,
	);
	target_ = "target_case";
	await click("確認分案");
	await until(
		() => bodyText().includes("要移入的案件剛被更新，請重新選擇。"),
		"要移入的案件的衝突句",
	);
	target_ = "report";
	await click("確認分案");
	await until(
		() => bodyText().includes("這筆通報剛被更新，頁面已重新載入"),
		"通報的衝突句",
	);
});

test("設定結果：選項來自伺服器，送出後關閉對話框", async () => {
	const resolved: Req[] = [];
	await openPage(
		transportWith({
			getReport: () => ({
				report: report({
					canResolve: true,
					availableOutcomes: [
						outcome(ReportOutcome.NO_VIOLATION, true),
						outcome(ReportOutcome.INSUFFICIENT, true),
					],
					etag: "r-4",
				}),
			}),
			listMeasures: () => ({ measures: [] }),
			resolveReport: (req) => {
				resolved.push(req);
				return { report: report({}) };
			},
		}),
	);
	await click("設定結果");
	await until(() => maybeById("resolve-report-outcome") !== null, "對話框");
	await React.act(async () => {
		setValue(
			byId("resolve-report-outcome"),
			String(ReportOutcome.INSUFFICIENT),
		);
	});
	await until(() => buttonByText("確認設定")?.disabled === false, "措施讀完");
	await click("確認設定");
	await until(() => maybeById("resolve-report-outcome") === null, "對話框關閉");
	assert.equal(resolved[0].outcome, ReportOutcome.INSUFFICIENT);
	assert.equal(resolved[0].etag, "r-4");
});

test("移到其他案件：原案件因為通報都移走而關閉時，頁面顯示橫幅", async () => {
	await openPage(
		transportWith({
			getReport: () => ({
				report: report({
					state: ReportState.IN_CASE,
					caseId: "c-1",
					assessedCategory: PolicyCategory.FOOD_SAFETY,
					canMove: true,
					triageTargets: [target(PolicyCategory.FRAUD)],
				}),
			}),
			moveReport: () => ({ report: report({}), sourceCaseClosed: true }),
		}),
	);
	await click("移到其他案件");
	await until(() => maybeById("move-report-category") !== null, "對話框");
	await React.act(async () => {
		setValue(byId("move-report-category"), String(PolicyCategory.FRAUD));
	});
	await React.act(async () => {
		setValue(byId("move-report-severity"), String(ViolationSeverity.LOW));
	});
	await click("確認移動");
	await until(
		() => bodyText().includes("原案件的通報都已移走，原案件已自動關閉。"),
		"橫幅",
	);
	assert.ok(maybeById("move-report-category") === null);
});

test("Connect 錯誤沒有伺服器的句子時用該 code 的預設文字", async () => {
	await openPage(
		transportWith({
			getReport: () => {
				throw new ConnectError("down", Code.Unavailable);
			},
		}),
	);
	assert.ok(bodyText().includes(defaultErrorTextZhTW(Code.Unavailable)));
});
