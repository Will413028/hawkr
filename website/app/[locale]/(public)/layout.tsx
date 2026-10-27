/**
 * Public layout — hosts the `@modal` parallel slot used by the product
 * quick-view. Opening a product is an *intercepting route* (soft navigations to
 * `/products/[id]` render in `@modal` over the page you were on); the underlying
 * page (e.g. the market list) stays mounted, so its scroll/state is preserved
 * and closing lands you exactly where you were. A hard visit / refresh of
 * `/products/[id]` skips the intercept and renders the full page in `children`.
 */
export default function PublicLayout({
	children,
	modal,
}: {
	children: React.ReactNode;
	modal: React.ReactNode;
}) {
	return (
		<>
			{children}
			{modal}
		</>
	);
}
