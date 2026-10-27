import { addons } from "storybook/manager-api";
import { create } from "storybook/theming";
import contract from "../design-system/cross-platform-tokens.json";

const primary = String(
	contract.tokens.find((token) => token.id === "brand.primaryStrong")?.value,
);
addons.setConfig({
	theme: create({
		base: "light",
		brandTitle: "Hawkr · 元件庫",
		brandImage: "./brand/logo-horizontal.svg",
		brandUrl: "./",
		colorPrimary: primary,
		colorSecondary: primary,
	}),
	showPanel: false,
});
