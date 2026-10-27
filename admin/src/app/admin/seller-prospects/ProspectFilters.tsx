"use client";

import {
	OutreachStatus,
	ProspectStatus,
	VendorType,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
	DEFAULT_STATUS_FILTER,
	EDITABLE_OUTREACH_STATUSES,
	filterParam,
	outreachStatusText,
	prospectStatusText,
	readProspectFilters,
	STATUS_FILTER_OPTIONS,
	VENDOR_TYPE_OPTIONS,
	vendorTypeText,
} from "@/lib/seller-prospects";

/** 列表頁路徑；page.tsx 的「清除篩選」與還沒改走 Connect 的頁面也要導回這裡，共用同一個常數。 */
export const LIST_PATH = "/admin/seller-prospects";
const DEBOUNCE_MS = 300;

/** 篩選在網址的鍵（值是 `readProspectFilters` 認得的小寫字串或文字）。 */
type FilterParam =
	| "status"
	| "outreachStatus"
	| "vendorType"
	| "category"
	| "region"
	| "keyword";

/**
 * 站外業者列表頁的篩選列。篩選全部落在 URL query（不用 local state 當唯一來源），重新整理或貼連結給同事都不會掉篩選。
 * 狀態沒有「全部」，預設進行中（網址不帶 status）；招商狀態與業者型態沒選就是不篩選。
 *
 * 三個文字輸入（關鍵字／分類／地區）各自維護一份本地 echo：URL 是最終落點，
 * 但每個鍵擊都寫 URL 會讓外層的 `useQuery` 逐鍵 refetch，所以停打 300ms 後才提交。
 */
export default function ProspectFilters() {
	const router = useRouter();
	const searchParams = useSearchParams();
	const filters = readProspectFilters(searchParams);

	const [keywordInput, setKeywordInput] = useState(filters.keyword);
	const [categoryInput, setCategoryInput] = useState(filters.category);
	const [regionInput, setRegionInput] = useState(filters.region);

	// 外部改變 URL（清除篩選鍵、瀏覽器上一頁）時同步輸入框，避免畫面停在舊值。
	useEffect(() => setKeywordInput(filters.keyword), [filters.keyword]);
	useEffect(() => setCategoryInput(filters.category), [filters.category]);
	useEffect(() => setRegionInput(filters.region), [filters.region]);

	const commit = useCallback(
		(patch: Partial<Record<FilterParam, string>>) => {
			const params = new URLSearchParams(searchParams.toString());
			for (const [key, value] of Object.entries(patch)) {
				if (value) params.set(key, value);
				else params.delete(key);
			}
			const query = params.toString();
			router.replace(query ? `${LIST_PATH}?${query}` : LIST_PATH);
		},
		[router, searchParams],
	);

	useEffect(() => {
		const trimmed = keywordInput.trim();
		if (trimmed === filters.keyword) return;
		const timer = setTimeout(() => commit({ keyword: trimmed }), DEBOUNCE_MS);
		return () => clearTimeout(timer);
	}, [keywordInput, filters.keyword, commit]);

	useEffect(() => {
		const trimmed = categoryInput.trim();
		if (trimmed === filters.category) return;
		const timer = setTimeout(() => commit({ category: trimmed }), DEBOUNCE_MS);
		return () => clearTimeout(timer);
	}, [categoryInput, filters.category, commit]);

	useEffect(() => {
		const trimmed = regionInput.trim();
		if (trimmed === filters.region) return;
		const timer = setTimeout(() => commit({ region: trimmed }), DEBOUNCE_MS);
		return () => clearTimeout(timer);
	}, [regionInput, filters.region, commit]);

	const statusValue = filterParam(ProspectStatus, filters.status);
	const defaultStatusValue = filterParam(ProspectStatus, DEFAULT_STATUS_FILTER);

	return (
		<div className="filter-bar">
			<div className="filter-search">
				<MaterialSymbol name="search" />
				<input
					type="text"
					aria-label="搜尋業者名稱、商品摘要、市場定位或調研備註"
					placeholder="搜尋業者名稱、商品摘要、市場定位或調研備註"
					value={keywordInput}
					onChange={(e) => setKeywordInput(e.target.value)}
				/>
			</div>
			<select
				aria-label="狀態"
				className="filter-select"
				value={statusValue}
				onChange={(e) =>
					commit({
						status: e.target.value === defaultStatusValue ? "" : e.target.value,
					})
				}
			>
				{STATUS_FILTER_OPTIONS.map((status) => (
					<option key={status} value={filterParam(ProspectStatus, status)}>
						{prospectStatusText(status)}
					</option>
				))}
			</select>
			<select
				aria-label="招商狀態"
				className="filter-select"
				value={
					filters.outreachStatus === undefined
						? ""
						: filterParam(OutreachStatus, filters.outreachStatus)
				}
				onChange={(e) => commit({ outreachStatus: e.target.value })}
			>
				<option value="">全部</option>
				{EDITABLE_OUTREACH_STATUSES.map((status) => (
					<option key={status} value={filterParam(OutreachStatus, status)}>
						{outreachStatusText(status)}
					</option>
				))}
			</select>
			<select
				aria-label="業者型態"
				className="filter-select"
				value={
					filters.vendorType === undefined
						? ""
						: filterParam(VendorType, filters.vendorType)
				}
				onChange={(e) => commit({ vendorType: e.target.value })}
			>
				<option value="">全部</option>
				{VENDOR_TYPE_OPTIONS.map((type) => (
					<option key={type} value={filterParam(VendorType, type)}>
						{vendorTypeText(type)}
					</option>
				))}
			</select>
			<div className="filter-search">
				<MaterialSymbol name="sell" />
				<input
					type="text"
					aria-label="分類"
					placeholder="分類"
					value={categoryInput}
					onChange={(e) => setCategoryInput(e.target.value)}
				/>
			</div>
			<div className="filter-search">
				<MaterialSymbol name="location_on" />
				<input
					type="text"
					aria-label="地區"
					placeholder="地區"
					value={regionInput}
					onChange={(e) => setRegionInput(e.target.value)}
				/>
			</div>
		</div>
	);
}
