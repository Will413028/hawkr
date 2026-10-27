"use client";

import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { Card } from "@/components/ui/Card";
import { useHasMeasures } from "@/lib/hooks/useSellerMeasures";
import { MEASURES_PATH } from "./measureDisplay";

/**
 * 帳號頁的「措施與申覆」入口：帳號有措施時顯示（不看店家狀態）。終止合作後店家關閉、賣家角色移除，賣家中心側欄進不來，
 * 這是關店後仍能申覆的入口；`/seller/measures` 以帳號為範圍。讀取中或失敗不顯示。
 */
export default function MeasuresAccountEntry() {
	const hasMeasures = useHasMeasures();
	if (!hasMeasures) return null;
	return (
		<Card
			as={Link}
			href={MEASURES_PATH}
			isHoverable
			className="flex items-center gap-3 p-4 no-underline"
		>
			<AppIcon name="shield-fail" size={24} />
			<span className="flex-1 text-[0.9375rem] font-bold text-[var(--hkui-foreground)]">
				措施與申覆
			</span>
			<AppIcon name="arrow-right" size={16} />
		</Card>
	);
}
