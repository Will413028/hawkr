// No modal on routes that aren't an intercepted product view (and on hard
// loads of /products/[id], where the intercept doesn't run). Required so the
// @modal parallel slot resolves for every public route.
export default function ModalDefault() {
	return null;
}
