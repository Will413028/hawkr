import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const interceptedProductPage = new URL(
	"../app/[locale]/(public)/@modal/(.)products/[id]/page.tsx",
	import.meta.url,
);
const productDetailPage = new URL(
	"../features/product/ProductDetailPage.tsx",
	import.meta.url,
);
const productModal = new URL(
	"../features/product/components/ProductModal.tsx",
	import.meta.url,
);

test("the intercepted product opens one persistent modal shell while content streams", async () => {
	const source = await readFile(interceptedProductPage, "utf8");

	assert.match(source, /<ProductModal>/);
	assert.match(
		source,
		/fallback=\{<RouteLoadingSkeleton page variant="product-detail" \/>\}/,
	);
	assert.match(source, /<InterceptedProductModalContent params=\{params\} \/>/);
	assert.match(
		source,
		/async function InterceptedProductModalContent\(\{ params \}: Props\)/,
	);

	const modalStart = source.indexOf("<ProductModal>");
	const suspenseStart = source.indexOf("<Suspense", modalStart);
	const contentStart = source.indexOf(
		"async function InterceptedProductModalContent",
	);
	const paramsAwait = source.indexOf(
		"const { id } = await params",
		contentStart,
	);
	const modalEnd = source.indexOf("</ProductModal>", suspenseStart);
	assert.ok(modalStart >= 0 && suspenseStart > modalStart);
	assert.ok(modalEnd > suspenseStart && contentStart > modalEnd);
	assert.ok(paramsAwait > contentStart);
	assert.doesNotMatch(source, /fallback=\{null\}/);
});

test("seller and reviews stream behind independent keyed fallbacks", async () => {
	const source = await readFile(productDetailPage, "utf8");

	assert.match(source, /<Suspense\s+key="seller-intro"\s+fallback=/);
	assert.match(source, /<ProductSellerIntro[\s\S]*\bpending\b[\s\S]*\/>/);
	assert.match(source, /<SellerIntroAsync/);
	assert.match(
		source,
		/<Suspense key="reviews" fallback=\{<ProductReviewsSkeleton \/>\}>/,
	);
	assert.match(source, /<ReviewsAsync productId=\{id\} \/>/);
	assert.doesNotMatch(source, /SellerIntroAsync\(\{/);
	assert.doesNotMatch(source, /ReviewsAsync\(\{/);
});

test("the streaming shell keeps keyboard close and focus-management behavior", async () => {
	const source = await readFile(productModal, "utf8");

	assert.match(source, /event\.key === "Escape"/);
	assert.match(source, /event\.target\.closest\("\[data-zoom-back\]"\)/);
	assert.doesNotMatch(source, /aria-label="關閉商品詳情"/);
	assert.match(source, /modal\?\.focus\(\{ preventScroll: true \}\)/);
	assert.match(source, /element\.inert = true/);
	assert.match(source, /previouslyFocused\.focus\(\{ preventScroll: true \}\)/);
});
