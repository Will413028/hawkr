/**
 * Shared chrome for the two site footers (buyer `Footer`, seller
 * `SellerFooter`). Both used to carry byte-identical copies of the link /
 * policy / heading classes and of the whole footer-bottom block, so every
 * change had to be made twice — and the 32px mobile social icon, the 6px link
 * gap and the 11px copyright all shipped in duplicate.
 *
 * Target size: links keep 44px rows for touch and phones; desktop pointer
 * users get compact 32px rows. Circular actions stay 44px at every width.
 *
 * Colour: `hk-cream` utilities resolve to `--color-neutral-cream`; the inline
 * `style={{ background: … }}` attributes these replaced held the same token.
 */

export const FOOTER_ROOT =
	"hk-footer w-full flex flex-col gap-4 pt-6 pb-0 bg-[var(--brand-text-black)] max-[768px]:pb-4 max-[560px]:gap-3 max-[560px]:pt-[18px] max-[560px]:pb-3";

/** Centred max-width shell shared by the column grid and the footer bottom. */
export const FOOTER_SHELL =
	"w-full max-w-[1440px] box-border mx-auto px-[60px] max-[960px]:px-6 max-[560px]:px-[18px]";

export const FOOTER_HEADING =
	"hk-footer__heading flex items-center gap-2 m-0 mb-2 text-hk-cream font-bold tracking-[-0.44px] break-keep [overflow-wrap:normal] max-[560px]:gap-1.5 max-[560px]:tracking-[-0.2px]";

export const FOOTER_HEADING_DOT =
	"inline-block w-1.5 h-6 rounded-[var(--radius-pill)] bg-hk-cream max-[560px]:w-[3px] max-[560px]:h-3.5";

export const FOOTER_LINK_LIST =
	"flex flex-col gap-0 pl-3.5 pr-0 m-0 list-none max-[560px]:p-0";

/**
 * Link column wrapper. Every column is a native `<details open>`: ≤560px it is
 * a real accordion (five stacked columns do not fit a phone otherwise), and
 * above that the disclosure is neutralised so the column keeps rendering as a
 * plain block — `open` in the markup covers engines without
 * `::details-content`, and the override covers a column the visitor collapsed
 * on a phone before widening the window.
 *
 * `group` is what `FOOTER_SUMMARY_CHEVRON` reads for its open-state rotation.
 */
export const FOOTER_COLUMN =
	"group min-w-0 max-[560px]:border-t max-[560px]:border-hk-cream/10 max-[560px]:pt-0 max-[560px]:pb-1 min-[561px]:[&::details-content]:[content-visibility:visible]";

/**
 * Disclosure row. The heading stays an `<h2>` inside the `<summary>` (allowed:
 * summary takes heading content) so the footer keeps its heading outline in
 * both layouts. 44px tall on phones; untouched above 560px, where the marker
 * is removed and the row is not presented as clickable.
 */
export const FOOTER_SUMMARY =
	"list-none [&::-webkit-details-marker]:hidden min-[561px]:cursor-default max-[560px]:flex max-[560px]:min-h-11 max-[560px]:cursor-pointer max-[560px]:select-none max-[560px]:items-center max-[560px]:justify-between max-[560px]:gap-2 max-[560px]:[&>h2]:mb-0";

/** Phone-only disclosure chevron; flips when its `FOOTER_COLUMN` is open. */
export const FOOTER_SUMMARY_CHEVRON =
	"hidden shrink-0 text-hk-cream/60 transition-transform duration-200 motion-reduce:transition-none group-open:rotate-180 max-[560px]:block";

export const FOOTER_TEXT_LINK =
	"inline-flex min-h-11 items-center min-[561px]:pointer-fine:min-h-8 max-w-full !text-hk-cream/70 text-[length:var(--text-sm)] leading-5 no-underline tracking-[-0.15px] transition-colors duration-200 hover:!text-hk-cream break-keep [overflow-wrap:anywhere] max-[560px]:text-xs max-[560px]:leading-[1.6] max-[560px]:!text-hk-cream/60";

export const FOOTER_POLICY_LINK =
	"inline-flex min-h-11 items-center min-[561px]:pointer-fine:min-h-8 !text-hk-cream/85 text-[length:var(--text-base)] leading-6 no-underline hover:!text-hk-cream hover:underline max-[560px]:text-xs max-[560px]:!text-hk-cream/70 max-[560px]:whitespace-nowrap";

export const FOOTER_CONTACT_LINK =
	"flex min-h-11 min-w-0 items-center min-[561px]:pointer-fine:min-h-9 gap-2 pl-3.5 py-0 !text-hk-cream/70 text-[length:var(--text-sm)] leading-[1.625] tracking-[-0.15px] no-underline transition-colors duration-200 hover:!text-hk-cream [overflow-wrap:anywhere] max-[560px]:p-0 max-[560px]:gap-0 max-[560px]:text-xs max-[560px]:leading-[1.55] max-[560px]:!text-hk-cream/60";

export const FOOTER_CONTACT_ICON =
	"flex items-center justify-center w-8 h-8 min-w-[32px] rounded-full bg-hk-cream/10 max-[560px]:hidden";

/** Shared footer action chrome: equal 44px circles for links and controls. */
export const FOOTER_ICON_LINK =
	"inline-flex shrink-0 items-center justify-center size-11 p-0 border-0 rounded-full bg-hk-cream/10 !text-hk-cream/80 cursor-pointer touch-manipulation transition-colors duration-200 motion-reduce:transition-none enabled:hover:bg-hk-cream/20 hover:!text-hk-cream [&:any-link:hover]:bg-hk-cream/20 focus-visible:outline-2 focus-visible:outline-hk-cream focus-visible:outline-offset-4 disabled:cursor-default disabled:opacity-50";

export const FOOTER_BRAND_LOGO =
	"[--brand-logo-render-height:52px] max-[560px]:[--brand-logo-render-height:36px]";

export const FOOTER_BRAND_TAGLINE =
	"m-0 mt-0 mb-3 text-hk-cream/60 text-[length:var(--text-sm)] leading-[1.625] tracking-[-0.15px] max-[560px]:mt-2 max-[560px]:mb-2 max-[560px]:text-xs max-[560px]:leading-[1.55] max-[560px]:text-hk-cream/50";
