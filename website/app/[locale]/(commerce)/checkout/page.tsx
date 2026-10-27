import CheckoutClient from "@/features/checkout/screens/CheckoutClient";

export const metadata = { title: "結帳" };

export default function Page() {
	return (
		<main className="relative isolate overflow-hidden bg-transparent pt-[var(--page-start-gap,6px)] text-hk-body max-md:px-0 max-md:py-[52px] max-md:pt-[var(--page-start-gap,var(--hkui-space-4))]">
			<div className="w-[min(1440px,calc(100%-48px))] mx-auto max-md:w-[min(100%-28px,680px)]">
				<CheckoutClient />
			</div>
		</main>
	);
}
