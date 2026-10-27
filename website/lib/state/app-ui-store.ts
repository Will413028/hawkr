"use client";

import { create } from "zustand";
import { devtools } from "zustand/middleware";

export interface LoginRequiredOverlayPayload {
	redirect?: string;
	title?: string;
	description?: string;
	actionLabel?: string;
}

type AppOverlay = {
	name: "login-required";
	payload?: LoginRequiredOverlayPayload;
} | null;

type AppUiState = {
	overlay: AppOverlay;
	openLoginRequired: (payload?: LoginRequiredOverlayPayload) => void;
	closeOverlay: () => void;
};

export const useAppUiStore = create<AppUiState>()(
	devtools(
		(set) => ({
			overlay: null,
			openLoginRequired: (payload) =>
				set(
					{ overlay: { name: "login-required", payload } },
					false,
					"app-ui/openLoginRequired",
				),
			closeOverlay: () => set({ overlay: null }, false, "app-ui/closeOverlay"),
		}),
		{
			name: "app-ui-store",
			enabled: process.env.NODE_ENV === "development",
		},
	),
);
