"use client";

import * as React from "react";
import BrandLogoLoader from "@/components/shared/BrandLogoLoader";
import Button from "@/components/ui/Button";
import { useAuth } from "@/lib/auth/context";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import { cn } from "@/lib/utils/cn";

export type SignOutAction = () => Promise<boolean>;

export interface SellerMenuChrome {
	root: string;
	trigger: string;
	avatar: string;
	name: string;
	chevron: string;
}

export interface SellerMenuProps {
	onSignOut: SignOutAction;
	chrome: SellerMenuChrome;
}

export interface SellerNavbarModule {
	SellerHeader: React.ComponentType;
	SellerMenu: React.ComponentType<SellerMenuProps>;
}

export interface SellerNavbarResource {
	peek: () => SellerNavbarModule | null;
	load: () => Promise<SellerNavbarModule>;
}

/**
 * Shares the seller-only navigation module across the workspace header and the
 * brief seller account rail shown before SellerWorkspaceLock redirects. Failed
 * chunk requests are evicted so the visible retry can make a fresh request.
 */
export function createSellerNavbarResource(
	importer: () => Promise<Partial<SellerNavbarModule>>,
): SellerNavbarResource {
	let loaded: SellerNavbarModule | null = null;
	let inFlight: Promise<SellerNavbarModule> | null = null;

	const load = (): Promise<SellerNavbarModule> => {
		if (loaded) return Promise.resolve(loaded);
		if (inFlight) return inFlight;

		let importRequest: Promise<Partial<SellerNavbarModule>>;
		try {
			importRequest = importer();
		} catch (error) {
			return Promise.reject(error);
		}

		const imported = importRequest.then((module) => {
			if (!module?.SellerHeader || !module?.SellerMenu) {
				throw new Error("Seller navbar module is missing required exports");
			}
			return module as SellerNavbarModule;
		});
		const tracked = imported.then(
			(module) => {
				if (inFlight === tracked) {
					loaded = module;
					inFlight = null;
				}
				return module;
			},
			(error: unknown) => {
				if (inFlight === tracked) inFlight = null;
				throw error;
			},
		);
		inFlight = tracked;
		return tracked;
	};

	return {
		peek: () => loaded,
		load,
	};
}

export const sellerNavbarResource = createSellerNavbarResource(
	() => import("@/components/shared/SellerNavbar"),
);

type LoadState =
	| { status: "loading"; attempt: number }
	| { status: "ready"; module: SellerNavbarModule }
	| { status: "error" };

function useSellerNavbarModule(resource: SellerNavbarResource) {
	const [attempt, setAttempt] = React.useState(0);
	const [state, setState] = React.useState<LoadState>(() => {
		const cached = resource.peek();
		return cached
			? { status: "ready", module: cached }
			: { status: "loading", attempt };
	});

	React.useEffect(() => {
		const cached = resource.peek();
		if (cached) {
			setState({ status: "ready", module: cached });
			return;
		}

		let active = true;
		setState({ status: "loading", attempt });
		void resource.load().then(
			(module) => {
				if (active) setState({ status: "ready", module });
			},
			() => {
				if (active) setState({ status: "error" });
			},
		);
		return () => {
			active = false;
		};
	}, [attempt, resource]);

	return {
		state,
		retry: () => setAttempt((value) => value + 1),
	};
}

export function SellerHeaderLoader({
	resource = sellerNavbarResource,
	onboarding = false,
}: {
	resource?: SellerNavbarResource;
	onboarding?: boolean;
}) {
	const { signOut } = useAuth();
	const { state, retry } = useSellerNavbarModule(resource);
	const [isSigningOut, setIsSigningOut] = React.useState(false);
	const signOutPendingRef = React.useRef(false);

	const handleSignOut = React.useCallback(async () => {
		if (signOutPendingRef.current) return;
		signOutPendingRef.current = true;
		setIsSigningOut(true);
		try {
			await signOut();
		} catch {
			// signOut owns the toast; keep this fail-closed rail available to retry.
			return;
		} finally {
			signOutPendingRef.current = false;
			setIsSigningOut(false);
		}
		commitDocumentNavigation("/");
	}, [signOut]);

	if (state.status === "ready" && !isSigningOut) {
		const SellerHeader = state.module.SellerHeader;
		return <SellerHeader />;
	}

	const isError = state.status === "error";

	return (
		<nav
			aria-label="賣家工作區導覽"
			className={cn(
				"root-nav-layout-reservation flex w-full items-center justify-end gap-2 px-4",
				// Match navigation-shell.css's existing narrow onboarding account rail.
				onboarding && "max-[640px]:h-[72px]",
			)}
			data-onboarding={onboarding || undefined}
			data-seller-navbar-load-state={isError ? "error" : "loading"}
			style={{ minHeight: "var(--floating-nav-height, 84px)" }}
		>
			{isError ? (
				<span className="mr-auto text-sm text-hk-muted" role="alert">
					賣家導覽載入失敗
				</span>
			) : (
				<span
					className="mr-auto inline-flex items-center gap-2 text-sm text-hk-muted"
					role="status"
				>
					<BrandLogoLoader size={32} />
					<span className="sr-only">賣家導覽載入中</span>
				</span>
			)}
			{isError && (
				<Button
					data-seller-navbar-retry
					size="md"
					variant="light"
					disabled={isSigningOut}
					onClick={retry}
				>
					重試
				</Button>
			)}
			<Button
				data-seller-navbar-logout
				size="md"
				variant="light"
				color="danger"
				isLoading={isSigningOut}
				loadingLabel="登出中…"
				onClick={() => void handleSignOut()}
			>
				登出
			</Button>
		</nav>
	);
}

export function SellerMenuLoader({
	onSignOut,
	chrome,
	resource = sellerNavbarResource,
}: SellerMenuProps & { resource?: SellerNavbarResource }) {
	const { state, retry } = useSellerNavbarModule(resource);
	const [isSigningOut, setIsSigningOut] = React.useState(false);
	const signOutPendingRef = React.useRef(false);

	const handleSignOut = React.useCallback(async () => {
		if (signOutPendingRef.current) return;
		signOutPendingRef.current = true;
		setIsSigningOut(true);
		try {
			await onSignOut();
		} catch {
			// Keep the fail-closed escape rail mounted; the owner reports the error.
			return;
		} finally {
			signOutPendingRef.current = false;
			setIsSigningOut(false);
		}
	}, [onSignOut]);

	if (state.status === "ready" && !isSigningOut) {
		const SellerMenu = state.module.SellerMenu;
		return <SellerMenu onSignOut={onSignOut} chrome={chrome} />;
	}

	const isError = state.status === "error";

	return (
		<div
			className={cn(chrome.root, "flex items-center gap-1")}
			data-seller-menu-load-state={isError ? "error" : "loading"}
		>
			{isError ? (
				<>
					{/* The alert is its own node: nesting role="alert" INSIDE the
					    button made the failure message double as the button's label,
					    so assistive tech read the error and the action as one string. */}
					<span role="alert" className="sr-only">
						賣家選單載入失敗
					</span>
					<Button
						data-seller-menu-retry
						size="md"
						variant="light"
						disabled={isSigningOut}
						onClick={retry}
					>
						重試選單
					</Button>
				</>
			) : (
				<button
					type="button"
					className={chrome.trigger}
					aria-label="賣家選單載入中"
					disabled
				>
					<span
						className={cn(chrome.avatar, "inline-grid place-items-center")}
						aria-hidden="true"
					>
						<BrandLogoLoader size={20} />
					</span>
					<span className={chrome.name}>賣家選單</span>
				</button>
			)}
			<Button
				data-seller-menu-logout
				size="md"
				variant="light"
				color="danger"
				isLoading={isSigningOut}
				loadingLabel="登出中…"
				onClick={() => void handleSignOut()}
			>
				登出
			</Button>
		</div>
	);
}
