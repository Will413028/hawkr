import { describe, expect, it, vi } from "vitest";

const rpc = vi.hoisted(() => ({
	createAddress: vi.fn(),
	updateAddress: vi.fn(),
	getAddress: vi.fn(),
	listAddresses: vi.fn(),
}));

vi.mock("../connect/buyer", () => ({ buyerAppClient: () => rpc }));

import {
	addressText,
	createAddress,
	fetchAddresses,
	setDefaultAddress,
	updateAddress,
} from "./repo";

describe("address etag", () => {
	it("uses the version loaded with the form and surfaces a stale version", async () => {
		rpc.updateAddress.mockReset().mockRejectedValue(new Error("版本衝突"));
		rpc.getAddress.mockReset().mockResolvedValue({ address: { etag: "v2" } });
		await expect(
			updateAddress("12", "v1", {
				receiverName: "甲",
				receiverPhone: "0912345678",
				townCode: "63000050",
				addressDetail: "一號",
				isDefault: false,
			}),
		).rejects.toThrow("版本衝突");
		expect(rpc.getAddress).not.toHaveBeenCalled();
		expect(rpc.updateAddress.mock.calls[0][0].address.etag).toBe("v1");
	});

	it("sets default with one write using the loaded version", async () => {
		rpc.updateAddress.mockReset().mockResolvedValue({});
		rpc.getAddress.mockReset();
		await setDefaultAddress("12", "v1");
		expect(rpc.updateAddress).toHaveBeenCalledOnce();
		expect(rpc.getAddress).not.toHaveBeenCalled();
		expect(rpc.updateAddress.mock.calls[0][0]).toMatchObject({
			address: { id: "12", etag: "v1", defaultAddress: true },
			updateMask: { paths: ["default_address"] },
		});
	});
});

describe("address list", () => {
	it("requests one page of 20", async () => {
		rpc.listAddresses.mockReset().mockResolvedValue({
			addresses: [{ id: "1", etag: "v1", defaultAddress: true }],
			nextPageToken: "",
		});
		const addresses = await fetchAddresses();
		expect(rpc.listAddresses).toHaveBeenCalledOnce();
		expect(rpc.listAddresses).toHaveBeenCalledWith({ pageSize: 20 });
		expect(addresses.map((a) => a.id)).toEqual(["1"]);
	});

	it("keeps reading while the response carries next_page_token", async () => {
		rpc.listAddresses
			.mockReset()
			.mockResolvedValueOnce({
				addresses: [{ id: "1", etag: "v1", defaultAddress: true }],
				nextPageToken: "p2",
			})
			.mockResolvedValueOnce({
				addresses: [{ id: "2", etag: "v1", defaultAddress: false }],
				nextPageToken: "",
			});
		const addresses = await fetchAddresses();
		expect(rpc.listAddresses).toHaveBeenCalledTimes(2);
		expect(rpc.listAddresses.mock.calls[1][0]).toEqual({
			pageSize: 20,
			pageToken: "p2",
		});
		expect(addresses.map((a) => a.id)).toEqual(["1", "2"]);
	});
});

describe("address wire shape", () => {
	const input = {
		receiverName: "甲",
		receiverPhone: "0912345678",
		townCode: "63000050",
		addressDetail: "一號",
		isDefault: false,
	};

	it("writes postal_address as one mask path on update", async () => {
		rpc.updateAddress.mockReset().mockResolvedValue({});
		await updateAddress("12", "v1", input);
		const request = rpc.updateAddress.mock.calls[0][0];
		expect(request.address.postalAddress).toEqual({
			townCode: "63000050",
			addressDetail: "一號",
		});
		expect(request.updateMask.paths).toContain("postal_address");
		expect(request.updateMask.paths).not.toContain("city");
		expect(request.updateMask.paths).not.toContain("detail_address");
	});

	it("lists exactly the fields the form owns in the update mask", async () => {
		rpc.updateAddress.mockReset().mockResolvedValue({});
		await updateAddress("12", "v1", input);
		expect(rpc.updateAddress.mock.calls[0][0].updateMask.paths).toEqual([
			"receiver_name",
			"receiver_phone",
			"postal_address",
			"default_address",
		]);
	});

	it("sends postalAddress on create and reads it back", async () => {
		rpc.createAddress.mockReset().mockResolvedValue({
			address: {
				id: "1",
				etag: "v1",
				postalAddress: { townCode: "63000050", addressDetail: "一號" },
			},
		});
		const created = await createAddress(input);
		expect(rpc.createAddress.mock.calls[0][0].address.postalAddress).toEqual({
			townCode: "63000050",
			addressDetail: "一號",
		});
		expect(created && addressText(created)).toBe("臺北市中正區一號");
	});
});
