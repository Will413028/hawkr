import AdminBrand from "@/components/Layout/AdminBrand";

export default function SessionExpiredPage() {
	return (
		<main className="page-content">
			<AdminBrand priority />
			<div role="alert">
				<h1 className="page-title">本機管理員憑證已失效</h1>
				<p className="page-subtitle">
					在 backend 執行 make dev-token args='-admin -user dev_ops'，更新
					admin/.env.local 的 ADMIN_DEV_TOKEN 並重啟後台。
				</p>
				<a className="btn btn-primary" href="/admin">
					更新憑證後重試
				</a>
			</div>
		</main>
	);
}
