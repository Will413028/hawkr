"use client";

import * as React from "react";
import { Sheet, type SheetProps } from "@/components/ui";

/**
 * 全站唯一的右側抽屜元件，並提供「編輯後同步父頁」的統一契約。
 * UI kit 的 Sheet 保持純底層，Drawer 只多包一個選配的 onMutated。
 */
export interface DrawerMutation {
	/** 通知父頁抽屜內資料已變更，與關閉抽屜無關。 */
	notify: () => void;
	/** 目前是否位於 Drawer 內。 */
	active: boolean;
}

const NOOP_MUTATION: DrawerMutation = { notify: () => {}, active: false };

const DrawerMutationContext =
	React.createContext<DrawerMutation>(NOOP_MUTATION);

export function useDrawerMutated(): DrawerMutation {
	return React.useContext(DrawerMutationContext);
}

export interface DrawerProps extends SheetProps {
	/** 抽屜內成功 mutation 後，要求父頁即時同步資料。 */
	onMutated?: () => void;
}

export function Drawer({ onMutated, children, ...sheetProps }: DrawerProps) {
	const onMutatedRef = React.useRef(onMutated);
	onMutatedRef.current = onMutated;

	const value = React.useMemo<DrawerMutation>(
		() => ({
			active: true,
			notify: () => {
				onMutatedRef.current?.();
			},
		}),
		[],
	);

	return (
		<DrawerMutationContext.Provider value={value}>
			<Sheet {...sheetProps}>{children}</Sheet>
		</DrawerMutationContext.Provider>
	);
}
