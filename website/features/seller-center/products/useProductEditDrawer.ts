"use client";

import * as React from "react";

/**
 * 商品快速編輯 drawer 的本地狀態 — 商品管理 / 庫存管理兩頁各自獨立持有
 * （兩路由互斥，無需跨頁共享，故用本地 state 而非全域 store）。
 *
 *   - `editProductId` — 正在編輯的商品；`null` 表示關閉。
 *   - `expanded`      — 窄版快速編輯（false）vs 寬版完整編輯（true）；
 *                       每次開啟重設為 false，讓 drawer 一律先落在 quick 模式。
 */
export function useProductEditDrawer() {
	const [editProductId, setEditProductId] = React.useState<string | null>(null);
	const [expanded, setExpanded] = React.useState(false);

	const openEdit = React.useCallback((productId: string) => {
		setEditProductId(productId);
		setExpanded(false);
	}, []);
	const closeEdit = React.useCallback(() => setEditProductId(null), []);
	const toggleExpanded = React.useCallback(() => setExpanded((v) => !v), []);

	return { editProductId, expanded, openEdit, closeEdit, toggleExpanded };
}
