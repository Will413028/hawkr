/**
 * Storybook 裡的 `@clerk/nextjs` 替身（由 main.ts 的 viteFinal 以 alias 換入）。
 *
 * 真的 `useClerk`／`useAuth` 在沒有 `<ClerkProvider>` 時會直接丟錯，而 Clerk 需要網路與 application key，
 * 預覽不該連出去。這裡的 Clerk 永遠是「已載入、未登入」，讓 `AuthProvider` 這類真元件能照常掛載；
 * 需要登入身分的預覽改用 `lib/hooks/useQueryIdentity` 的 mock，不靠這裡。
 */
import type { ReactNode } from "react";

export function ClerkProvider({ children }: { children?: ReactNode }) {
	return children;
}

export function useClerk() {
	return {
		loaded: true,
		signOut: async (callback?: () => void) => {
			callback?.();
		},
	};
}

export function useAuth() {
	return {
		isLoaded: true,
		isSignedIn: false,
		userId: null,
		sessionId: null,
		getToken: async () => null,
	};
}

export function useUser() {
	return { isLoaded: true, isSignedIn: false, user: null };
}

export function useSignIn() {
	return { isLoaded: true, signIn: undefined, setActive: undefined };
}

export function AuthenticateWithRedirectCallback() {
	return null;
}
