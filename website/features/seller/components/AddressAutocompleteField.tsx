"use client";

import "@/styles/components/address-autocomplete.css";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { Button, Dialog, Input } from "@/components/ui";
import { toLogisticsName } from "@/lib/seller/certification-options";
import {
	type AddressSuggestion,
	getAddressSuggestions,
} from "@/lib/seller/shipping";

/**
 * Address autocomplete field — port of the seller App's
 * `AddressAutocompleteField` (lib/.../widgets/address_autocomplete_field.dart).
 *
 * Behaviour summary:
 *   1. Suggestions only appear when (county, district) are both set
 *      and the seller clicks 驗證. This avoids the old typeahead menu
 *      covering the form while the user is still typing.
 *   2. When TGOS returns a likely canonical address, a confirmation
 *      dialog asks the seller to apply it or keep their original input.
 *   3. After applying a suggestion, `verified` flips to true; the
 *      caller normally also stores the (lat, lng) for downstream PayUni
 *      submission.
 *   4. Any manual text edit re-arms the field to "unverified" so the
 *      seller can't sneak through a free-typed address that bypassed
 *      TGOS validation.
 *   5. The right-side 「驗證」 button calls strict
 *      `validateAddress(...)` — used when the seller types the address
 *      from memory and expects it to validate without seeing
 *      suggestions (e.g. their building name auto-completes a level
 *      that the suggestions list doesn't surface).
 */

/** 地址真實存在卻查不到時，賣家自己改不出結果 —— 直接導向客服。 */
const SUPPORT_EMAIL = "supports@example.com";

interface AddressAutocompleteFieldProps {
	value: string;
	onChange: (next: string) => void;
	county: string | undefined;
	district: string | undefined;
	disabled?: boolean;
	verified: boolean;
	verifying: boolean;
	onSuggestionSelected: (s: AddressSuggestion) => void;
	onManualEdit: () => void;
	onVerifyClick: () => void;
	verifyFeedback?: "error" | null;
	required?: boolean;
}

export function AddressAutocompleteField({
	value,
	onChange,
	county,
	district,
	disabled,
	verified,
	verifying,
	onSuggestionSelected,
	onManualEdit,
	onVerifyClick,
	verifyFeedback,
	required,
}: AddressAutocompleteFieldProps) {
	const containerRef = React.useRef<HTMLDivElement>(null);
	const [suggestions, setSuggestions] = React.useState<AddressSuggestion[]>([]);
	const [confirmOpen, setConfirmOpen] = React.useState(false);
	const [confirmTone, setConfirmTone] = React.useState<
		"suggestion" | "retry" | "empty"
	>("suggestion");
	const [formatCandidate, setFormatCandidate] = React.useState<string | null>(
		null,
	);
	const [checkingSuggestions, setCheckingSuggestions] = React.useState(false);

	const lastFetchTokenRef = React.useRef(0);
	const failedSuggestionTokenRef = React.useRef(0);
	const mountedRef = React.useRef(false);
	const latestAddressKey = `${county ?? ""}\0${district ?? ""}\0${value.trim()}`;
	const latestAddressKeyRef = React.useRef(latestAddressKey);

	React.useEffect(() => {
		mountedRef.current = true;
		return () => {
			mountedRef.current = false;
		};
	}, []);

	React.useEffect(() => {
		latestAddressKeyRef.current = latestAddressKey;
	}, [latestAddressKey]);

	// Reset suggestions whenever the county/district pair changes — old
	// suggestions are guaranteed stale because the backend filters by them.
	// biome-ignore lint/correctness/useExhaustiveDependencies: county/district are the trigger, not values read inside
	React.useEffect(() => {
		failedSuggestionTokenRef.current += 1;
		setSuggestions([]);
		setFormatCandidate(null);
		setConfirmOpen(false);
		setConfirmTone("suggestion");
	}, [county, district]);

	React.useEffect(() => {
		if (verifyFeedback !== "error") return;
		const trimmed = value.trim();
		if (!county || !district || trimmed.length < 2) return;
		const requestKey = latestAddressKey;
		const token = ++failedSuggestionTokenRef.current;

		void getSuggestedAddressCandidates({
			county,
			district,
			query: trimmed,
		}).then(({ items, candidate }) => {
			if (
				!mountedRef.current ||
				token !== failedSuggestionTokenRef.current ||
				requestKey !== latestAddressKeyRef.current
			) {
				return;
			}
			const next = dedupeAddressSuggestions(items);
			setSuggestions(next);
			setFormatCandidate(candidate);
			setConfirmTone(next.length > 0 ? "retry" : "empty");
			setConfirmOpen(true);
		});
	}, [verifyFeedback, county, district, value, latestAddressKey]);

	const handleChange = (next: string) => {
		onChange(next);

		// Any keystroke after a successful verify breaks the verified state.
		onManualEdit();
		failedSuggestionTokenRef.current += 1;
		if (confirmOpen) setConfirmOpen(false);
		setSuggestions([]);
		setFormatCandidate(null);
		setConfirmTone("suggestion");
	};

	const currentFullAddress = `${county ?? ""}${district ?? ""}${value.trim()}`;
	const primarySuggestion = suggestions[0];
	const secondarySuggestions = suggestions.slice(1, 3);

	const applySuggestion = (s: AddressSuggestion) => {
		// Use the suggestion's `detail` (路名門牌 portion only — county
		// and district stay in the dropdowns above) so the field doesn't
		// duplicate "桃園市桃園區桃園市桃園區..." after selection.
		onChange(s.detail || s.fullAddress);
		setConfirmOpen(false);
		onSuggestionSelected(s);
	};

	const applyFormatCandidate = () => {
		if (!formatCandidate) return;
		onChange(formatCandidate);
		onManualEdit();
		setConfirmOpen(false);
		setSuggestions([]);
		setFormatCandidate(null);
		setConfirmTone("suggestion");
	};

	const verifyOriginalInput = () => {
		setConfirmOpen(false);
		onVerifyClick();
	};

	const handleVerifyRequest = async () => {
		const trimmed = value.trim();
		if (!county || !district || trimmed.length < 2) {
			onVerifyClick();
			return;
		}

		const token = ++lastFetchTokenRef.current;
		setCheckingSuggestions(true);
		try {
			const { items, candidate } = await getSuggestedAddressCandidates({
				county,
				district,
				query: trimmed,
			});
			const next = dedupeAddressSuggestions(items);
			if (token !== lastFetchTokenRef.current) return;
			setFormatCandidate(candidate);
			if (next.length > 0) {
				setSuggestions(next);
				setConfirmTone("suggestion");
				setConfirmOpen(true);
				return;
			}
			onVerifyClick();
		} finally {
			if (token === lastFetchTokenRef.current) {
				setCheckingSuggestions(false);
			}
		}
	};

	const canVerify =
		!disabled &&
		!verified &&
		!verifying &&
		!checkingSuggestions &&
		Boolean(value.trim()) &&
		Boolean(county) &&
		Boolean(district);
	const hasVerifyError = verifyFeedback === "error";
	const hasDialogBody = confirmTone === "empty" || Boolean(primarySuggestion);
	const dialogTitle =
		confirmTone === "empty"
			? "暫時找不到建議地址"
			: confirmTone === "retry"
				? "地址未通過，建議使用"
				: "確認地址格式";
	const dialogDescription =
		confirmTone === "empty"
			? "這類村里或門牌格式不一定會有系統建議，請確認地址細節後再驗證。"
			: confirmTone === "retry"
				? "系統找到可能符合的地址格式，套用建議後可再送出驗證。"
				: "系統找到較完整的地址格式，建議套用後再送出驗證。";
	const showVerificationNotice = confirmTone !== "suggestion";

	return (
		<div ref={containerRef} className="seller-addr-autocomplete">
			<div className="seller-addr-autocomplete__row">
				<div className="seller-addr-autocomplete__input">
					<Input
						floatingLabel
						variant="bordered"
						size="lg"
						label="路名門牌"
						value={value}
						placeholder="例：松仁路 100 號"
						disabled={disabled}
						isRequired={required}
						onChange={(e) => handleChange(e.target.value)}
						endContent={
							verified ? (
								<span
									className="seller-addr-autocomplete__check"
									aria-hidden="true"
									title="地址已驗證"
								>
									<AppIcon
										name="check"
										size={28}
										style={{ color: "var(--hkui-success)" }}
									/>
								</span>
							) : null
						}
					/>
				</div>

				<Button
					type="button"
					className="seller-addr-autocomplete__verify"
					data-verified={verified || undefined}
					data-feedback={hasVerifyError ? "error" : undefined}
					disabled={!canVerify || hasVerifyError}
					isLoading={checkingSuggestions || verifying}
					loadingLabel={checkingSuggestions ? "比對地址中…" : "驗證地址中…"}
					onClick={() => void handleVerifyRequest()}
				>
					<span className="seller-addr-autocomplete__verify-label">
						{hasVerifyError ? (
							<AppIcon
								name="delete"
								className="seller-addr-autocomplete__verify-icon"
								size={18}
								style={{
									color: "var(--seller-addr-verify-feedback-color, #fffdf7)",
								}}
							/>
						) : null}
						{hasVerifyError ? "未通過" : verified ? "已驗證" : "驗證"}
					</span>
				</Button>
			</div>

			<Dialog
				open={confirmOpen && hasDialogBody}
				onClose={() => setConfirmOpen(false)}
				title={dialogTitle}
				description={dialogDescription}
				size="sm"
				footer={
					<>
						<Button
							variant="light"
							color="default"
							onClick={() => setConfirmOpen(false)}
						>
							{confirmTone === "empty" ? "我了解" : "返回修改"}
						</Button>
						{confirmTone === "suggestion" ? (
							<Button
								variant="bordered"
								color="default"
								onClick={verifyOriginalInput}
							>
								維持原輸入驗證
							</Button>
						) : null}
						{primarySuggestion ? (
							<Button
								color="primary"
								onClick={() => applySuggestion(primarySuggestion)}
							>
								套用建議地址
							</Button>
						) : null}
						{confirmTone === "empty" && formatCandidate ? (
							<Button color="primary" onClick={applyFormatCandidate}>
								套用建議格式
							</Button>
						) : null}
					</>
				}
			>
				{primarySuggestion ? (
					<div className="seller-addr-confirm">
						<div className="seller-addr-confirm__block">
							<span className="seller-addr-confirm__label">你輸入的地址</span>
							<p className="seller-addr-confirm__text">{currentFullAddress}</p>
						</div>
						<div className="seller-addr-confirm__block seller-addr-confirm__block--suggested">
							<span className="seller-addr-confirm__label">建議使用</span>
							<p className="seller-addr-confirm__suggestion">
								{formatSuggestionAddress(primarySuggestion)}
							</p>
						</div>
						{secondarySuggestions.length > 0 ? (
							<div className="seller-addr-confirm__alternates">
								<span className="seller-addr-confirm__label">其他可能格式</span>
								{secondarySuggestions.map((s) => (
									<button
										key={formatSuggestionAddress(s)}
										type="button"
										className="seller-addr-confirm__alternate"
										onClick={() => applySuggestion(s)}
									>
										{formatSuggestionAddress(s)}
									</button>
								))}
							</div>
						) : null}
						{showVerificationNotice ? <AddressVerificationNotice /> : null}
					</div>
				) : (
					<div className="seller-addr-confirm">
						<div className="seller-addr-confirm__block">
							<span className="seller-addr-confirm__label">目前輸入</span>
							<p className="seller-addr-confirm__text">{currentFullAddress}</p>
						</div>
						{formatCandidate ? (
							<div className="seller-addr-confirm__block seller-addr-confirm__block--suggested">
								<span className="seller-addr-confirm__label">可先改成</span>
								<p className="seller-addr-confirm__suggestion">
									{`${county ?? ""}${district ?? ""}${formatCandidate}`}
								</p>
							</div>
						) : null}
						<ul className="seller-addr-confirm__tips">
							<li>
								從名片複製時，路名門牌欄不用重複輸入郵遞區號、縣市與鄉鎮區。
							</li>
							<li>門牌分號可改成「之」，例如「113之50號」。</li>
						</ul>
						<AddressVerificationNotice />
					</div>
				)}
			</Dialog>
		</div>
	);
}

function AddressVerificationNotice() {
	return (
		<div className="seller-addr-confirm__notice">
			<strong>為什麼一定要驗證地址？</strong>
			<p>
				黑貓宅急便開通與後續叫件會使用這個寄件地址。地址必須能被系統驗證，才能送出物流開通申請，避免之後黑貓無法到府收件。
			</p>
			<p>先確認縣市、區域與路名門牌是否一致再驗證一次。</p>
			<p>
				若這是真實存在的地址仍無法通過，請聯絡客服{" "}
				<a href={`mailto:${SUPPORT_EMAIL}`}>{SUPPORT_EMAIL}</a>
				，我們會協助你完成開通，不需要一直重試。
			</p>
		</div>
	);
}

async function getSuggestedAddressCandidates(params: {
	county: string;
	district: string;
	query: string;
}): Promise<{ items: AddressSuggestion[]; candidate: string | null }> {
	const original = params.query.trim();
	const candidate = normalizeAddressDetailCandidate(
		original,
		params.county,
		params.district,
	);
	const primary = dedupeAddressSuggestions(
		await getAddressSuggestions({
			...params,
			query: original,
		}),
	);
	if (primary.length > 0 || !candidate || candidate === original) {
		return { items: primary, candidate };
	}
	const fallback = dedupeAddressSuggestions(
		await getAddressSuggestions({
			...params,
			query: candidate,
		}),
	);
	return { items: fallback, candidate };
}

export function normalizeAddressDetailCandidate(
	value: string,
	county: string,
	district: string,
): string | null {
	const original = value.trim();
	if (!original) return null;
	let next = toLogisticsName(toHalfWidth(original)).replace(/^\d{3,5}/, "");
	const prefixes = [`${county}${district}`, county, district].map((s) =>
		toLogisticsName(toHalfWidth(s)),
	);
	for (const prefix of prefixes) {
		if (prefix && next.startsWith(prefix)) {
			next = next.slice(prefix.length);
			break;
		}
	}
	next = next
		.replace(/([0-9])\s*[-－–—]\s*([0-9])/g, "$1之$2")
		.replace(/\s+/g, "")
		.trim();
	return next && next !== original ? next : null;
}

function toHalfWidth(value: string): string {
	return value.replace(/[！-～]/g, (char) =>
		String.fromCharCode(char.charCodeAt(0) - 0xfee0),
	);
}

function formatSuggestionAddress(s: AddressSuggestion): string {
	if (s.fullAddress) return s.fullAddress;
	return `${s.county ?? ""}${s.district ?? ""}${s.detail ?? ""}`.trim();
}

function dedupeAddressSuggestions(
	items: AddressSuggestion[],
): AddressSuggestion[] {
	const seen = new Set<string>();
	const out: AddressSuggestion[] = [];
	for (const item of items) {
		const key = formatSuggestionAddress(item).replace(/\s|　/g, "");
		if (!key || seen.has(key)) continue;
		seen.add(key);
		out.push(item);
		if (out.length >= 4) break;
	}
	return out;
}
