// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	refresh: vi.fn(async () => {}),
	uploadImageFile: vi.fn(async () => ({ stagedToken: "tok" })),
	updateAvatar: vi.fn(async (_token: string) => {}),
}));

vi.mock("@/lib/hooks/useAccount", () => ({
	useProfile: () => ({
		profile: { nickname: "n" },
		loading: false,
		saving: false,
		error: null,
		refresh: mocks.refresh,
		save: vi.fn(),
		updateAvatar: mocks.updateAvatar,
	}),
}));
vi.mock("@/lib/i18n/ui-text", () => ({ useUiText: () => ({}) }));
vi.mock("./genderOptions", () => ({
	genderOptions: () => [{ label: "x", value: "0" }],
	genderFromValue: () => undefined,
}));
vi.mock("@/lib/media/upload", () => ({
	uploadImageFile: mocks.uploadImageFile,
}));
vi.mock("@/components/shared/Avatar", () => ({ default: () => null }));
vi.mock("@/components/shared/SellerPageShell", () => ({
	SellerPageShell: ({ children }: { children: React.ReactNode }) =>
		React.createElement("div", null, children),
}));
vi.mock("@/components/shared/UploadLoadingIndicator", () => ({
	default: () => null,
}));
vi.mock("@/components/ui", () => ({
	DatePicker: () => null,
	Dropdown: () => null,
	Input: () => null,
}));
vi.mock("@/components/ui/Button", () => ({
	default: ({ children }: { children?: React.ReactNode }) =>
		React.createElement("button", { type: "button" }, children),
}));
vi.mock("@/components/ui/ImageCropDialog", () => ({
	ImageCropDialog: ({
		files,
		onCropped,
	}: {
		files: File[];
		onCropped: (file: File) => void;
	}) =>
		files.length
			? React.createElement(
					"button",
					{ type: "button", onClick: () => onCropped(files[0] as File) },
					"confirm-crop",
				)
			: null,
}));

import ProfileClient from "./ProfileClient";

afterEach(cleanup);

describe("ProfileClient avatar upload", () => {
	it("hands the staged upload to the profile hook, and a failed refetch afterwards still ends the busy state", async () => {
		mocks.refresh.mockRejectedValueOnce(new Error("refetch failed"));
		const { container } = render(React.createElement(ProfileClient));
		const input = container.querySelector(
			'input[type="file"]',
		) as HTMLInputElement;
		const file = new File(["x"], "a.png", { type: "image/png" });
		Object.defineProperty(input, "files", { value: [file] });
		input.dispatchEvent(new Event("change", { bubbles: true }));
		(await screen.findByText("confirm-crop")).click();
		await waitFor(() => expect(mocks.updateAvatar).toHaveBeenCalledWith("tok"));
		await waitFor(() => expect(mocks.refresh).toHaveBeenCalled());
	});
});
