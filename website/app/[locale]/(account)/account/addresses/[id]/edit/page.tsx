import AddressFormClient from "@/features/account/screens/addresses/AddressFormClient";

export const metadata = { title: "編輯地址" };

export default async function Page({
	params,
}: {
	params: Promise<{ id: string }>;
}) {
	const { id } = await params;
	const numericId = Number(id);
	if (!Number.isFinite(numericId)) {
		return <AddressFormClient mode="create" />;
	}
	return <AddressFormClient mode="edit" initialId={id} />;
}
