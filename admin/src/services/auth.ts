"use client";

import { Code, ConnectError, createClient } from "@connectrpc/connect";
import { IdentityAdminService } from "@hawkr/api/gen/hawkr/admin/identity/v1alpha1/identity_pb";
import { adminTransport } from "@/lib/connect/transport";

const LOGOUT_FAILED = "登出失敗，請再試一次";
const LOGOUT_PATH = "/api/admin/logout";
let expiring: Promise<void> | null = null;

async function revokeDelegations(): Promise<void> {
	const client = createClient(IdentityAdminService, adminTransport());
	await client.signOut({});
}

export const AuthService = {
	async logout(): Promise<void> {
		try {
			await revokeDelegations();
		} catch (err) {
			const code = ConnectError.from(err).code;
			if (code !== Code.Unauthenticated && code !== Code.PermissionDenied) {
				alert(LOGOUT_FAILED);
				return;
			}
		}
		window.location.href = LOGOUT_PATH;
	},

	sessionExpired(): void {
		if (expiring) return;
		expiring = AuthService.logout().finally(() => {
			expiring = null;
		});
	},
};
