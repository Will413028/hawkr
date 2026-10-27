import { headers } from "next/headers";
import LoginPage from "@/features/auth/screens/login/LoginPage";
import { detectInAppBrowser } from "@/lib/browser/in-app-browser";

export const metadata = { title: "登入" };

export default async function LoginRoute() {
	const requestHeaders = await headers();
	const userAgent = requestHeaders.get("user-agent") ?? "";

	return <LoginPage initialInAppBrowser={detectInAppBrowser(userAgent)} />;
}
