"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { formatDateLabel, formatWeekday } from "@/lib/format";

interface TopbarProps {
	onMenuClick: () => void;
}

export default function Topbar({ onMenuClick }: TopbarProps) {
	// 🔴 頁首日期也釘台北（backend#3053 的稽查）：這是**平台營運日**，不是看的人的
	// 本機日期。管理員在日本時，23:00–24:00 (JST) 這一小時的「今天」與台北差一天
	// —— 而底下所有資料的時間都是台北。
	const today = new Date();
	const dateStr = formatDateLabel(today);
	const weekday = formatWeekday(today);

	return (
		<header className="topbar">
			<div className="topbar-context">
				<button
					type="button"
					className="topbar-menu-button"
					onClick={onMenuClick}
					aria-label="開啟主選單"
				>
					<MaterialSymbol name="menu" />
				</button>
				<span className="topbar-context-icon" aria-hidden="true">
					<MaterialSymbol name="shield_person" />
				</span>
				<div>
					<strong>平台營運管理</strong>
					<span>Hawkr Admin</span>
				</div>
			</div>
			<div className="topbar-right">
				<div className="topbar-date">
					<MaterialSymbol name="calendar_today" />
					<span>
						<span className="date">{dateStr}</span>
						<span className="weekday">{weekday}</span>
					</span>
				</div>
			</div>
		</header>
	);
}
