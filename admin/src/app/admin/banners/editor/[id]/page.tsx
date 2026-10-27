"use client";

import dynamic from "next/dynamic";
import { useParams } from "next/navigation";

const BannerEditorClient = dynamic(
	() => import("@/components/banner/BannerEditorClient"),
	{
		ssr: false,
		loading: () => (
			<div style={{ padding: "40px", textAlign: "center" }}>
				載入編輯器中...
			</div>
		),
	},
);

export default function BannerEditorEditPage() {
	const params = useParams();
	return <BannerEditorClient id={params.id as string} />;
}
