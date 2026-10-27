import { Code, ConnectError } from "@connectrpc/connect";
import {
	codeToHttpStatus,
	errorToJson,
} from "@connectrpc/connect/protocol-connect";

/**
 * 替 stub 掉的 fetch 組出 Connect（JSON）的回應：以 connect-web 真正會讀到的 wire 格式測試 client，
 * 不假設它內部怎麼解碼。
 */
export function connectOk(body: unknown = {}): Response {
	return new Response(JSON.stringify(body), {
		status: 200,
		headers: { "content-type": "application/json" },
	});
}

export function connectError(
	code: Code,
	message = Code[code],
	details: ConstructorParameters<typeof ConnectError>[3] = [],
): Response {
	const error = new ConnectError(message, code, undefined, details);
	return new Response(JSON.stringify(errorToJson(error, undefined)), {
		status: codeToHttpStatus(code),
		headers: { "content-type": "application/json" },
	});
}

/**
 * GetSessionResponse（proto3 JSON，identity.proto）。每個帳號都是買家：`roles` 只列 ROLE_SELLER；
 * 工作區（視角）是 client 的狀態，回應不帶。
 */
export function sessionJson(id: string, account: { seller?: boolean } = {}) {
	return {
		account: {
			id,
			roles: account.seller ? ["ROLE_SELLER"] : [],
		},
	};
}

/** 以 URL 判斷打的是哪一支 Connect 方法，回傳 `Service/Method`（去掉 package；GET 的 query 不影響）。 */
export function procedureOf(input: RequestInfo | URL): string {
	const url = new URL(
		typeof input === "string" || input instanceof URL ? input : input.url,
	);
	const [service = "", method = ""] = url.pathname.split("/").slice(-2);
	return `${service.split(".").at(-1)}/${method}`;
}
