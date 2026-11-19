/**
 * 站內的 warning tone 提示條（`<p role="status">`）：預告終止橫幅與帳號停權橫幅共用。
 * 樣式只用既有 warning token 與 dark-mode 覆寫（同 `AfterSaleStatusBadge`），不帶自己的 props 語彙。
 */
export default function WarningNotice({
	children,
}: {
	children: React.ReactNode;
}) {
	return (
		<p
			role="status"
			className="m-0 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-hkui-md bg-warning-50 px-3.5 py-2.5 text-[0.8125rem] font-semibold text-warning-700 dark:bg-[#a9842c]/26 dark:text-[#d6c27e]"
		>
			{children}
		</p>
	);
}
