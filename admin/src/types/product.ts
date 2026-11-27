export interface InlineRun {
	text: string;
	bold?: boolean;
	italic?: boolean;
	underline?: boolean;
	color?: string;
	href?: string;
}

export interface DescriptionBlock {
	type: string;
	id?: string;
	runs?: InlineRun[];
	content?: string;
	tableJson?: string;
	listJson?: string;
	assetId?: string;
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
