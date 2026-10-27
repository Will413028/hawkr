"use client";

import { useEffect, useRef } from "react";
import Button from "@/components/ui/Button";
import { PRODUCT_CERT_CATALOG } from "@/lib/products/certifications";
import type { UseProductFormResult } from "../hooks/useProductForm";

/** Once creation succeeds, this replaces the create form. Retries cannot create
 * another product or send the already-finalized gallery a second time. */
export default function ProductCreationCertifications({
	form,
}: {
	form: Pick<
		UseProductFormResult,
		| "createdProduct"
		| "submitting"
		| "createdCertificationCount"
		| "certificationDrafts"
		| "errors"
		| "submit"
		| "certErrors"
	>;
}) {
	const heading = useRef<HTMLHeadingElement>(null);
	useEffect(() => heading.current?.focus(), []);
	const id = form.createdProduct?.id;
	return (
		<div className="mx-auto w-full max-w-[640px] px-5 py-6">
			<h1
				ref={heading}
				tabIndex={-1}
				className="text-2xl font-bold text-hk-heading outline-none"
			>
				商品已建立
			</h1>
			<p className="mt-2 text-sm text-hk-muted" role="status">
				{form.submitting
					? form.certificationDrafts.length > 0
						? "正在送出認證申請，請稍候…"
						: "正在開啟商品頁面…"
					: `已確認 ${form.createdCertificationCount} 項申請結果，尚有 ${form.certificationDrafts.length} 項待確認。`}
			</p>
			{form.errors.length > 0 && (
				<div role="alert" className="my-4 text-sm text-hk-danger">
					{form.errors.map((message) => (
						<p key={message}>{message}</p>
					))}
				</div>
			)}
			<form
				className="mt-5 flex flex-col gap-4"
				aria-busy={form.submitting}
				onSubmit={(event) => {
					event.preventDefault();
					if (id) void form.submit("save");
				}}
			>
				<ul className="list-none m-0 p-0 flex flex-col gap-2">
					{form.certificationDrafts.map((entry) => {
						const item = PRODUCT_CERT_CATALOG.find(
							(item) => item.code === entry.code,
						);
						if (!item) return null;
						return (
							<li
								key={entry.code}
								className="rounded-lg border border-hk-border-soft bg-hk-surface-card px-4 py-3"
							>
								<strong className="text-sm text-hk-heading">
									{item.label}
								</strong>
								<p className="mt-1 break-all text-sm text-hk-muted">
									證書編號：{entry.certNo}
								</p>
								{form.certErrors[entry.code] && (
									<p className="mt-1 text-sm text-hk-danger">
										{form.certErrors[entry.code]}
									</p>
								)}
							</li>
						);
					})}
				</ul>
				{!form.submitting && id && (
					<p className="text-sm text-hk-muted">
						重試會確認原申請的送出結果。如需修改編號或撤回申請，請前往商品編輯。
					</p>
				)}
				<div className="flex flex-wrap gap-3">
					{id && form.certificationDrafts.length > 0 && (
						<Button
							type="submit"
							disabled={form.submitting}
							isLoading={form.submitting}
							loadingLabel="送出申請中…"
						>
							重試未完成的申請
						</Button>
					)}
					<Button
						href={
							id
								? `/seller-center/products/${encodeURIComponent(String(id))}/edit`
								: "/seller-center/products"
						}
						variant="bordered"
						disabled={form.submitting}
					>
						{id ? "前往商品編輯" : "回商品管理"}
					</Button>
				</div>
			</form>
		</div>
	);
}
