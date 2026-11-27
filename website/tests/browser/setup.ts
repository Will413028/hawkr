import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";
import { page } from "vitest/browser";
import "../../styles/globals.css";
import "../../styles/components/product-modal.css";
import "../../styles/components/ui-kit.css";
import "../../styles/components/input-field-fx.css";

if (typeof globalThis.process === "undefined") {
	Object.assign(globalThis, { process: { env: {} } });
}

const initialViewport = {
	width: window.innerWidth,
	height: window.innerHeight,
};

afterEach(async () => {
	cleanup();
	await page.viewport(initialViewport.width, initialViewport.height);
	document.documentElement.style.removeProperty("font-size");
	document.documentElement.style.removeProperty("--hk-text-scale");
});
