import type { DescMessage } from "@bufbuild/protobuf";
import type { FieldMask } from "@bufbuild/protobuf/wkt";

/**
 * 建立 `resource`（被更新的資源訊息，例如 `ProfileSchema`）的 `FieldMask` 初始化形狀（請求的 `updateMask` 欄位直接收；
 * 回傳純物件而不是 `create(FieldMaskSchema)`，與其他請求欄位的組法一致）。
 *
 * `paths` 是 proto 的 snake_case 欄位名。依 API 設計規範（Connect）「更新請求的 update_mask」：路徑只支援最上層的欄位
 * （不支援 `action.url`），唯一的例外是下到 `oneof` 的子訊息（`individual.id_card_no`）；其他子訊息整個換掉，
 * 路徑停在那個欄位，不能再往下。不認得的路徑，或在非 oneof 成員的欄位後面再接子路徑，丟出 Error。
 */
export function fieldMask(
	resource: DescMessage,
	paths: Iterable<string>,
): Pick<FieldMask, "paths"> {
	const checked: string[] = [];
	for (const path of paths) {
		check(resource, path);
		checked.push(path);
	}
	return { paths: checked };
}

function check(root: DescMessage, path: string): void {
	let message = root;
	const segments = path.split(".");
	for (const [i, segment] of segments.entries()) {
		const field = message.fields.find((f) => f.name === segment);
		if (!field) {
			throw new Error(
				`update_mask 路徑 ${path}：欄位 ${segment} 不存在於 ${root.typeName}`,
			);
		}
		if (i === segments.length - 1) return;
		if (field.fieldKind !== "message") {
			throw new Error(`update_mask 路徑 ${path}：${segment} 不是訊息欄位`);
		}
		if (!field.oneof) {
			throw new Error(
				`update_mask 路徑 ${path}：${segment} 不是 oneof 的子訊息，路徑只能停在它`,
			);
		}
		message = field.message;
	}
}
