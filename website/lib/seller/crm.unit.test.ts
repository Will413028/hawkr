import { create } from "@bufbuild/protobuf";
import { timestampFromMs } from "@bufbuild/protobuf/wkt";
import {
	Code,
	ConnectError,
	createClient,
	createRouterTransport,
	type ServiceImpl,
} from "@connectrpc/connect";
import {
	CrmAppService,
	CustomerSchema,
	CustomerSort,
	CustomerType,
	type ListCustomersRequest,
	type RemoveCustomerTagRequest,
	type UpdateCustomerRequest,
	type UpdateTagRequest,
} from "@hawkr/api/gen/hawkr/crm/v1alpha1/crm_pb";
import { describe, expect, it } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import {
	addCustomerTags,
	createTag,
	customerSortText,
	customerTypeClass,
	customerTypeText,
	deleteTag,
	getCustomer,
	listAllCustomers,
	listCustomers,
	listTags,
	normalizeCustomerSort,
	removeCustomerTag,
	renameTag,
	updateCustomerType,
} from "./crm";

function customer(buyerId: string, over: Record<string, unknown> = {}) {
	return create(CustomerSchema, {
		buyerId,
		display: { nickname: `買家${buyerId}` },
		type: CustomerType.VIP,
		totalSpent: BigInt(1200),
		orderCount: 3,
		firstOrderTime: timestampFromMs(1_700_000_000_000),
		tags: [{ id: "t1", name: "回購" }],
		...over,
	});
}

function harness(handlers: Partial<ServiceImpl<typeof CrmAppService>> = {}) {
	const seen = {
		list: [] as ListCustomersRequest[],
		update: [] as UpdateCustomerRequest[],
		updateTag: [] as UpdateTagRequest[],
		remove: [] as RemoveCustomerTagRequest[],
		add: [] as { storeId: string; buyerId: string; tagNames: string[] }[],
		del: [] as string[],
	};
	const transport = createRouterTransport((r) =>
		r.service(CrmAppService, {
			listCustomers(req) {
				seen.list.push(req);
				return { customers: [customer("1")], nextPageToken: "" };
			},
			getCustomer: () => ({ customer: customer("1") }),
			updateCustomer(req) {
				seen.update.push(req);
				return { customer: customer("1", { type: req.customer?.type }) };
			},
			listTags: () => ({ tags: [{ id: "t1", name: "回購" }] }),
			createTag: (req) => ({ tag: { id: "t9", name: req.name } }),
			updateTag(req) {
				seen.updateTag.push(req);
				return { tag: req.tag };
			},
			deleteTag(req) {
				seen.del.push(req.tagId);
				return {};
			},
			addCustomerTags(req) {
				seen.add.push({
					storeId: req.storeId,
					buyerId: req.buyerId,
					tagNames: req.tagNames,
				});
				return {
					customer: customer("1", {
						tags: req.tagNames.map((name, i) => ({ id: `n${i}`, name })),
					}),
				};
			},
			removeCustomerTag(req) {
				seen.remove.push(req);
				return { customer: customer("1", { tags: [] }) };
			},
			...handlers,
		}),
	);
	return { client: () => createClient(CrmAppService, transport), seen };
}

describe("listCustomers", () => {
	it("maps the filters, sort and paging to the request and the customer to the view", async () => {
		const { client, seen } = harness();
		const page = await listCustomers(
			"7",
			{
				type: CustomerType.VIP,
				tagId: "t1",
				sort: CustomerSort.ORDER_COUNT,
				descending: true,
				pageSize: 20,
				pageToken: "tok",
			},
			client,
		);
		expect(seen.list[0]).toMatchObject({
			storeId: "7",
			type: CustomerType.VIP,
			tagId: "t1",
			sort: CustomerSort.ORDER_COUNT,
			descending: true,
			pageSize: 20,
			pageToken: "tok",
		});
		expect(page.nextPageToken).toBe("");
		expect(page.customers[0]).toMatchObject({
			id: "1",
			name: "買家1",
			deleted: false,
			customerType: CustomerType.VIP,
			totalSpent: 1200,
			orderCount: 3,
			firstOrderAt: 1_700_000_000_000,
			tags: [{ id: "t1", name: "回購" }],
		});
	});

	it("does not send type when no classification filter is chosen (UNSPECIFIED would be invalid_argument)", async () => {
		const { client, seen } = harness();
		await listCustomers("7", { type: CustomerType.UNSPECIFIED }, client);
		await listCustomers("7", {}, client);
		expect(seen.list[0].type).toBeUndefined();
		expect(seen.list[1].type).toBeUndefined();
		expect(seen.list[1].tagId).toBeUndefined();
	});

	it("a read failure rejects instead of resolving to an empty page", async () => {
		const { client } = harness({
			listCustomers: () => {
				throw new ConnectError("down", Code.Unavailable);
			},
		});
		await expect(listCustomers("7", {}, client)).rejects.toMatchObject({
			code: Code.Unavailable,
		});
	});

	it("an empty store resolves to an empty page", async () => {
		const { client } = harness({
			listCustomers: () => ({ customers: [], nextPageToken: "" }),
		});
		const page = await listCustomers("7", {}, client);
		expect(page.customers).toEqual([]);
	});
});

describe("listAllCustomers", () => {
	it("follows next_page_token to the end", async () => {
		const tokens: string[] = [];
		const { client } = harness({
			listCustomers(req) {
				tokens.push(req.pageToken);
				return req.pageToken === ""
					? { customers: [customer("1")], nextPageToken: "p2" }
					: { customers: [customer("2")], nextPageToken: "" };
			},
		});
		const all = await listAllCustomers("7", {}, client);
		expect(all.map((c) => c.id)).toEqual(["1", "2"]);
		expect(tokens).toEqual(["", "p2"]);
	});

	it("fails as a whole when a later page fails", async () => {
		const { client } = harness({
			listCustomers(req) {
				if (req.pageToken === "") {
					return { customers: [customer("1")], nextPageToken: "p2" };
				}
				throw new ConnectError("down", Code.Unavailable);
			},
		});
		await expect(listAllCustomers("7", {}, client)).rejects.toBeInstanceOf(
			ConnectError,
		);
	});
});

describe("customer and tag mutations", () => {
	it("getCustomer maps a deleted buyer without a nickname", async () => {
		const { client } = harness({
			getCustomer: () => ({
				customer: customer("1", { display: { deleted: true } }),
			}),
		});
		const c = await getCustomer("7", "1", client);
		expect(c.deleted).toBe(true);
		expect(c.name).toBe("");
	});

	it("updateCustomerType sends the type with update_mask [type]", async () => {
		const { client, seen } = harness();
		const c = await updateCustomerType("7", "1", CustomerType.NORMAL, client);
		expect(seen.update[0].customer?.buyerId).toBe("1");
		expect(seen.update[0].customer?.type).toBe(CustomerType.NORMAL);
		expect(seen.update[0].updateMask?.paths).toEqual(["type"]);
		expect(c.customerType).toBe(CustomerType.NORMAL);
	});

	it("renameTag sends update_mask [name]", async () => {
		const { client, seen } = harness();
		const tag = await renameTag("7", "t1", "新名", client);
		expect(seen.updateTag[0].updateMask?.paths).toEqual(["name"]);
		expect(tag).toEqual({ id: "t1", name: "新名" });
	});

	it("tag pool CRUD", async () => {
		const { client, seen } = harness();
		expect(await listTags("7", client)).toEqual([{ id: "t1", name: "回購" }]);
		expect(await createTag("7", "新", client)).toEqual({
			id: "t9",
			name: "新",
		});
		await deleteTag("7", "t1", client);
		expect(seen.del).toEqual(["t1"]);
	});

	it("addCustomerTags returns the customer with the tags after the attach", async () => {
		const { client, seen } = harness();
		const c = await addCustomerTags("7", "1", ["a", "b"], client);
		expect(seen.add[0]).toEqual({
			storeId: "7",
			buyerId: "1",
			tagNames: ["a", "b"],
		});
		expect(c.tags.map((t) => t.name)).toEqual(["a", "b"]);
	});

	it("removeCustomerTag sends the tag id and returns the customer", async () => {
		const { client, seen } = harness();
		const c = await removeCustomerTag("7", "1", "t1", client);
		expect(seen.remove[0]).toMatchObject({
			storeId: "7",
			buyerId: "1",
			tagId: "t1",
		});
		expect(c.tags).toEqual([]);
	});

	it("a failed mutation rejects with the server error", async () => {
		const { client } = harness({
			addCustomerTags: () => {
				throw new ConnectError("limit", Code.FailedPrecondition);
			},
		});
		await expect(
			addCustomerTags("7", "1", ["x"], client),
		).rejects.toMatchObject({ code: Code.FailedPrecondition });
	});
});

describe("customer type presentation", () => {
	it("takes the labels from the catalog and shows unknown for unspecified", () => {
		const ut = uiTextFor("zh-TW");
		expect(customerTypeText(ut, CustomerType.VIP)).toBe("VIP");
		expect(customerTypeText(ut, CustomerType.NORMAL)).toBe("一般客戶");
		expect(customerTypeText(ut, CustomerType.UNSPECIFIED)).toBe(
			ut({ key: "common.unknown" }),
		);
		expect(customerSortText(ut, CustomerSort.ORDER_COUNT)).toBe("訂單數");
		expect(customerSortText(ut, CustomerSort.UNSPECIFIED)).toBe("首購時間");
		expect(normalizeCustomerSort(CustomerSort.UNSPECIFIED)).toBe(
			CustomerSort.FIRST_ORDER_TIME,
		);
		expect(normalizeCustomerSort(CustomerSort.TOTAL_SPENT)).toBe(
			CustomerSort.TOTAL_SPENT,
		);
		expect(customerTypeClass(CustomerType.VIP)).toBe("is-vip");
	});
});
