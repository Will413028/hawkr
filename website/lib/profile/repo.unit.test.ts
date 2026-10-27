import { Gender } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { beforeEach, describe, expect, it, vi } from "vitest";

const rpc = vi.hoisted(() => ({
	getProfile: vi.fn(),
	updateProfile: vi.fn(),
}));
vi.mock("../connect/buyer", () => ({ buyerAppClient: () => rpc }));

import { fetchProfile, updateBuyerAvatar, updateProfile } from "./repo";

const MASK = ["display_name", "gender", "birth_date"];

describe("buyer profile Connect contract", () => {
	beforeEach(() => {
		for (const fn of Object.values(rpc)) fn.mockReset();
		rpc.getProfile.mockResolvedValue({
			profile: {
				id: "9007199254740993",
				etag: "v1",
				displayName: "買家",
				avatar: { uriTemplate: "/i/{preset}/avatar.png" },
				birthDate: { year: 1991, month: 3, day: 14 },
			},
		});
	});

	it("keeps the exact buyer id, avatar URL and birth date", async () => {
		expect(await fetchProfile()).toMatchObject({
			id: "9007199254740993",
			nickname: "買家",
			avatarKey: "/i/avatar/avatar.png",
			birthday: "1991-03-14",
		});
	});

	it("updates only profile fields with the loaded etag", async () => {
		const loaded = await fetchProfile();
		await updateProfile(
			{ nickname: "新買家", gender: undefined, birthDate: "1992-04-05" },
			loaded,
		);
		expect(rpc.updateProfile).toHaveBeenCalledWith({
			profile: expect.objectContaining({
				id: "9007199254740993",
				etag: "v1",
				displayName: "新買家",
				birthDate: { year: 1992, month: 4, day: 5 },
			}),
			updateMask: { paths: MASK },
		});
		expect(rpc.updateProfile.mock.calls[0][0].profile).not.toHaveProperty(
			"avatar",
		);
	});

	it("rejects a malformed birth date without writing, so the stored birthday is kept", async () => {
		const loaded = await fetchProfile();
		for (const malformed of ["abc", "1990-13-01", "1990-02-30", "a-b-c"]) {
			await expect(
				updateProfile(
					{ nickname: "新買家", gender: undefined, birthDate: malformed },
					loaded,
				),
			).rejects.toThrow("生日格式不正確");
		}
		expect(rpc.updateProfile).not.toHaveBeenCalled();
	});

	it("clears the birthday: an empty birthDate keeps birth_date in the mask with no value", async () => {
		const loaded = await fetchProfile();
		await updateProfile(
			{ nickname: "新買家", gender: undefined, birthDate: "" },
			loaded,
		);
		const request = rpc.updateProfile.mock.calls[0][0];
		expect(request.updateMask).toEqual({ paths: MASK });
		expect(request.profile.birthDate).toBeUndefined();
	});

	it("reads an unset gender as undefined and keeps a stored one", async () => {
		expect((await fetchProfile()).gender).toBeUndefined();
		rpc.getProfile.mockResolvedValue({
			profile: { id: "1", etag: "v1", gender: Gender.FEMALE },
		});
		expect((await fetchProfile()).gender).toBe(Gender.FEMALE);
	});

	it("treats UNSPECIFIED and an unrecognized value as not filled", async () => {
		for (const value of [Gender.UNSPECIFIED, 99]) {
			rpc.getProfile.mockResolvedValue({
				profile: { id: "1", etag: "v1", gender: value },
			});
			expect((await fetchProfile()).gender, String(value)).toBeUndefined();
		}
	});

	it("sends a chosen gender as the enum, and no value when there is none", async () => {
		const loaded = await fetchProfile();
		await updateProfile(
			{ nickname: "新買家", gender: Gender.OTHER, birthDate: "" },
			loaded,
		);
		expect(rpc.updateProfile.mock.calls[0][0].profile.gender).toBe(
			Gender.OTHER,
		);

		rpc.updateProfile.mockClear();
		await updateProfile(
			{ nickname: "新買家", gender: undefined, birthDate: "" },
			loaded,
		);
		const request = rpc.updateProfile.mock.calls[0][0];
		expect(request.profile.gender).toBeUndefined();
		expect(request.updateMask.paths).toContain("gender");
	});

	it("finalizes the staged avatar through UpdateProfile with the loaded etag", async () => {
		const loaded = await fetchProfile();
		rpc.updateProfile.mockClear();
		rpc.updateProfile.mockResolvedValue({
			profile: { avatar: { uriTemplate: "/i/{preset}/new.png" } },
		});
		expect(await updateBuyerAvatar("staged-token", loaded)).toBe(
			"/i/avatar/new.png",
		);
		expect(rpc.updateProfile).toHaveBeenCalledWith({
			profile: {
				id: loaded.id,
				etag: "v1",
				avatarStagedToken: "staged-token",
			},
			updateMask: { paths: ["avatar_staged_token"] },
		});
	});

	it("does not send an empty staged token", async () => {
		const loaded = await fetchProfile();
		rpc.updateProfile.mockClear();
		await expect(updateBuyerAvatar("", loaded)).rejects.toThrow();
		expect(rpc.updateProfile).not.toHaveBeenCalled();
	});

	it("preserves a stale etag and exposes the conflict", async () => {
		const loaded = await fetchProfile();
		rpc.getProfile.mockResolvedValue({
			profile: { id: loaded.id, etag: "v2" },
		});
		rpc.updateProfile.mockRejectedValue(new Error("conflict"));
		await expect(
			updateProfile(
				{ nickname: "過期修改", gender: undefined, birthDate: "" },
				loaded,
			),
		).rejects.toThrow("conflict");
		expect(rpc.getProfile).toHaveBeenCalledTimes(1);
		expect(rpc.updateProfile.mock.calls[0][0].profile.etag).toBe("v1");
	});
});
