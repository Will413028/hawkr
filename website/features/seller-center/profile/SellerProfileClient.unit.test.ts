// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	BadRequestSchema,
	ErrorInfoSchema,
	LocalizedMessageSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import {
	GetMyStoreResponseSchema,
	KYCFlowSchema,
	KYCFlowStatus,
	SellerCapability,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	cleanup,
	fireEvent,
	render as renderWithoutIntl,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

type Init = Parameters<typeof create<typeof GetMyStoreResponseSchema>>[1];

// 店家 query 的快取：換掉它就像 invalidate 之後 refetch 帶回新資料，訂閱者會重新渲染。
const cache = vi.hoisted(() => {
	const listeners = new Set<() => void>();
	let current: unknown = null;
	return {
		get: () => current,
		set: (next: unknown) => {
			current = next;
			for (const listener of listeners) listener();
		},
		subscribe: (listener: () => void) => {
			listeners.add(listener);
			return () => listeners.delete(listener);
		},
	};
});

const mocks = vi.hoisted(() => ({
	flow: null as unknown,
	invalidate: vi.fn(),
	updateMyStore: vi.fn(),
	fetchMyStore: vi.fn(),
	brandingProps: null as null | {
		onError: (m: string, cause?: unknown) => Promise<void> | void;
		readOnly?: boolean;
	},
	toastError: vi.fn(),
	toastSuccess: vi.fn(),
}));

vi.mock("@/lib/hooks/useSellerSelf", async () => {
	const react = await import("react");
	return {
		useSellerSelf: () => ({
			data: react.useSyncExternalStore(cache.subscribe, cache.get, cache.get),
			loading: false,
			error: null,
			refresh: async () => {},
		}),
		useMyKycFlow: () => ({ flow: mocks.flow, loading: false, error: null }),
		useInvalidateSellerSelf: () => mocks.invalidate,
	};
});
vi.mock("@/lib/hooks/useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "1", activeRole: "seller" }),
}));
vi.mock("@/lib/profile/repo", () => ({
	fetchProfile: async () => null,
}));
vi.mock("@/lib/feedback/toast", () => ({
	toast: { error: mocks.toastError, success: mocks.toastSuccess },
}));
vi.mock("@/features/seller-center/profile/actions", () => ({
	revalidateOwnSellerProfile: async () => {},
}));
vi.mock("@/lib/seller-data/scope", () => ({
	useSellerDataScope: () => ({
		mode: "seller",
		ownerStoreId: null,
		hasCapability: () => true,
		confirmMutation: async () => ({ ok: true }),
		repos: {
			storefront: {
				updateMyStore: mocks.updateMyStore,
				fetchMyStore: mocks.fetchMyStore,
			},
		},
	}),
}));
vi.mock("@/features/seller/components/SellerProfileBranding", () => ({
	SellerProfileBranding: (props: {
		onError: (m: string, cause?: unknown) => Promise<void> | void;
		readOnly?: boolean;
	}) => {
		mocks.brandingProps = props;
		return null;
	},
}));
vi.mock("@/features/seller-center/profile/SellerShareCardPreview", () => ({
	default: () => null,
}));
vi.mock("@/features/seller-center/profile/CloseStoreSection", () => ({
	default: () => null,
}));
vi.mock("@/components/shared/UploadLoadingIndicator", () => ({
	default: () => null,
}));
vi.mock("@/features/account/screens/addresses/TownFields", () => ({
	TownFields: () => null,
}));
vi.mock("@/components/ui", () => ({
	Input: ({
		label,
		value,
		onChange,
		readOnly,
		errorMessage,
	}: {
		label: string;
		value: string;
		onChange: React.ChangeEventHandler<HTMLInputElement>;
		readOnly?: boolean;
		errorMessage?: string;
	}) =>
		React.createElement(
			React.Fragment,
			null,
			React.createElement("input", {
				"aria-label": label,
				value,
				onChange,
				readOnly,
			}),
			errorMessage
				? React.createElement("span", { role: "alert" }, errorMessage)
				: null,
		),
	Textarea: ({
		label,
		value,
		onChange,
		readOnly,
	}: {
		label: string;
		value: string;
		onChange: React.ChangeEventHandler<HTMLTextAreaElement>;
		readOnly?: boolean;
	}) =>
		React.createElement("textarea", {
			"aria-label": label,
			value,
			onChange,
			readOnly,
		}),
}));
vi.mock("@/components/ui/Button", () => ({
	default: ({
		children,
		type,
		disabled,
	}: {
		children: React.ReactNode;
		type?: "submit" | "button";
		disabled?: boolean;
	}) =>
		React.createElement(
			"button",
			{ type: type === "submit" ? "submit" : "button", disabled },
			children,
		),
}));

import { activeStatuses } from "@/lib/seller/capability-fixtures";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import SellerProfileClient from "./SellerProfileClient";

const render = (ui: React.ReactElement) =>
	renderWithoutIntl(ui, { wrapper: IntlTestWrapper });

function store(init: Init) {
	return create(GetMyStoreResponseSchema, init);
}

const FIRST: Init = {
	capabilities: activeStatuses([SellerCapability.STOREFRONT_WRITE]),
	store: {
		id: "1",
		storeName: "山田農場",
		description: "原本的介紹",
		etag: "e1",
		address: { townCode: "66000290", addressDetail: "東關路一段 100 號" },
	},
};

function mount() {
	const client = new QueryClient();
	return render(
		React.createElement(
			QueryClientProvider,
			{ client },
			React.createElement(SellerProfileClient),
		),
	);
}

function conflict() {
	return new ConnectError("stale", Code.Aborted, undefined, [
		{
			desc: ErrorInfoSchema,
			value: {
				reason: ErrorReason[ErrorReason.VERSION_CONFLICT],
				domain: "hawkr",
			},
		},
		{
			desc: LocalizedMessageSchema,
			value: { locale: "zh-TW", message: "資料已被更新" },
		},
	]);
}

async function save() {
	const button = await screen.findByRole("button", { name: "儲存商店資訊" });
	await waitFor(() =>
		expect((button as HTMLButtonElement).disabled).toBe(false),
	);
	fireEvent.click(button);
}

beforeEach(() => {
	cache.set(store(FIRST));
	mocks.flow = null;
	mocks.invalidate.mockReset();
	mocks.invalidate.mockResolvedValue(undefined);
	mocks.updateMyStore.mockReset();
	mocks.fetchMyStore.mockReset();
	mocks.fetchMyStore.mockImplementation(async () => cache.get());
	mocks.brandingProps = null;
	mocks.toastError.mockReset();
	mocks.toastSuccess.mockReset();
});

afterEach(() => cleanup());

describe("SellerProfileClient writes", () => {
	it("sends the etag it loaded with only the edited field", async () => {
		mocks.updateMyStore.mockResolvedValue({ etag: "e2" });
		mount();

		fireEvent.change(await screen.findByLabelText("商店名稱"), {
			target: { value: "改過的店名" },
		});
		await save();

		await waitFor(() => expect(mocks.updateMyStore).toHaveBeenCalledOnce());
		expect(mocks.updateMyStore.mock.calls[0]?.[0]).toEqual({
			etag: "e1",
			storeName: "改過的店名",
		});
		await waitFor(() => expect(mocks.toastSuccess).toHaveBeenCalled());
	});

	it("writes the contact name, mobile and support email as store fields", async () => {
		mocks.updateMyStore.mockResolvedValue({ etag: "e2" });
		mount();

		fireEvent.change(await screen.findByLabelText("聯絡人姓名"), {
			target: { value: "王小明" },
		});
		fireEvent.change(screen.getByLabelText("聯絡人手機"), {
			target: { value: "0912345678" },
		});
		fireEvent.change(screen.getByLabelText("客服電子信箱"), {
			target: { value: "shop@example.com" },
		});
		await save();

		await waitFor(() => expect(mocks.updateMyStore).toHaveBeenCalledOnce());
		expect(mocks.updateMyStore.mock.calls[0]?.[0]).toEqual({
			etag: "e1",
			contactName: "王小明",
			contactMobile: "0912345678",
			supportEmail: "shop@example.com",
		});
	});

	it("does not send a malformed contact mobile or email", async () => {
		mount();

		fireEvent.change(await screen.findByLabelText("客服電子信箱"), {
			target: { value: "not-an-email" },
		});
		await save();

		expect(mocks.updateMyStore).not.toHaveBeenCalled();
	});

	it("shows the server's required-field error on the cleared contact field", async () => {
		mocks.updateMyStore.mockRejectedValue(
			new ConnectError("必填", Code.InvalidArgument, undefined, [
				{
					desc: BadRequestSchema,
					value: {
						fieldViolations: [
							{
								field: "store.contact_name",
								description: "必填",
								localizedMessage: { locale: "zh-TW", message: "必填" },
							},
						],
					},
				},
			]),
		);
		mount();

		fireEvent.change(await screen.findByLabelText("聯絡人姓名"), {
			target: { value: "" },
		});
		fireEvent.change(screen.getByLabelText("商店名稱"), {
			target: { value: "改過的店名" },
		});
		await save();

		await waitFor(() => expect(mocks.updateMyStore).toHaveBeenCalledOnce());
		expect(await screen.findByText("必填")).toBeTruthy();
		fireEvent.change(screen.getByLabelText("聯絡人姓名"), {
			target: { value: "王" },
		});
		await waitFor(() => expect(screen.queryByText("必填")).toBeNull());
	});

	it("uses the etag from its own write for the next write, not a refetched one", async () => {
		mocks.updateMyStore.mockResolvedValue({ etag: "e2" });
		mount();

		fireEvent.change(await screen.findByLabelText("商店名稱"), {
			target: { value: "第一次" },
		});
		await save();
		await waitFor(() => expect(mocks.updateMyStore).toHaveBeenCalledTimes(1));
		// 背景 refetch 帶回別人寫入的版本：不可被當成自己的版本。
		cache.set(
			store({
				...FIRST,
				store: { ...FIRST.store, etag: "e9", storeName: "第一次" },
			}),
		);
		await waitFor(() => expect(mocks.toastSuccess).toHaveBeenCalled());

		fireEvent.change(screen.getByLabelText("商店名稱"), {
			target: { value: "第二次" },
		});
		await save();
		await waitFor(() => expect(mocks.updateMyStore).toHaveBeenCalledTimes(2));
		expect(mocks.updateMyStore.mock.calls[1]?.[0]).toMatchObject({
			etag: "e2",
			storeName: "第二次",
		});
	});

	it("keeps the typed input on a version conflict, shows the latest store, and retries with its etag", async () => {
		mocks.updateMyStore
			.mockRejectedValueOnce(conflict())
			.mockResolvedValueOnce({ etag: "e3" });
		// 另一個 session 在這段期間改了介紹，版本推進到 e2。
		const latest = store({
			...FIRST,
			store: { ...FIRST.store, description: "別處改過的介紹", etag: "e2" },
		});
		mocks.fetchMyStore.mockResolvedValue(latest);
		mocks.invalidate.mockImplementation(async () => cache.set(latest));
		mount();

		const name = await screen.findByLabelText("商店名稱");
		fireEvent.change(name, { target: { value: "衝突時輸入的店名" } });
		await save();

		await waitFor(() => expect(mocks.toastError).toHaveBeenCalled());
		expect(mocks.toastError.mock.calls[0]?.[0]).toBe(
			"商店資料已在其他地方更新",
		);
		// 使用者輸入保留，沒改的欄位顯示最新版本。
		expect((screen.getByLabelText("商店名稱") as HTMLInputElement).value).toBe(
			"衝突時輸入的店名",
		);
		await waitFor(() =>
			expect(
				(screen.getByLabelText("商店介紹") as HTMLTextAreaElement).value,
			).toBe("別處改過的介紹"),
		);

		await save();
		await waitFor(() => expect(mocks.updateMyStore).toHaveBeenCalledTimes(2));
		expect(mocks.updateMyStore.mock.calls[0]?.[0].etag).toBe("e1");
		expect(mocks.updateMyStore.mock.calls[1]?.[0]).toEqual({
			etag: "e2",
			storeName: "衝突時輸入的店名",
		});
	});

	it("retries with the latest etag after a conflict even when the cache already held it", async () => {
		mocks.updateMyStore
			.mockRejectedValueOnce(conflict())
			.mockResolvedValueOnce({ etag: "e10" });
		mount();

		fireEvent.change(await screen.findByLabelText("商店名稱"), {
			target: { value: "輸入的店名" },
		});
		// 視窗聚焦的 refetch 早已把快取換成最新版本，衝突後的 invalidate 不會再改變它。
		cache.set(store({ ...FIRST, store: { ...FIRST.store, etag: "e9" } }));
		await save();
		await waitFor(() => expect(mocks.toastError).toHaveBeenCalled());
		await save();

		await waitFor(() => expect(mocks.updateMyStore).toHaveBeenCalledTimes(2));
		expect(mocks.updateMyStore.mock.calls[0]?.[0].etag).toBe("e1");
		expect(mocks.updateMyStore.mock.calls[1]?.[0]).toEqual({
			etag: "e9",
			storeName: "輸入的店名",
		});
	});

	it("adopts the latest etag when a logo or cover write conflicts, keeping typed input", async () => {
		mocks.updateMyStore.mockResolvedValue({ etag: "e12" });
		mount();
		const name = await screen.findByLabelText("商店名稱");
		fireEvent.change(name, { target: { value: "輸入的店名" } });
		cache.set(store({ ...FIRST, store: { ...FIRST.store, etag: "e11" } }));

		await React.act(async () => {
			await mocks.brandingProps?.onError("上傳失敗", conflict());
		});
		expect(mocks.toastError.mock.calls[0]?.[0]).toBe(
			"商店資料已在其他地方更新",
		);
		expect((screen.getByLabelText("商店名稱") as HTMLInputElement).value).toBe(
			"輸入的店名",
		);
		await save();

		await waitFor(() => expect(mocks.updateMyStore).toHaveBeenCalledOnce());
		expect(mocks.updateMyStore.mock.calls[0]?.[0]).toEqual({
			etag: "e11",
			storeName: "輸入的店名",
		});
	});

	it("shows the server message for other failures and keeps the input", async () => {
		mocks.updateMyStore.mockRejectedValue(
			new ConnectError("no", Code.InvalidArgument, undefined, [
				{
					desc: LocalizedMessageSchema,
					value: { locale: "zh-TW", message: "店名太長" },
				},
			]),
		);
		mount();

		fireEvent.change(await screen.findByLabelText("商店名稱"), {
			target: { value: "改過的店名" },
		});
		await save();

		await waitFor(() => expect(mocks.toastError).toHaveBeenCalled());
		expect(mocks.toastError.mock.calls[0]?.[1]).toEqual({
			description: "店名太長",
		});
		expect((screen.getByLabelText("商店名稱") as HTMLInputElement).value).toBe(
			"改過的店名",
		);
	});

	it("locks the store name and description once PayUni holds the identity", async () => {
		mocks.flow = create(KYCFlowSchema, {
			status: KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED,
			identityLocked: true,
		});
		mount();

		expect(
			((await screen.findByLabelText("商店名稱")) as HTMLInputElement).readOnly,
		).toBe(true);
		expect(
			(screen.getByLabelText("商店介紹") as HTMLTextAreaElement).readOnly,
		).toBe(true);
		expect(
			(screen.getByLabelText("得獎紀錄 / 店鋪亮點") as HTMLInputElement)
				.readOnly,
		).toBe(false);
	});

	it("is read-only without STOREFRONT_WRITE: no save button and no image upload", async () => {
		cache.set(
			store({
				store: FIRST.store,
				capabilities: activeStatuses([SellerCapability.ACCOUNT_CLOSE]),
			}),
		);
		mount();

		const name = (await screen.findByLabelText("商店名稱")) as HTMLInputElement;
		expect(name.readOnly).toBe(true);
		expect(screen.queryByRole("button", { name: "儲存商店資訊" })).toBeNull();
		expect(mocks.brandingProps?.readOnly).toBe(true);
	});
});
