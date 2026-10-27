"use client";

import { toast } from "@/lib/feedback/toast";

export const POST_IMAGE_REQUIRED_MESSAGE = "請先補上照片";
export const POST_IMAGE_REQUIRED_DESCRIPTION =
	"每則動態至少需要 1 張照片才能發布。";

/**
 * 發布照片缺漏共用提示。固定 id 可避免使用者連點反灰按鈕時堆出多張相同通知。
 */
export function showPostImageRequiredToast(
	description = POST_IMAGE_REQUIRED_DESCRIPTION,
): void {
	toast.warning(POST_IMAGE_REQUIRED_MESSAGE, {
		description,
		id: "post-image-required",
	});
}
