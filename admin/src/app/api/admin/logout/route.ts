export function GET(request: Request): Response {
	const access = request.headers.get("Cf-Access-Jwt-Assertion")?.trim();
	const destination = access ? "/cdn-cgi/access/logout" : "/session-expired";
	return new Response(null, {
		status: 303,
		headers: {
			Location: new URL(destination, request.url).toString(),
			"Cache-Control": "no-store",
		},
	});
}
