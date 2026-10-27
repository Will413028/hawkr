/**
 * Post-comment ownership checks, extracted out of `PostComments.tsx` so the
 * permission logic is unit-testable without rendering the component.
 *
 * Who wrote a comment is decided by the server: every comment and reply carries
 * `isMine` (the author is the viewer's buyer or store identity). The client
 * never compares ids, so a buyer and a store sharing the same number cannot be
 * confused.
 */

/** Content edits are author-only regardless of moderation rights — a post
 *  owner can remove someone else's comment but not rewrite it. */
export function canEditComment(isMine: boolean): boolean {
	return isMine;
}

/** Post owners (canModerate) may delete any comment on their own post;
 *  otherwise only the author may delete their own. */
export function canDeleteComment(
	isMine: boolean,
	canModerate: boolean,
): boolean {
	return canModerate || canEditComment(isMine);
}
