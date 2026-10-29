import { COUNTIES } from "@hawkr/twaddr/lookup";
import { describe, expect, it } from "vitest";
import type { StoryItem } from "@/features/feed/components/StoryRail";
import {
	normalizeCity,
	sortNearbyStories,
	TAIWAN_CITY_CENTERS,
} from "./nearbyStories";

const story = (id: string, city?: string): StoryItem => ({
	id,
	storeCode: id,
	name: id,
	avatar: "",
	city,
});

describe("normalizeCity", () => {
	it("turns the MOI spelling 臺 into the 台 used by the city table", () => {
		expect(normalizeCity("臺北市")).toBe("台北市");
		expect(normalizeCity("臺中市")).toBe("台中市");
		expect(normalizeCity("臺南市")).toBe("台南市");
		expect(normalizeCity("臺東縣")).toBe("台東縣");
	});

	it("leaves 台 and names without the character unchanged", () => {
		expect(normalizeCity("台北市")).toBe("台北市");
		expect(normalizeCity(" 高雄市 ")).toBe("高雄市");
		expect(normalizeCity(undefined)).toBe("");
	});

	it("maps every county name of the twaddr table onto a known city centre", () => {
		for (const county of COUNTIES) {
			expect(
				TAIWAN_CITY_CENTERS[normalizeCity(county.name)],
				county.name,
			).toBeDefined();
		}
	});
});

describe("sortNearbyStories", () => {
	const taipei = { lat: 25.0375, lng: 121.5637 };

	it("sorts by distance whether the city is spelled 臺 or 台", () => {
		const sorted = sortNearbyStories(
			[
				story("kaohsiung", "高雄市"),
				story("taichung", "臺中市"),
				story("taipei", "臺北市"),
			],
			taipei,
		);
		expect(sorted.map((s) => s.id)).toEqual([
			"taipei",
			"taichung",
			"kaohsiung",
		]);
		const asTai = sortNearbyStories(
			[story("a", "台中市"), story("b", "臺中市")],
			taipei,
		);
		expect(asTai.map((s) => s.id)).toEqual(["a", "b"]);
	});

	it("puts stories without a recognisable city last, in their original order", () => {
		const sorted = sortNearbyStories(
			[story("none"), story("unknown", "火星市"), story("taichung", "臺中市")],
			taipei,
		);
		expect(sorted.map((s) => s.id)).toEqual(["taichung", "none", "unknown"]);
	});
});
