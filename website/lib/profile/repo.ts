/** BuyerAppService profile methods mapped to the account view. */

import { createContextValues } from "@connectrpc/connect";
import {
	Gender,
	GenderSchema,
	ProfileSchema,
} from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { formatDate, parseDate } from "@hawkr/api/proto/date";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import { buyerAppClient } from "../connect/buyer";
import { SUPPRESS_AUTH_EXPIRED } from "../connect/transport";
import { imageUri } from "../media/image";

/**
 * The subset of `handler.BuyerProfileResponse` this app actually renders.
 *
 * The endpoint used to return the raw proto (snake_case tags, `created_at` as a
 * Unix-second STRING); it now returns a Gateway DTO whose keys are camelCase and
 * whose timestamps are ISO 8601 UTC. Only fields with a consumer live here — a
 * field nobody reads is a field nobody notices going wrong.
 */
export interface MemberProfile {
	etag?: string;
	/** Current `buyer_id` (gateway `BuyerProfileResponse.ID`), kept as the exact
	 * decimal string the wire carries. Since backend commit 2269417e3
	 * (2026-07-18) this is a freshly minted 64-bit TSID — 18-19 digits, past
	 * `Number.MAX_SAFE_INTEGER` — so `Number()` would silently corrupt its low
	 * digits. Chat compares it against `senderId` with `===`
	 * (`lib/chat/chatIdentity.ts`), where that corruption means "never matches". */
	id?: string;
	nickname?: string;
	/** Bare objectKey (49 §6.1 SSOT). `<Avatar>`/`buildImageUrl` resolve it — the
	 * paired `avatarUrl` the wire also carries is derived from this one. */
	avatarKey?: string;
	/** `undefined` 是買家沒有填。 */
	gender?: Gender;
	birthday?: string;
	phone?: string;
	email?: string;
}

/** `suppressAuthExpired`：「沒有登入」由呼叫端自己處理，不廣播 AUTH_EXPIRED_EVENT（同 getSession）。 */
export type FetchProfileOptions = { suppressAuthExpired?: boolean };

export async function fetchProfile(
	options: FetchProfileOptions = {},
): Promise<MemberProfile> {
	const { profile } = await buyerAppClient().getProfile(
		{},
		options.suppressAuthExpired
			? {
					contextValues: createContextValues().set(SUPPRESS_AUTH_EXPIRED, true),
				}
			: undefined,
	);
	const birthday = profile?.birthDate;
	return {
		etag: profile?.etag,
		id: profile?.id || undefined,
		nickname: profile?.displayName,
		avatarKey: imageUri(profile?.avatar, "avatar"),
		gender: knownGender(profile?.gender),
		birthday: birthday ? formatDate(birthday) : undefined,
		phone: profile?.phone,
	};
}

/** optional enum：不認得的值與沒有設值無法分辨（API 規範），UNSPECIFIED 同樣當作沒有填。 */
function knownGender(value: Gender | undefined): Gender | undefined {
	if (value === undefined || value === Gender.UNSPECIFIED) return undefined;
	return GenderSchema.values.some((v) => v.number === value)
		? value
		: undefined;
}

/** 格式錯誤直接丟錯，不送出（不能讓它變成清除生日）。 */
function parseBirthDate(value: string) {
	const date = parseDate(value);
	if (!date) throw new Error("生日格式不正確");
	return date;
}

/**
 * 個人資料表單送出的內容：三個欄位都是表單擁有的，一律列進 update_mask。
 * `gender` 是 `undefined`、`birthDate` 是空字串都表示沒有值，送出時清除。
 */
export type ProfileEdit = {
	nickname: string;
	gender: Gender | undefined;
	birthDate: string;
};

/**
 * UpdateProfile 的文字欄位。頭像不在這裡改，走 {@link updateBuyerAvatar}（同一個 RPC、另一組 mask）。
 */
export async function updateProfile(
	body: ProfileEdit,
	loaded: MemberProfile,
): Promise<void> {
	const client = buyerAppClient();
	if (!loaded.id || !loaded.etag) throw new Error("買家資料不存在");
	const birthDate = body.birthDate ? parseBirthDate(body.birthDate) : undefined;
	await client.updateProfile({
		profile: {
			id: loaded.id,
			etag: loaded.etag,
			displayName: body.nickname,
			gender: body.gender,
			birthDate,
		},
		updateMask: fieldMask(ProfileSchema, [
			"display_name",
			"gender",
			"birth_date",
		]),
	});
}

/**
 * 換頭像：UpdateProfile 帶 `avatar_staged_token`（INPUT_ONLY）並把它列進 update mask，etag 用載入時的版本。
 * `stagedToken` 來自 `uploadImageFile`；後端 finalize 暫存圖並回傳永久的 objectKey。空 token 不送出
 * （進了 mask 的空 token 會被後端當成錯誤）。
 */
export async function updateBuyerAvatar(
	stagedToken: string,
	loaded: MemberProfile,
): Promise<string> {
	const client = buyerAppClient();
	if (!loaded.id || !loaded.etag) throw new Error("買家資料不存在");
	if (!stagedToken) throw new Error("頭像上傳尚未完成");
	const result = await client.updateProfile({
		profile: {
			id: loaded.id,
			etag: loaded.etag,
			avatarStagedToken: stagedToken,
		},
		updateMask: fieldMask(ProfileSchema, ["avatar_staged_token"]),
	});
	return imageUri(result.profile?.avatar, "avatar") ?? "";
}
