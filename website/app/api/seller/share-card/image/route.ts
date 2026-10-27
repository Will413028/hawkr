import { type NextRequest, NextResponse } from "next/server";
import { isConfiguredMediaUrl } from "@/lib/media/cdn";

const MAX_IMAGE_BYTES = 8 * 1024 * 1024;

// The proxy fetches server-side, so the only allowed host is the configured
// media origin (isConfiguredMediaUrl); anything else would make it an open image proxy.
export async function GET(request: NextRequest) {
	const rawUrl = request.nextUrl.searchParams.get("url");
	if (!rawUrl) {
		return NextResponse.json({ error: "Missing image URL" }, { status: 400 });
	}

	let imageUrl: URL;
	try {
		imageUrl = new URL(rawUrl);
	} catch {
		return NextResponse.json({ error: "Invalid image URL" }, { status: 400 });
	}

	if (!isConfiguredMediaUrl(imageUrl)) {
		return NextResponse.json(
			{ error: "Image host is not allowed" },
			{ status: 400 },
		);
	}

	const response = await fetch(imageUrl, {
		cache: "force-cache",
		redirect: "error",
		headers: {
			accept:
				"image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
		},
	});
	if (!response.ok) {
		return NextResponse.json(
			{ error: "Unable to fetch image" },
			{ status: 502 },
		);
	}

	const contentType = response.headers.get("content-type") ?? "";
	if (!contentType.toLowerCase().startsWith("image/")) {
		return NextResponse.json({ error: "URL is not an image" }, { status: 415 });
	}

	const reader = response.body?.getReader();
	if (!reader) {
		return NextResponse.json(
			{ error: "Unable to fetch image" },
			{ status: 502 },
		);
	}

	const chunks: Uint8Array[] = [];
	let total = 0;
	while (true) {
		const { done, value } = await reader.read();
		if (done) break;
		total += value.byteLength;
		if (total > MAX_IMAGE_BYTES) {
			await reader.cancel();
			return NextResponse.json(
				{ error: "Image is too large" },
				{ status: 413 },
			);
		}
		chunks.push(value);
	}

	const body = new Uint8Array(total);
	let offset = 0;
	for (const chunk of chunks) {
		body.set(chunk, offset);
		offset += chunk.byteLength;
	}

	return new NextResponse(body, {
		headers: {
			"cache-control": "public, max-age=86400, stale-while-revalidate=604800",
			"content-type": contentType,
		},
	});
}
