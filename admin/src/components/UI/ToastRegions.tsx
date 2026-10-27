"use client";

/** 兩個空的 live region（polite 的 status、立即的 alert）。Dialog 放在自己的面板裡，Provider 在 body 也放一份。 */
export function ToastRegions() {
	return (
		<div className="adm-toast-host">
			<div role="status" data-toast-region="status" />
			<div role="alert" data-toast-region="alert" />
		</div>
	);
}
