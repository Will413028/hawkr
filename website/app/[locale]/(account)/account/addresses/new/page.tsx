import AddressFormClient from "@/features/account/screens/addresses/AddressFormClient";

export const metadata = { title: "新增地址" };

export default function Page() {
	return <AddressFormClient mode="create" />;
}
