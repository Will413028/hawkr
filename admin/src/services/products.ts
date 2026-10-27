import type { ProductDetail } from "@/types/product";
import { api } from "./api";

interface ProductDetailResponse {
	code: number;
	data: ProductDetail;
}

export async function getProductDetail(
	id: string,
): Promise<ProductDetail | null> {
	const res = await api.get<ProductDetailResponse>(
		`/api/v1/admin/products/${id}`,
	);
	return res?.data ?? null;
}
