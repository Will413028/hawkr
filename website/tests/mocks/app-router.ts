import {
	AppRouterContext,
	type AppRouterInstance,
} from "next/dist/shared/lib/app-router-context.shared-runtime";
import React from "react";
import { ViewTransitions } from "@/components/shared/view-transitions";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";

/**
 * SSR 單元測試用的 App Router + ViewTransitions + intl stub。
 *
 * intl：頁面路徑類的 hook（`usePathname`，lib/intl/navigation.ts）是 next-intl 的，沒有 provider 會 throw。
 *
 * 共用 Link（next-view-transitions）內部呼叫 useRouter() 與
 * useSetFinishViewTransition()，在 renderToStaticMarkup 這種脫離 Next 的
 * 裸渲染下缺 context 會直接 throw。凡是會轉出 <Link>（Button/HomeButton
 * 帶 href 等）的測試，都用 withAppRouter 包一層。
 */
const noop = () => {};

const routerStub: AppRouterInstance = {
	bfcacheId: "test",
	back: noop,
	forward: noop,
	refresh: noop,
	push: noop,
	replace: noop,
	prefetch: noop,
};

export function withAppRouter(children: React.ReactNode): React.ReactElement {
	return React.createElement(
		IntlTestWrapper,
		null,
		React.createElement(
			AppRouterContext.Provider,
			{ value: routerStub },
			React.createElement(ViewTransitions, null, children),
		),
	);
}
