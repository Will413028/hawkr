import type { Environment } from "./environment";

// 環境在 build 時固定（NEXT_PUBLIC_* 會被 inline）。dev build → "dev"、prod build → "prod"。
export const APP_ENV: Environment =
	(process.env.NEXT_PUBLIC_APP_ENV as Environment | undefined) ?? "dev";
