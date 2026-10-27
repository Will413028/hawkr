import type { Metadata } from "next";
import localFont from "next/font/local";
import Providers from "@/components/Providers";
import "./globals.css";
import "@/styles/AdminVisualRefresh.css";

const roboto = localFont({
	src: "./fonts/roboto-latin.woff2",
	weight: "100 900",
	style: "normal",
	variable: "--font-roboto",
	display: "optional",
	preload: false,
});

const openhuninn = localFont({
	src: "./fonts/hawkr-rounded-2.0-subset.woff2",
	weight: "400 900",
	variable: "--font-openhuninn",
	display: "optional",
	preload: false,
});

export const metadata: Metadata = {
	title: "Hawkr｜平台營運管理",
	description: "Hawkr 平台營運管理後台",
	icons: {
		icon: "/brand/logo-mark.svg",
		apple: "/brand/logo-mark.svg",
	},
};

export default function RootLayout({
	children,
}: Readonly<{
	children: React.ReactNode;
}>) {
	return (
		<html
			lang="zh-Hant"
			className={`${roboto.variable} ${openhuninn.variable}`}
		>
			<body>
				<Providers>{children}</Providers>
			</body>
		</html>
	);
}
