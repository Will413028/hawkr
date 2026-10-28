import { describe, expect, it } from "vitest";
import type { ShippingQuoteOption } from "@/lib/products/seller-repo";
import { quoteOptionFor } from "./SkuShippingEditor";

const option = (shippingMethod: string, fee: string): ShippingQuoteOption => ({
	shippingMethod,
	displayName: shippingMethod,
	fee,
	estimatedDays: "2",
});

describe("quoteOptionFor", () => {
	it("returns the server option for a method without client-side filtering", () => {
		const quotes = [option("3001", "100"), option("3002", "60")];
		expect(quoteOptionFor(quotes, "3001")?.fee).toBe("100");
		expect(quoteOptionFor(quotes, "3002")?.fee).toBe("60");
	});

	it("returns the first server option even if it carries an extra temperature field", () => {
		const withTemperature = { ...option("3001", "100"), temperature: "3202" };
		const quotes: ShippingQuoteOption[] = [withTemperature];
		expect(quoteOptionFor(quotes, "3001")?.fee).toBe("100");
	});

	it("is undefined when the server did not offer the method", () => {
		expect(quoteOptionFor([option("3001", "100")], "3002")).toBeUndefined();
	});
});
