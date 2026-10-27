import AppIcon from "@/components/shared/AppIcon";
import { PAYMENT_METHOD_LABEL } from "@/lib/codemeta/enums";

/**
 * Payment-method glyph. Shared by the
 * checkout payment selector and the order「再次付款」sheet so both surfaces
 * render identical marks from one source of truth.
 */
export default function PaymentGlyph({
	value,
	/**
	 * Accessible name. Defaults to the CodeMeta payment-method label, because on
	 * an order-detail row the mark IS the answer to「方式」and had no text at all.
	 * Call sites that already print the label beside it wrap the glyph in an
	 * `aria-hidden` span, which hides this name with it — so the default is safe
	 * there and there is nothing to opt out of. Pass `""` to force decorative.
	 */
	label = PAYMENT_METHOD_LABEL[value] ?? "",
}: {
	value: string;
	label?: string;
}) {
	// Every method shares one wallet mark; the label carries the name.
	return (
		<AppIcon
			name="wallet"
			size={20}
			className="shrink-0"
			title={label || undefined}
		/>
	);
}
