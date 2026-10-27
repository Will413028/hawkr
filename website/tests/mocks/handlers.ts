import { HttpResponse, http } from "msw";

export const handlers = [
	http.get("*/health", () => HttpResponse.json({ ok: true })),
];
