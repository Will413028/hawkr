"use client";

import { api, request } from "./api";

const PRESIGNED_PUT_TIMEOUT_MS = 120_000;

/**
 * Upload an image through the Media Service staged-upload flow and return the
 * resulting stagedToken (design book 49 §media staging).
 *
 * Shared by the admin editors that upload images (§15 重複實作防護 — extracted
 * from the original inline copy in BannerEditorClient):
 *   1. Create an upload session  -> sessionId + presigned uploadUrl
 *   2. PUT the file bytes to the presigned URL
 *   3. Complete the session      -> stagedToken (fed to the create/update RPC)
 */
export async function uploadImageToMedia(
	file: File,
	options: { isSecure?: boolean } = {},
): Promise<string> {
	const sessionResp = await api.post<{
		code: number;
		data: { sessionId?: string; uploadUrl?: string };
	}>("/api/v1/app/media/upload_sessions", {
		filename: file.name,
		contentType: file.type,
		fileSize: String(file.size),
		// 🔴 機敏檔（發票取消證據：財政部平台截圖／光貿對帳檔）必須走 secure 資產
		// （backend #3108）。預設 false 維持既有 Banner／食譜的行為不變——
		// 那些是公開圖，走 secure 會讓它們需要簽章才看得到。
		isSecure: options.isSecure ?? false,
	});
	const sessionId = sessionResp?.data?.sessionId;
	if (!sessionId) {
		throw new Error("建立上傳會話失敗：缺少 sessionId");
	}

	// 🔴 機敏檔走**另一條**上傳路徑，不是「同一條加個旗標」（backend #3108，2026-09-05 實測）。
	//
	// media-service 的 UploadSessionService 逐字寫著「僅公開上傳提供簽名 URL」——
	// `isSecure: true` 的會話**不回 uploadUrl**，機敏檔要 PUT 到
	// `/app/media/secure_upload/{sessionId}`，而且**那一支直接回 stagedToken**，
	// 沒有第三步的 complete。
	//
	// 少了這條分支的症狀不是「傳得比較慢」而是**一定失敗**：原本的寫法會在
	// `!uploadUrl` 那一格丟出「建立上傳會話失敗：缺少 sessionId 或 uploadUrl」，
	// 而發票取消證據（A4 人工解鎖）是目前唯一 isSecure:true 的呼叫端 ⇒
	// #3108 的證據檔在 ADM 上從來沒有傳上去過。
	// dev 實測：同一個檔案改走 secure_upload ⇒ 端點 54 換發出 objectKey、
	// 覆核端下載回來位元組與原檔完全相同。
	if (options.isSecure) {
		// 走 `request` 而不是 `api.put`：後者對 body 做 `JSON.stringify`，
		// 二進位會被轉成 `{}`。這裡要送的是原始位元組。
		const secureResp = await request<{
			code: number;
			data: { stagedToken?: string };
		}>(`/api/v1/app/media/secure_upload/${sessionId}`, {
			method: "PUT",
			body: await file.arrayBuffer(),
			headers: { "Content-Type": file.type },
		});
		const secureToken = secureResp?.data?.stagedToken;
		if (!secureToken) {
			throw new Error("機敏檔上傳失敗：缺少 stagedToken");
		}
		return secureToken;
	}

	let uploadUrl = sessionResp?.data?.uploadUrl as string;
	if (!uploadUrl) {
		throw new Error("建立上傳會話失敗：缺少 uploadUrl");
	}

	if (uploadUrl.startsWith("http://")) {
		uploadUrl = uploadUrl.replace("http://", "https://");
	}

	const fileBytes = await file.arrayBuffer();
	const uploadResp = await fetch(uploadUrl, {
		method: "PUT",
		headers: { "Content-Type": file.type },
		body: fileBytes,
		// presigned URL 是第三方儲存網域：不帶 cookie、不送 referrer，逾時不讓請求掛住。
		credentials: "omit",
		referrerPolicy: "no-referrer",
		signal: AbortSignal.timeout(PRESIGNED_PUT_TIMEOUT_MS),
	});
	if (!uploadResp.ok) {
		throw new Error(`上傳圖片至儲存服務失敗 (${uploadResp.status})`);
	}

	const completeResp = await api.post<{
		code: number;
		data: { stagedToken?: string };
	}>(`/api/v1/app/media/upload_sessions/${sessionId}/complete`, {});
	const stagedToken = completeResp?.data?.stagedToken;
	if (!stagedToken) {
		throw new Error("完成上傳會話失敗：缺少 stagedToken");
	}
	return stagedToken;
}
