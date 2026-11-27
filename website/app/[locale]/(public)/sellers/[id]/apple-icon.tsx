import { ImageResponse } from "next/og";
import { getCachedStore } from "@/lib/api/cached";
import { isConfiguredMediaUrl, resolveCdnUrl } from "@/lib/media/cdn";

export const size = { width: 180, height: 180 };
export const contentType = "image/png";


type SellerLike = {
	logo_url?: string;
} | null;

const MAX_AVATAR_BYTES = 2 * 1024 * 1024;
const FETCH_TIMEOUT_MS = 3000;

/**
 * 把商家頭像轉成 data URI 餵給 Satori。任何失敗（無頭像 / 非允許 host /
 * 抓取失敗 / 逾時 / 過大 / 非圖片）都回 null，呼叫端保留空白品牌底色。
 */
async function loadAvatarDataUri(seller: SellerLike): Promise<string | null> {
	const raw = resolveCdnUrl(seller?.logo_url?.trim() || "");
	if (!raw.startsWith("http")) return null;

	let url: URL;
	try {
		url = new URL(raw);
	} catch {
		return null;
	}
	if (!isConfiguredMediaUrl(url)) return null;

	try {
		const res = await fetch(url, {
			redirect: "error",
			signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
		});
		if (!res.ok) return null;
		const type = res.headers.get("content-type") || "";
		if (!type.startsWith("image/")) return null;

		const reader = res.body?.getReader();
		if (!reader) return null;
		const chunks: Uint8Array[] = [];
		let total = 0;
		while (true) {
			const { done, value } = await reader.read();
			if (done) break;
			total += value.byteLength;
			if (total > MAX_AVATAR_BYTES) {
				await reader.cancel();
				return null;
			}
			chunks.push(value);
		}
		const body = new Uint8Array(total);
		let offset = 0;
		for (const chunk of chunks) {
			body.set(chunk, offset);
			offset += chunk.byteLength;
		}

		const base64 = Buffer.from(body).toString("base64");
		return `data:${type};base64,${base64}`;
	} catch {
		return null;
	}
}

export default async function Icon({
	params,
}: {
	params: Promise<{ id: string }>;
}) {
	const { id } = await params;
	const seller = (await getCachedStore(id)) as SellerLike;
	const avatar = await loadAvatarDataUri(seller);

	return new ImageResponse(
		<div
			style={{
				width: "100%",
				height: "100%",
				display: "flex",
				alignItems: "center",
				justifyContent: "center",
				background: "#b8441f",
			}}
		>
			{avatar ? (
				// biome-ignore lint/performance/noImgElement: ImageResponse (Satori) only renders <img>, not next/image
				<img
					src={avatar}
					width={180}
					height={180}
					alt=""
					style={{ width: "100%", height: "100%", objectFit: "cover" }}
				/>
			) : null}
		</div>,
		{ ...size },
	);
}
