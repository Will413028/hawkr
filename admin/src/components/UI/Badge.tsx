import {
	MaterialSymbol,
	type MaterialSymbolName,
} from "@hawkr/icons/material-symbol";
import type React from "react";

interface BadgeProps {
	status: number;
	type: "merchant" | "payment" | "banner" | "certification";
}

const Badge: React.FC<BadgeProps> = ({ status, type }) => {
	let text = "";
	let variant = "";
	let icon: MaterialSymbolName | "" = "";

	if (type === "merchant") {
		switch (status) {
			case 0:
				text = "待審核";
				variant = "pending";
				icon = "schedule";
				break;
			case 1:
				text = "已通過";
				variant = "approved";
				icon = "check_circle";
				break;
			case 2:
				text = "已退回";
				variant = "rejected";
				icon = "cancel";
				break;
			case 3:
				text = "已停用";
				variant = "rejected";
				icon = "block";
				break;
			default:
				text = "未知";
				variant = "gray";
				break;
		}
	} else if (type === "payment") {
		switch (status) {
			case 0:
				text = "未認證";
				variant = "pending";
				break;
			case 1:
				text = "審核中";
				variant = "pending";
				break;
			case 9:
				text = "已認證";
				variant = "approved";
				break;
			case 2:
				text = "審核失敗";
				variant = "rejected";
				break;
			default:
				text = "未知";
				variant = "gray";
				break;
		}
	} else if (type === "banner") {
		switch (status) {
			case 1:
				text = "已啟用";
				variant = "approved";
				break;
			case 0:
				text = "已停用";
				variant = "gray";
				break; // Using gray or pending
			default:
				text = "未知";
				variant = "gray";
				break;
		}
	} else if (type === "certification") {
		switch (status) {
			case 0:
				text = "待審核";
				variant = "pending";
				icon = "schedule";
				break;
			case 1:
				text = "已通過";
				variant = "approved";
				icon = "check_circle";
				break;
			case 2:
				text = "已退回";
				variant = "rejected";
				icon = "cancel";
				break;
			default:
				text = "未知";
				variant = "gray";
				break;
		}
	}

	return (
		<span className={`status-badge ${variant}`}>
			{icon && <MaterialSymbol name={icon} />} {text}
		</span>
	);
};

export default Badge;
