"use client";

import { COUNTIES, townsByCounty } from "@hawkr/twaddr/lookup";
import * as React from "react";
import { Dropdown } from "@/components/ui";

const COUNTY_ITEMS = COUNTIES.map((c) => ({ value: c.code, label: c.name }));

/**
 * 縣市 → 鄉鎮市區的連動下拉。值是內政部代碼（縣市 5 碼、鄉鎮市區 8 碼），顯示名稱來自 twaddr.gen
 * （內政部寫法）。鄉鎮市區在選了縣市之前不能選；換縣市時，原本的鄉鎮市區不屬於新縣市就清掉。
 */
export function TownFields({
	countyCode,
	townCode,
	onChange,
	countyError,
	townError,
	disabled = false,
	required = true,
	countyLabel = "縣市",
	townLabel = "鄉鎮市區",
}: {
	countyCode: string;
	townCode: string;
	onChange: (countyCode: string, townCode: string) => void;
	countyError?: string;
	townError?: string;
	disabled?: boolean;
	required?: boolean;
	countyLabel?: string;
	townLabel?: string;
}) {
	const towns = React.useMemo(() => townsByCounty(countyCode), [countyCode]);
	const townItems = React.useMemo(
		() => towns.map((t) => ({ value: t.code, label: t.name })),
		[towns],
	);
	const townValue = towns.some((t) => t.code === townCode) ? townCode : "";
	const canPickTown = towns.length > 0;

	return (
		<>
			<Dropdown
				label={countyLabel}
				placeholder="選擇縣市"
				variant="bordered"
				size="lg"
				floatingLabel
				isRequired={required}
				value={countyCode || undefined}
				onChange={(nextCounty) => {
					const keep = townsByCounty(nextCounty).some(
						(t) => t.code === townCode,
					);
					onChange(nextCounty, keep ? townCode : "");
				}}
				items={COUNTY_ITEMS}
				searchable
				disabled={disabled}
				noResultsLabel="找不到符合的縣市"
				errorMessage={countyError}
				isInvalid={Boolean(countyError)}
			/>
			<Dropdown
				label={townLabel}
				placeholder={canPickTown ? "選擇鄉鎮市區" : "請先選擇縣市"}
				variant="bordered"
				size="lg"
				floatingLabel
				isRequired={required}
				value={townValue || undefined}
				onChange={(nextTown) => onChange(countyCode, nextTown)}
				items={townItems}
				searchable
				disabled={disabled || !canPickTown}
				noResultsLabel="找不到符合的鄉鎮市區"
				errorMessage={townError}
				isInvalid={Boolean(townError)}
			/>
		</>
	);
}
