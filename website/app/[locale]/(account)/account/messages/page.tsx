import MessagesClient from "@/features/account/screens/messages/MessagesClient";

export const metadata = { title: "訊息" };

export default function Page() {
	return <MessagesClient backHref="/account" />;
}
