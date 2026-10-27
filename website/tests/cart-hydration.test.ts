import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const cartClientPath = new URL(
	"../features/cart/CartClient.tsx",
	import.meta.url,
);

test("cart keeps browser-only state out of the first client render", async () => {
	const cartClient = await readFile(cartClientPath, "utf8");

	assert.match(
		cartClient,
		/const \[mounted, setMounted\] = React\.useState\(false\)/,
	);
	assert.match(
		cartClient,
		/const \[saved, setSaved\] = React\.useState<CartItem\[]>\(\[\]\)/,
	);
	assert.doesNotMatch(
		cartClient,
		/React\.useState<CartItem\[]>\(\(\) => getSavedItems\(\)\)/,
	);
	assert.match(cartClient, /await useCartUiStore\.persist\.rehydrate\(\)/);
	assert.match(cartClient, /setSaved\(getSavedItems\(\)\)/);
	assert.match(cartClient, /if \(!mounted \|\| !authHydrated\)/);
});
