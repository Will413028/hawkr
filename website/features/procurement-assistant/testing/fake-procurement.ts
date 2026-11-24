import {
	Code,
	ConnectError,
	createClient,
	createRouterTransport,
} from "@connectrpc/connect";
import { BuyerProcurementAppService } from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";

/** 測試用的 handler 表：方法名（lowerCamelCase）對應一個收請求回回應的函式；沒登記的方法回 unimplemented。串流方法的 handler 回一串要依序送出的回應（拋錯會讓串流以該錯誤結束）。 */
export type ProcurementHandlers = Record<
	string,
	((request: never) => unknown) | undefined
>;

export function fakeProcurementClient(handlers: ProcurementHandlers) {
	return createClient(
		BuyerProcurementAppService,
		createRouterTransport(({ service }) => {
			const impl: Record<string, unknown> = {};
			for (const method of BuyerProcurementAppService.methods) {
				const name = method.localName;
				impl[name] =
					method.methodKind === "server_streaming"
						? async function* (request: never) {
								const handler = handlers[name];
								if (!handler) {
									throw new ConnectError(
										`${name} unimplemented`,
										Code.Unimplemented,
									);
								}
								yield* (await handler(request)) as Iterable<unknown>;
							}
						: (request: never) => {
								const handler = handlers[name];
								if (!handler) {
									throw new ConnectError(
										`${name} unimplemented`,
										Code.Unimplemented,
									);
								}
								return handler(request);
							};
			}
			service(BuyerProcurementAppService, impl as never);
		}),
	);
}

const closedStoreText = () => "已關閉的店家";
const defaultErrorText = (code: number) => `default-${code}`;

/** `@/lib/i18n/ui-text` 的替身：文字取固定值，讓測試不依賴 next-intl。回傳的函式必須是穩定的引用（和正式的 hook 用 useMemo 一樣），否則依賴它的 useCallback／useEffect 每次 render 都重跑，無限重繪。 */
export const uiTextStub = {
	useUiText: () => closedStoreText,
	uiTextFor: () => closedStoreText,
	optionalText: (_ut: unknown, m: unknown) =>
		m === undefined ? undefined : "x",
	useDefaultErrorText: () => defaultErrorText,
	defaultErrorTextFor: () => defaultErrorText,
};
