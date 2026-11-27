import { ProductFormPage } from "@/features/seller-center/products";

export const metadata = { title: "編輯商品 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ id: string }>;
}) {
	const { id } = await params;
	if (!id || !/^\d+$/.test(id)) {
		return <ProductFormPage mode="create" />;
	}
	return <ProductFormPage mode="edit" productId={id} />;
}
