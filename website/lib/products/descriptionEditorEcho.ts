/**
 * Tells apart the controlled `value` that is the echo of the editor's own
 * `onChange` from a value that was changed from outside (undo/redo, AI patch).
 *
 * Every signature the editor reports is remembered until it comes back, so
 * several reports in flight at once are each recognised as an echo no matter
 * how the parent batches or orders its renders.
 */
export class DescriptionEditorEcho {
	private signature: string;
	private pending: string[] = [];

	constructor(initialSignature: string) {
		this.signature = initialSignature;
	}

	/** True when the editor already holds this content. */
	isCurrent(signature: string): boolean {
		return signature === this.signature;
	}

	/** Record content applied to the editor without reporting it to the parent. */
	adopt(signature: string): void {
		this.signature = signature;
	}

	/** Record content the editor reports to the parent through `onChange`. */
	emit(signature: string): void {
		this.signature = signature;
		this.pending.push(signature);
	}

	/**
	 * Classify an incoming controlled value. `external` means the caller must
	 * render it; the editor's signature then becomes this value.
	 */
	receive(signature: string): "echo" | "external" {
		const index = this.pending.indexOf(signature);
		if (index >= 0) {
			this.pending.splice(0, index + 1);
			return "echo";
		}
		if (signature === this.signature) return "echo";
		this.pending = [];
		this.signature = signature;
		return "external";
	}
}
