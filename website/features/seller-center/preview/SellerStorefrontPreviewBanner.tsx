"use client";

import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { SellerMobileDisclosureList } from "@/features/seller-center/components/SellerMobileDisclosureList";

const PREVIEW_NOTICE = [{ id: "buyer-view-preview" }] as const;

function previewNoticeKey(item: (typeof PREVIEW_NOTICE)[number]): string {
	return item.id;
}

function PreviewIcon() {
	return <AppIcon name="preview" size={18} />;
}

/**
 * Seller-only preview notice. The buyer storefront rendered below this notice
 * deliberately stays untouched: changing its product, post, forum, or review
 * cards would make 「買家視角預覽」 stop matching the real public storefront.
 */
export default function SellerStorefrontPreviewBanner() {
	return (
		<>
			<section
				className="seller-storefront-preview__banner seller-mobile-disclosure-desktop mx-auto mb-4 flex w-[calc(100%_-_48px)] max-w-[1440px] flex-wrap items-center justify-between gap-3 rounded-2xl border border-hkui-primary/15 bg-hkui-primary/6 px-4 py-3 max-[768px]:w-[calc(100%_-_28px)] max-[768px]:max-w-[680px]"
				aria-label="買家視角預覽狀態"
			>
				<div className="flex min-w-0 items-center gap-3">
					<span
						className="grid size-9 shrink-0 place-items-center rounded-full bg-hkui-primary text-hkui-primary-foreground"
						aria-hidden="true"
					>
						<PreviewIcon />
					</span>
					<div className="min-w-0">
						<p className="m-0 text-sm font-bold text-foreground">
							買家視角預覽
						</p>
						<p className="m-0 text-xs text-foreground-muted">
							使用正式公開商店資料；預覽期間不會送出訂閱、聊天、收藏、按讚、留言、分享或購買操作。
						</p>
					</div>
				</div>
				<Button href="/seller-center/me" variant="bordered" size="sm">
					編輯商店資訊
				</Button>
			</section>

			<div className="mx-auto mb-4 hidden w-[calc(100%_-_28px)] max-w-[680px] max-[767px]:block">
				<SellerMobileDisclosureList
					className="w-full"
					items={PREVIEW_NOTICE}
					getKey={previewNoticeKey}
					ariaLabel="買家視角預覽狀態"
					getSummary={() => ({
						label: "買家視角預覽說明",
						leading: <PreviewIcon />,
						title: "買家視角預覽",
						status: "唯讀",
					})}
					renderDetails={() => (
						<>
							<div className="seller-mobile-disclosure__body">
								<p className="m-0 text-sm leading-6 text-foreground-muted">
									使用正式公開商店資料；預覽期間不會送出訂閱、聊天、收藏、按讚、留言、分享或購買操作。
								</p>
							</div>
							<div className="seller-mobile-disclosure__actions">
								<Button href="/seller-center/me" variant="bordered" size="sm">
									編輯商店資訊
								</Button>
							</div>
						</>
					)}
				/>
			</div>
		</>
	);
}
