import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";
import "../../styles/globals.css";
import "../../styles/components/product-modal.css";
import "../../styles/components/ui-kit.css";
import "../../styles/components/input-field-fx.css";

// Next 的 client bundle 提供空的 process polyfill；沒有在 define 替換的 process.env.* 讀成 undefined。
// setupFiles 在測試檔載入前執行，被測模組頂層讀 process.env 時墊片已經存在。
if (typeof globalThis.process === "undefined") {
	Object.assign(globalThis, { process: { env: {} } });
}

vi.mock("next/image", async () => await import("./next-image"));

// 沒有開 vitest globals，@testing-library/react 不會自己註冊 cleanup。
afterEach(() => {
	cleanup();
	document.documentElement.style.removeProperty("font-size");
	document.documentElement.style.removeProperty("--hk-text-scale");
});
