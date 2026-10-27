const PRODUCT =
	process.env.LHCI_SAMPLE_PRODUCT_URL || "http://localhost:3000/products/1";
const SELLER =
	process.env.LHCI_SAMPLE_SELLER_URL || "http://localhost:3000/sellers/1";

module.exports = {
	ci: {
		collect: {
			startServerCommand: "pnpm start",
			url: [
				"http://localhost:3000/",
				"http://localhost:3000/market/products",
				"http://localhost:3000/faq",
				"http://localhost:3000/seller",
				PRODUCT,
				SELLER,
			],
			numberOfRuns: 1,
			settings: {
				// Lighthouse's default form factor is mobile; emulate a mid-tier phone.
				formFactor: "mobile",
				screenEmulation: {
					mobile: true,
					width: 412,
					height: 823,
					deviceScaleFactor: 2.625,
					disabled: false,
				},
				// The self-hosted runner has no usable Chrome sandbox (restricted
				// user namespaces), so headless Chrome crashes on launch without
				// --no-sandbox. --disable-dev-shm-usage avoids /dev/shm crashes.
				chromeFlags: "--no-sandbox --disable-dev-shm-usage",
				budgetPath: "./budget.json",
			},
		},
		assert: {
			preset: "lighthouse:recommended",
			assertions: {
				"resource-summary:script:size": ["warn", { maxNumericValue: 700000 }],
				"resource-summary:total:size": ["warn", { maxNumericValue: 3500000 }],
				"categories:performance": ["warn", { minScore: 0.4 }],
				"categories:seo": ["warn", { minScore: 0.9 }],
				"unused-javascript": "off",
				"uses-responsive-images": "off",
			},
		},
		// Keep reports in-CI (archived as an artifact). NOT temporary-public-storage:
		// the prod-flavored build serves real dev-backend seller/product data, which
		// must not land in publicly-shareable screenshots for a pre-launch site.
		// outputDir is NOT dot-prefixed: actions/upload-artifact@v6 silently fails
		// to match dot-directories ("No files were found" despite the directory
		// being populated — reproduced live in CI, see actions/upload-artifact#717).
		upload: { target: "filesystem", outputDir: "./lighthouse-reports" },
	},
};
