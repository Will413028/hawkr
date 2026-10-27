/**
 * GIF helpers shared by the upload and display paths.
 *
 * Persisted post media is selected before the Image Gateway reads object
 * metadata, so a stable `.gif` object-key suffix is part of the animation
 * contract. Browser pickers occasionally omit MIME for extension-only files;
 * normalize that narrow case without treating an arbitrary non-image file as a
 * GIF merely because its name ends in `.gif`.
 */

export function isGifMedia(value: string | null | undefined): boolean {
	if (!value) return false;
	const pathname = value.trim().split(/[?#]/, 1)[0];
	return /\.gif$/i.test(pathname);
}

export function isGifUploadFile(file: Pick<File, "name" | "type">): boolean {
	const type = file.type.trim().toLowerCase();
	return type === "image/gif" || (type === "" && isGifMedia(file.name));
}

export function normalizeGifUploadFile(file: File): File {
	if (!isGifUploadFile(file)) return file;

	const normalizedName = isGifMedia(file.name)
		? file.name
		: `${file.name || "post-image"}.gif`;
	if (file.type === "image/gif" && normalizedName === file.name) return file;

	return new File([file], normalizedName, {
		type: "image/gif",
		lastModified: file.lastModified,
	});
}
