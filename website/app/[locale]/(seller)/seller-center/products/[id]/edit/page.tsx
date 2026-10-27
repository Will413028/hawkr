import { ProductFormPage } from "@/features/seller-center/products";

export const metadata = { title: "編輯商品 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ id: string }>;
}) {
	const { id } = await params;
	// Product IDs are 64-bit snowflakes that overflow Number — keep them as
	// strings end-to-end so /app/products/{id} hits the right record.
	if (!id || !/^\d+$/.test(id)) {
		return <ProductFormPage mode="create" />;
	}
	return <ProductFormPage mode="edit" productId={id} />;
}
