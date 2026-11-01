import "@/styles/features/seller-center.css";
import "@/styles/features/delegated-operations.css";
import DelegatedRuntimeBoundary from "@/features/delegated-operations/components/DelegatedRuntimeBoundary";
import { getEnv } from "@/lib/config/env";
import { noindexMetadata } from "@/lib/seo/metadata";

/**
 * ADM 代行工作區的唯一外框（56 號 §4.5 L1027）。
 *
 * ⚠️ **不得**沿用 `app/[locale]/(seller)/seller-center/layout.tsx` 的
 * `ZoneGate required="seller"`：代行者不是賣家，在 www 上通常根本沒有登入
 * 態，那道 guard 會把他整頁踢回首頁。這裡的 gate 改由工作階段本身承擔
 * （`GET /adm/api/session/current`，見 `DelegatedSessionProvider`）。
 *
 * `seller-center.css` 必須在這裡 import：整套 `.seller-shell` 版面與它的深色覆寫
 * 都在那支檔案裡，目前只有賣家中心的 layout 引入它。
 */

export const metadata = noindexMetadata();

export default function DelegatedWorkspaceLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	return (
		<DelegatedRuntimeBoundary admOrigin={getEnv().admOrigin}>
			{children}
		</DelegatedRuntimeBoundary>
	);
}
