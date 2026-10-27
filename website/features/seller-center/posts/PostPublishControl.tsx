"use client";

import Button from "@/components/ui/Button";
import { PostPublishStatus } from "@/lib/codemeta/enums";
import {
	POST_IMAGE_REQUIRED_MESSAGE,
	showPostImageRequiredToast,
} from "./postImageRequirement";
import {
	isExpandingPublishTarget,
	POST_PUBLISH_ACTION_LABEL,
	POST_PUBLISH_PENDING_LABEL,
	type PostPublishTarget,
	postPublishToggleTarget,
} from "./postPublishActions";

export interface PostPublishControlProps {
	/** `PostListItem.publishStatus`（4401／4402／4403，缺值＝上游沒給）。 */
	publishStatus?: string;
	/** 這則貼文被平台強制下架（`hiddenAt > 0`）。與發布狀態是兩件事。 */
	hidden?: boolean;
	/** 賣家目前處於停權（`useSellerIdentity().isSuspended`）。 */
	suspended?: boolean;
	/** `PostListItem.imageKeys[0]` 是否存在；只有發布方向需要照片。 */
	hasImage: boolean;
	busy?: boolean;
	onToggle: (target: PostPublishTarget) => void;
}

/**
 * 賣家自己那條路徑的發布／下架入口。
 *
 * **一顆按鈕、方向由狀態決定**，與商品那側的 `ShelfStatusControl` 同構：未發布
 * （4401）與已下架（4403）給「發布」，已發布（4402）給「下架」，而**沒有任何組合
 * 會產出「改回未發布」** —— 那不是這裡省略掉的選項，是端點根本不接受的值
 * （`postPublishToggleTarget` 的回傳型別已排除 4401）。
 *
 * 為什麼不做成可點的狀態 pill（商品那側的形狀）：貼文的 `PostPublishStatusBadge`
 * 對「已發布」刻意不畫任何 pill（已發布是常態，標記它會把真正需要注意的兩態淹掉）。
 * 把 pill 改成控制項就得為已發布補一顆 pill，等於推翻那個判斷，而本次要補的是
 * 「摸不到的入口」，不是重排列表的視覺密度。所以入口放在既有的動作欄，與編輯／
 * 刪除同一排。
 *
 * 三種「不能發布」的處置刻意不同：
 *   - **停權**（能力層沒授予 `CAP_MARKETING`）⇒ 整顆不渲染。沒有核發的權限就不
 *     出現 CTA，與代行工作區「有 read、沒有 write ⇒ 不出現 write CTA」同一條
 *     （`productsCapabilityCopy.ts`）。
 *   - **平台下架**（`hiddenAt`）⇒ 渲染但停用並說明原因。權限在、動作合法，只是
 *     發布了買家仍然看不到（social 的 `UpdatePublishStatus` 對 hidden 的貼文
 *     `delta = 0`）——這是「按了沒有效果」，要說出來而不是讓人找不到按鈕。
 *   - **缺照片** ⇒ 呈現反灰的 `aria-disabled` 狀態但保留 focus/click，觸發時提示先
 *     編輯補圖，絕不呼叫 onToggle。原生 disabled 收不到事件，無法完成這段說明。
 *
 * 三種情況都**只擋發布方向**，下架一律留著：停權賣家必須留得住自清的手（46 §3.3
 * 收縮型白名單，端點掛的正是 `CAP_CONTENT_CONTRACT`）。
 */
export default function PostPublishControl({
	publishStatus,
	hidden = false,
	suspended = false,
	hasImage,
	busy = false,
	onToggle,
}: PostPublishControlProps) {
	const target = postPublishToggleTarget(publishStatus);
	// 缺值＝上游沒給。畫一顆方向不明的按鈕，比不畫更糟：按錯方向的 4403 是回不去的。
	if (target === null) return null;

	const expanding = isExpandingPublishTarget(target);
	if (expanding && suspended) return null;

	const imageRequired = expanding && !hasImage;
	const blockedReason =
		expanding && hidden ? "已被平台下架，發布後買家仍看不到" : undefined;
	const unavailableReason =
		blockedReason ??
		(imageRequired ? `${POST_IMAGE_REQUIRED_MESSAGE}後再發布` : undefined);
	const label = POST_PUBLISH_ACTION_LABEL[target];
	const pendingLabel = POST_PUBLISH_PENDING_LABEL[target];

	return (
		<Button
			type="button"
			className="post-publish-control"
			variant="bordered"
			color={expanding && !imageRequired ? "primary" : "default"}
			size="sm"
			disabled={busy || Boolean(blockedReason)}
			aria-disabled={imageRequired || undefined}
			data-image-required={imageRequired || undefined}
			isLoading={busy}
			loadingLabel={pendingLabel}
			// 兩個方向的 pending 字寬不同，閒置時就預留最寬的那個，避免送出瞬間整排位移。
			loadingReservationLabel={
				POST_PUBLISH_PENDING_LABEL[PostPublishStatus.UNPUBLISHED]
			}
			title={unavailableReason}
			aria-label={
				unavailableReason ? `${label}（${unavailableReason}）` : label
			}
			onClick={() => {
				if (imageRequired) {
					showPostImageRequiredToast(
						"請先編輯貼文並加入至少 1 張照片，再回來發布。",
					);
					return;
				}
				onToggle(target);
			}}
		>
			{label}
		</Button>
	);
}
