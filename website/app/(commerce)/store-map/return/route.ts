/**
 * Web callback for the PayUni 7-11 electronic-map flow.
 *
 * The gateway issues a session via `POST /app/store_map_sessions` with
 * `appType: "web"`. PayUni's `MapReturnURL` is wired to call us here
 * after the buyer picks a store; the gateway decrypts PayUni's payload
 * and 302s to this page with plain query params (storeCode + storeName
 * + storeAddress). For native apps the same gateway 302s to a `hawkrbuyer://`
 * deep link instead.
 *
 * We render a tiny HTML doc that `postMessage`s the result up to the
 * opener window (the cart's pickup-picker), then closes itself. The
 * opener verifies `event.origin` matches the site, so this page is the
 * only thing allowed to deliver a store result into the picker.
 *
 * Both GET (302 from the gateway) and POST (PayUni form-encoded
 * fallback, in case the gateway forwards the form verbatim) are
 * accepted. Field names are read tolerantly (camelCase + PayUni's
 * native names) because the wire-shape on this hop has historically
 * drifted.
 */

import { type NextRequest, NextResponse } from "next/server";

interface Store {
	storeId: string;
	storeName: string;
	storeAddress: string;
	error?: string;
}

function pickStore(params: URLSearchParams): Store {
	const get = (...keys: string[]) => {
		for (const k of keys) {
			const v = params.get(k);
			if (v != null && v !== "") return v;
		}
		return "";
	};
	return {
		storeId: get(
			"storeId",
			"storeCode",
			"store_id",
			"store_code",
			"StoreID",
			"CVSStoreID",
			"CVSStoreCode",
		),
		storeName: get("storeName", "store_name", "StoreName", "CVSStoreName"),
		storeAddress: get(
			"storeAddress",
			"store_address",
			"address",
			"StoreAddress",
			"CVSAddress",
			"CVSStoreAddress",
		),
		error: get("error", "errorMessage", "msg") || undefined,
	};
}

function escapeForScript(value: string): string {
	return value
		.replace(/\\/g, "\\\\")
		.replace(/"/g, '\\"')
		.replace(/<\//g, "<\\/")
		.replace(/\r/g, "\\r")
		.replace(/\n/g, "\\n")
		.replace(/\u2028/g, "\\u2028")
		.replace(/\u2029/g, "\\u2029");
}

function renderHtml(store: Store, origin: string): string {
	const payload = JSON.stringify({
		type: "hk-store-map-result",
		storeId: store.storeId,
		storeName: store.storeName,
		storeAddress: store.storeAddress,
		error: store.error,
	});
	const safePayload = escapeForScript(payload);
	const safeOrigin = escapeForScript(origin);
	return `<!doctype html>
<html lang="zh-Hant">
<head>
<meta charset="utf-8" />
<title>選擇門市回傳中…</title>
<meta name="viewport" content="width=device-width,initial-scale=1" />
<style>
  body { margin: 0; padding: 32px; font-family: system-ui, -apple-system, "Noto Sans TC", sans-serif; color: #28333f; background: #f5f7fa; }
  main { max-width: 360px; margin: 0 auto; text-align: center; }
  h1 { font-size: 16px; margin: 0 0 8px; }
  p { margin: 0; font-size: 14px; color: #616e7c; }
  button { margin-top: 18px; padding: 10px 18px; border-radius: 999px; border: 1px solid #b8441f; background: #b8441f; color: white; font-weight: 600; cursor: pointer; }
</style>
</head>
<body>
<main>
  <h1>已選擇門市</h1>
  <p id="msg">正在回傳資料到結帳頁面…</p>
  <button type="button" id="close">關閉視窗</button>
</main>
<script>
(function () {
  var payload = JSON.parse("${safePayload}");
  var target = window.opener;
  var origins = [];
  function addOrigin(value) {
    if (!value || origins.indexOf(value) !== -1) return;
    origins.push(value);
  }
  addOrigin(window.location.origin);
  try {
    if (document.referrer) addOrigin(new URL(document.referrer).origin);
  } catch (e) {
    /* ignore malformed referrer */
  }
  addOrigin("${safeOrigin}");
  try {
    if (target && !target.closed) {
      for (var i = 0; i < origins.length; i += 1) {
        target.postMessage(payload, origins[i]);
      }
    }
  } catch (e) {
    /* opener may be cross-origin or already closed */
  }
  document.getElementById("close").addEventListener("click", function () {
    try { window.close(); } catch (e) {}
  });
  // postMessage is queued synchronously on the opener; closing immediately
  // can still race with popup.closed polling in some browsers. Keep this page
  // alive briefly so the opener can process the result before close detection.
  window.setTimeout(function () {
    try { window.close(); } catch (e) {}
  }, 250);
})();
</script>
</body>
</html>`;
}

function siteOrigin(req: NextRequest): string {
	const env = process.env.NEXT_PUBLIC_SITE_URL;
	if (env) {
		try {
			return new URL(env).origin;
		} catch {
			/* fall through */
		}
	}
	// Next standalone behind nginx: NextRequest.url is built from the
	// container's internal HOSTNAME:PORT (e.g. https://0.0.0.0:3080), not the
	// proxied Host header — so postMessage targetOrigin would be unreachable
	// for the opener. Read the proxy-forwarded host explicitly.
	const proto = req.headers.get("x-forwarded-proto") ?? "https";
	const host = req.headers.get("x-forwarded-host") ?? req.headers.get("host");
	if (host) return `${proto}://${host}`;
	return new URL(req.url).origin;
}

function handle(req: NextRequest, params: URLSearchParams): NextResponse {
	const store = pickStore(params);
	const origin = siteOrigin(req);
	return new NextResponse(renderHtml(store, origin), {
		status: 200,
		headers: {
			"Content-Type": "text/html; charset=utf-8",
			"Cache-Control": "no-store",
			"X-Frame-Options": "SAMEORIGIN",
		},
	});
}

export function GET(req: NextRequest): NextResponse {
	return handle(req, new URL(req.url).searchParams);
}

export async function POST(req: NextRequest): Promise<NextResponse> {
	const contentType = req.headers.get("content-type") ?? "";
	let params: URLSearchParams;
	if (contentType.includes("application/x-www-form-urlencoded")) {
		const text = await req.text();
		params = new URLSearchParams(text);
	} else if (contentType.includes("multipart/form-data")) {
		const form = await req.formData();
		params = new URLSearchParams();
		for (const [k, v] of form.entries()) {
			if (typeof v === "string") params.set(k, v);
		}
	} else if (contentType.includes("application/json")) {
		const body = (await req.json().catch(() => ({}))) as Record<
			string,
			unknown
		>;
		params = new URLSearchParams();
		for (const [k, v] of Object.entries(body)) {
			if (v != null) params.set(k, String(v));
		}
	} else {
		params = new URL(req.url).searchParams;
	}
	return handle(req, params);
}
