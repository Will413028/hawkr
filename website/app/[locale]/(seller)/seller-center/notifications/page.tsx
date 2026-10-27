import NotificationsClient from "@/features/account/screens/notifications/NotificationsClient";

export const metadata = { title: "通知中心 — 賣家中心" };

export default function Page() {
	return (
		<NotificationsClient
			title="通知中心"
			backHref="/seller-center/dashboard"
			backLabel="回儀表板"
			preferencesHref="/seller-center/notifications/preferences"
		/>
	);
}
