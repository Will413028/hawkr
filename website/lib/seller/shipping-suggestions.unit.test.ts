import { beforeEach, describe, expect, it, vi } from "vitest";

const authedFetch = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api/client", () => ({ authedFetch }));

import { normalizeAddressDetailCandidate } from "@/features/seller/components/AddressAutocompleteField";
import { getAddressSuggestions } from "@/lib/seller/shipping";

beforeEach(() => authedFetch.mockReset());

describe("TGOS suggestions vs the logistics-table 台 names", () => {
	it("keeps suggestions whose county and district use 臺 anywhere in the name", async () => {
		const tw = (county: string, district: string) => ({
			fullAddress: `${county}${district}x`,
			county,
			district,
			detail: "x",
		});
		authedFetch.mockResolvedValue({
			data: [
				tw("雲林縣", "臺西鄉"),
				tw("臺東縣", "臺東市"),
				tw("臺東縣", "海端鄉"),
			],
		});
		const got = await getAddressSuggestions({
			county: "台東縣",
			district: "台東市",
			query: "中華路",
		});
		expect(got.map((s) => s.district)).toEqual(["臺東市"]);
		authedFetch.mockResolvedValue({ data: [tw("屏東縣", "霧臺鄉")] });
		expect(
			await getAddressSuggestions({
				county: "屏東縣",
				district: "霧台鄉",
				query: "某路",
			}),
		).toHaveLength(1);
		authedFetch.mockResolvedValue({ data: [tw("雲林縣", "臺西鄉")] });
		expect(
			await getAddressSuggestions({
				county: "雲林縣",
				district: "台西鄉",
				query: "中山路",
			}),
		).toHaveLength(1);
	});
});

describe("normalizeAddressDetailCandidate", () => {
	it("strips a 臺-spelled county and district prefix from the typed detail", () => {
		expect(
			normalizeAddressDetailCandidate(
				"臺東縣臺東市中華路1號",
				"台東縣",
				"台東市",
			),
		).toBe("中華路1號");
		expect(
			normalizeAddressDetailCandidate(
				"雲林縣臺西鄉中山路1號",
				"雲林縣",
				"台西鄉",
			),
		).toBe("中山路1號");
	});
});
