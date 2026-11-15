import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";
import "../../styles/globals.css";
import "../../styles/components/product-modal.css";
import "../../styles/components/ui-kit.css";
import "../../styles/components/input-field-fx.css";

// 沒有開 vitest globals，@testing-library/react 不會自己註冊 cleanup。
afterEach(() => {
	cleanup();
});
