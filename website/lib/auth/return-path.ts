/**
 * middleware 在受保護路徑上設的 request header：訪客要求的 URL（含 query），登入頁用它當 `redirect`。
 * 伺服器端 layout 讀不到 pathname 與 query，而 DAL 判定「cookie 在但單體拒絕」時要把訪客送回他要的網址。
 * 只由 middleware 寫（同名的 client header 在受保護路徑上被覆寫），讀的人一律再過 `safeRedirectPath`。
 * edge-safe（middleware 也 import）。
 */
export const RETURN_PATH_HEADER = "x-hk-return-path";
