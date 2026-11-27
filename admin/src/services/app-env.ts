import type { Environment } from "./environment";

export const APP_ENV: Environment =
	(process.env.NEXT_PUBLIC_APP_ENV as Environment | undefined) ?? "dev";
