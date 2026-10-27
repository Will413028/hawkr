import type { Favorite } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";

/** The overview rail needs an active product with a name and image. */
export function isDisplayableFavorite(favorite: Favorite): boolean {
	const product = favorite.product;
	return Boolean(
		product &&
			!product.deleted &&
			product.available &&
			product.name.trim() &&
			product.image?.uriTemplate.trim(),
	);
}

export function displayableFavorites(items: Favorite[]): Favorite[] {
	return items.filter(isDisplayableFavorite);
}

/** The full list retains deleted products so the existing unavailable card can show them. */
export function isBrowsableFavorite(favorite: Favorite): boolean {
	return Boolean(favorite.product?.id);
}

export function browsableFavorites(items: Favorite[]): Favorite[] {
	return items.filter(isBrowsableFavorite);
}
