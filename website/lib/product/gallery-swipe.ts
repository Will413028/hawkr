export const PRODUCT_GALLERY_DRAG_START_PX = 6;
export const PRODUCT_GALLERY_SWIPE_THRESHOLD = 48;
export const PRODUCT_GALLERY_SLIDE_GAP_PX = 12;

export function galleryIndexAfterSwipe(
	currentIndex: number,
	imageCount: number,
	deltaX: number,
) {
	if (imageCount < 2) return 0;
	if (deltaX <= -PRODUCT_GALLERY_SWIPE_THRESHOLD) {
		return Math.min(imageCount - 1, currentIndex + 1);
	}
	if (deltaX >= PRODUCT_GALLERY_SWIPE_THRESHOLD) {
		return Math.max(0, currentIndex - 1);
	}
	return currentIndex;
}
