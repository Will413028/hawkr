"use client";

import {
	MaterialSymbol,
	type MaterialSymbolName,
} from "@hawkr/icons/material-symbol";
import type * as React from "react";
import { cn } from "@/lib/utils/cn";

const MATERIAL_SYMBOL_BY_NAME = {
	activity: "show_chart",
	"align-center": "format_align_center",
	"align-left": "format_align_left",
	"align-right": "format_align_right",
	bold: "format_bold",
	italic: "format_italic",
	underline: "format_underlined",
	"list-bullet": "format_list_bulleted",
	"list-number": "format_list_numbered",
	table: "table",
	undo: "undo",
	redo: "redo",
	"add-user": "person_add",
	"arrow-down": "arrow_downward",
	"arrow-left": "arrow_back",
	"arrow-right": "arrow_forward",
	"arrow-up": "arrow_upward",
	bag: "shopping_bag",
	bolt: "bolt",
	box: "inventory_2",
	bookmark: "bookmark",
	buy: "shopping_basket",
	calendar: "calendar_month",
	call: "call",
	camera: "photo_camera",
	cart: "shopping_cart",
	category: "category",
	chart: "bar_chart",
	chat: "chat_bubble",
	check: "check",
	clock: "schedule",
	"chevron-down": "keyboard_arrow_down",
	"chevron-left": "keyboard_arrow_left",
	"chevron-right": "keyboard_arrow_right",
	"chevron-up": "keyboard_arrow_up",
	clipboard: "assignment",
	close: "close",
	coin: "paid",
	"cloud-upload": "cloud_upload",
	cubes: "widgets",
	danger: "warning",
	delete: "delete",
	discount: "percent",
	discovery: "explore",
	document: "description",
	download: "download",
	edit: "edit",
	egg: "egg_alt",
	fail: "cancel",
	flower: "local_florist",
	filter: "filter_list",
	flag: "flag",
	folder: "folder",
	goods: "package_2",
	grain: "grain",
	grass: "grass",
	graph: "trending_up",
	heart: "favorite",
	herb: "spa",
	hide: "visibility_off",
	home: "home",
	image: "image",
	info: "info",
	leaf: "eco",
	location: "location_on",
	lock: "lock",
	login: "login",
	logout: "logout",
	meat: "kebab_dining",
	message: "mail",
	more: "more_horiz",
	notification: "notifications",
	paper: "article",
	"paper-download": "download",
	"paper-plus": "note_add",
	"paper-upload": "upload",
	party: "celebration",
	people: "group",
	pin: "pin_drop",
	play: "play_arrow",
	produce: "nutrition",
	plus: "add",
	preview: "preview",
	processed: "takeout_dining",
	reorder: "swap_vert",
	refresh: "refresh",
	scan: "qr_code_scanner",
	search: "search",
	seafood: "set_meal",
	send: "send",
	settings: "settings",
	share: "share",
	shield: "verified_user",
	"shield-fail": "gpp_bad",
	show: "visibility",
	sparkle: "stars",
	success: "check_circle",
	spinner: "progress_activity",
	star: "star",
	stock: "inventory",
	tag: "sell",
	ticket: "confirmation_number",
	trophy: "trophy",
	truck: "local_shipping",
	upload: "upload",
	user: "person",
	users: "group",
	verified: "verified",
	wallet: "account_balance_wallet",
	warning: "warning",
	work: "work",
} as const satisfies Record<string, MaterialSymbolName>;

export type AppIconName = keyof typeof MATERIAL_SYMBOL_BY_NAME;

export interface AppIconProps
	extends Omit<React.SVGProps<SVGSVGElement>, "name" | "fill"> {
	name: AppIconName;
	size?: number;
	/** Accessible label. When omitted the icon is decorative. */
	title?: string;
	/** Filled variant, for selected or active states. */
	fill?: boolean;
	spin?: boolean;
}

/**
 * The only static icon entrypoint for product UI. Feature code chooses a
 * semantic name here and never owns SVG geometry; the glyphs are Material
 * Symbols drawn by `@hawkr/icons`.
 */
export default function AppIcon({
	name,
	size = 24,
	title,
	fill = false,
	spin = false,
	className,
	...rest
}: AppIconProps) {
	return (
		<MaterialSymbol
			{...rest}
			name={MATERIAL_SYMBOL_BY_NAME[name]}
			size={size}
			title={title}
			fill={fill}
			className={cn(
				"hk-app-icon",
				spin && "animate-spin motion-reduce:animate-none",
				className,
			)}
			data-icon-name={name}
		/>
	);
}
