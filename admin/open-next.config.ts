// open-next.config.ts
import { defineCloudflareConfig } from "@opennextjs/cloudflare";

// admin 全 client component、無 ISR/SSG 動態快取 → 不掛 R2/DO，用 default。
export default defineCloudflareConfig();
