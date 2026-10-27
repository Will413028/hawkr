const SAFE_METHODS = new Set(["GET", "HEAD", "OPTIONS"]);

/**
 * 後台自己的 route（`/api/*`：BFF、代行）只給後台的頁面呼叫：非安全方法要是同源的請求。判準與後端
 * 的 `http.CrossOriginProtection` 相同：有 `Sec-Fetch-Site` 時看它；沒有時比對 `Origin` 與自己的 origin；兩者
 * 都沒有的不是瀏覽器發的，放行。
 *
 * 這一段只能在這裡擋：後端收到的是 BFF 的伺服器請求，看不到瀏覽器的來源；而同一個網站的其他子網域
 * （例如前台）發起的請求仍可能帶上 Access cookie。
 */
export function isCrossOriginWrite(
	method: string,
	headers: Headers,
	ownOrigin: string,
): boolean {
	if (SAFE_METHODS.has(method.toUpperCase())) return false;
	const site = headers.get("sec-fetch-site");
	if (site !== null) return site !== "same-origin" && site !== "none";
	const origin = headers.get("origin");
	if (origin !== null) return origin !== ownOrigin;
	return false;
}
