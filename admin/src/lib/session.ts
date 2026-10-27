export const adminSession = {
	async token(request: Request): Promise<string | null> {
		const accessToken = request.headers.get("Cf-Access-Jwt-Assertion")?.trim();
		if (accessToken) return accessToken;
		return process.env.ADMIN_DEV_TOKEN?.trim() || null;
	},
};
