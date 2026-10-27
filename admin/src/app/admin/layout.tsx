"use client";

import { useQuery } from "@connectrpc/connect-query";
import { IdentityAdminService } from "@hawkr/api/gen/hawkr/admin/identity/v1alpha1/identity_pb";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import Sidebar from "@/components/Layout/Sidebar";
import Topbar from "@/components/Layout/Topbar";
import { Button, ToastProvider } from "@/components/UI";
import { useEscapeKey, useFocusTrap, useModalStack } from "@/lib/a11y";

export default function AdminLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	const pathname = usePathname();
	const [mounted, setMounted] = useState(false);
	const [sidebarOpen, setSidebarOpen] = useState(false);
	const [isMobileNavigation, setIsMobileNavigation] = useState(false);
	const navigationLayerRef = useRef<HTMLDivElement>(null);
	const sidebarRef = useRef<HTMLElement>(null);
	const mobileSidebarOpen = isMobileNavigation && sidebarOpen;
	const session = useQuery(IdentityAdminService.method.getSession, {});

	useEffect(() => {
		setMounted(true);
	}, []);

	useEffect(() => {
		if (pathname) setSidebarOpen(false);
	}, [pathname]);

	useEffect(() => {
		const mediaQuery = window.matchMedia("(max-width: 960px)");
		const syncNavigationMode = () => {
			setIsMobileNavigation(mediaQuery.matches);
			if (!mediaQuery.matches) setSidebarOpen(false);
		};

		syncNavigationMode();
		mediaQuery.addEventListener("change", syncNavigationMode);
		return () => mediaQuery.removeEventListener("change", syncNavigationMode);
	}, []);

	const { isTopmost } = useModalStack(navigationLayerRef, mobileSidebarOpen);
	useEscapeKey(() => setSidebarOpen(false), mobileSidebarOpen, isTopmost);
	useFocusTrap(sidebarRef, mobileSidebarOpen, isTopmost);

	if (!mounted) {
		return null;
	}

	return (
		<ToastProvider>
			<div className="admin-app show">
				<a className="admin-skip-link" href="#admin-main-content">
					跳到主要內容
				</a>
				<div ref={navigationLayerRef} className="admin-navigation-layer">
					<Sidebar
						ref={sidebarRef}
						open={sidebarOpen}
						mobile={isMobileNavigation}
						onClose={() => setSidebarOpen(false)}
					/>
					{mobileSidebarOpen && (
						<button
							type="button"
							className="admin-sidebar-scrim"
							onClick={() => setSidebarOpen(false)}
							aria-label="關閉主選單"
							tabIndex={-1}
						/>
					)}
				</div>
				<div className="main-content">
					<Topbar onMenuClick={() => setSidebarOpen(true)} />
					<main id="admin-main-content" className="page-content" tabIndex={-1}>
						{session.isPending ? (
							<p className="page-subtitle" role="status">
								載入管理員資料中…
							</p>
						) : session.isError ? (
							<div role="alert">
								<h1 className="page-title">無法載入管理後台</h1>
								<p className="page-subtitle">服務暫時不可用，請稍後重試。</p>
								<Button onClick={() => void session.refetch()}>重試</Button>
							</div>
						) : (
							children
						)}
					</main>
				</div>
			</div>
		</ToastProvider>
	);
}
