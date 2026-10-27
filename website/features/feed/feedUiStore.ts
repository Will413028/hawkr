"use client";

import { create } from "zustand";
import { devtools } from "zustand/middleware";
import type { FeedListSnapshotTab } from "@/lib/feed/listSnapshot";

type FeedUiState = {
	activeTab: FeedListSnapshotTab;
	activeTabScopeKey: string;
	openPostId: string | null;
	setActiveTab: (activeTab: FeedListSnapshotTab, scopeKey?: string) => void;
	setOpenPostId: (postId: string | number | null) => void;
	resetFeedUi: () => void;
};

export const useFeedUiStore = create<FeedUiState>()(
	devtools(
		(set) => ({
			activeTab: "recommend",
			activeTabScopeKey: "",
			openPostId: null,
			setActiveTab: (activeTab, activeTabScopeKey = "") =>
				set({ activeTab, activeTabScopeKey }, false, "feed-ui/setActiveTab"),
			setOpenPostId: (postId) =>
				set(
					{ openPostId: postId === null ? null : String(postId) },
					false,
					"feed-ui/setOpenPostId",
				),
			resetFeedUi: () =>
				set(
					{ activeTab: "recommend", activeTabScopeKey: "", openPostId: null },
					false,
					"feed-ui/resetFeedUi",
				),
		}),
		{
			name: "feed-ui-store",
			enabled: process.env.NODE_ENV === "development",
		},
	),
);
