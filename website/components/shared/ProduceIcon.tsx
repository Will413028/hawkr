import AppIcon, {
	type AppIconName,
	type AppIconProps,
} from "@/components/shared/AppIcon";

export const PRODUCE_ICON_NAMES = [
	"produce",
	"leaf",
	"herb",
	"grass",
] as const satisfies readonly AppIconName[];

export type ProduceIconName = (typeof PRODUCE_ICON_NAMES)[number];

export type ProduceIconProps = Omit<AppIconProps, "name"> & {
	name: ProduceIconName;
};

/**
 * Generic farm-produce glyphs. Material Symbols has no per-fruit shapes, so
 * the set is a handful of produce, leaf and grass marks.
 */
export default function ProduceIcon({ name, ...rest }: ProduceIconProps) {
	return <AppIcon {...rest} name={name} data-produce-icon={name} />;
}
