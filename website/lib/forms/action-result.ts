export interface FieldErrors {
	[field: string]: string[] | undefined;
}

export interface FormActionResult<T = undefined> {
	ok: boolean;
	data?: T;
	message?: string;
	fieldErrors?: FieldErrors;
}

export function formActionSuccess<T>(
	data?: T,
	message?: string,
): FormActionResult<T> {
	return { ok: true, data, message };
}

export function formActionFailure(
	message: string,
	fieldErrors?: FieldErrors,
): FormActionResult {
	return { ok: false, message, fieldErrors };
}
