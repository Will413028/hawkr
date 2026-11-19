import type * as React from "react";

/** 預覽邊界放行區：只包住翻頁等「讀取」控制，其內的點擊不被預覽邊界取消。 */
export default function PreviewReadingControl({
	children,
}: {
	children: React.ReactNode;
}) {
	return <div data-storefront-preview-reading-control="true">{children}</div>;
}
