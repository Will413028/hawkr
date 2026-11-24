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

/** `@/lib/i18n/ui-text` 的替身：文字取固定值，讓測試不依賴 next-intl。 */
export const uiTextStub = {
	useUiText: () => () => "已關閉的店家",
	uiTextFor: () => () => "已關閉的店家",
	optionalText: (_ut: unknown, m: unknown) =>
		m === undefined ? undefined : "x",
	useDefaultErrorText: () => (code: number) => `default-${code}`,
	defaultErrorTextFor: () => (code: number) => `default-${code}`,
};
