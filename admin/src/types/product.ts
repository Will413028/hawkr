export interface InlineRun {
	text: string;
	bold?: boolean;
	italic?: boolean;
	underline?: boolean;
	color?: string;
	href?: string;
}

export interface DescriptionBlock {
	type: string; // "text" | "table" | "list" | "image"
	id?: string;
	runs?: InlineRun[];
	content?: string;
	tableJson?: string;
	listJson?: string;
	assetId?: string;
	// 49 §6.1：帶一張圖的物件一律 imageKey（objectKey SSOT）+ imageUrl（Gateway 組）。
	imageKey?: string;
	imageUrl?: string;
	width?: string;
	height?: string;
	alt?: string;
}

export interface ProductDescription {
	version?: string;
	blocks?: DescriptionBlock[];
}

export interface GalleryItem {
	assetId?: string;
	// 49 §6.1：imageKey 是 objectKey SSOT，imageUrl 是 Gateway 由它組出的顯示 URL
	// （固定 `/i/original/`）。兩欄在 gateway `GalleryItem` 都帶 omitempty，且成對
	// 產生（imageUrl 由 imageKey 組），所以顯示一律讀 imageKey 過 buildImageUrl 取
	// 需要的 preset；讀 imageUrl 等於先拉原圖 URL 再剝回 key，多一次無謂轉換。
	imageKey?: string;
	imageUrl?: string;
}

export interface ShippingMethod {
	type: string;
	freeThreshold: string;
	feePayer: string;
}

export interface ProductSku {
	id: string;
	skuName: string;
	weight?: string;
	unit: string;
	price: string;
	salePrice?: string;
	stockQty: string;
	lengthCm?: string;
	widthCm?: string;
	heightCm?: string;
	shippingMethods?: ShippingMethod[];
}

export interface ProductDetail {
	id: string;
	name: string;
	status: string;
	origin?: string;
	rating?: string;
	description?: ProductDescription;
	imageKey?: string;
	imageUrl?: string;
	imageKeys?: string[];
	imageUrls?: string[];
	gallery?: GalleryItem[];
	categoryId?: string;
	categoryName?: string;
	categoryLv1Id?: string;
	categoryLv1Name?: string;
	categoryLv2Id?: string;
	categoryLv2Name?: string;
	categoryLv3Id?: string;
	categoryLv3Name?: string;
	priceMin?: string;
	priceMax?: string;
	listPriceMin?: string;
	listPriceMax?: string;
	salePriceMin?: string;
	salePriceMax?: string;
	stock: string;
	salesCount?: string;
	favoriteCount: string;
	reviewCount: string;
	shippingMethods?: ShippingMethod[];
	skus?: ProductSku[];
	createdAt?: string;
	updatedAt?: string;
	createdBy?: string;
}
