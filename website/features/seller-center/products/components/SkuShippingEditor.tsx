"use client";

/**
 * Per-SKU 可配送方案 picker — the web port of the seller app's product-spec
 * shipping section (product_setup_spec_screen.dart). The seller enters the
 * package 長/寬/高 + 重量; we debounce-call `POST /app/shipping_quotes`, which
 * returns the carrier × 溫層 combinations that actually fit the package (黑貓 /
 * 7-11, per-carrier size & weight limits) with their fees.
 *
 * We always render the FULL carrier × 溫層 matrix so the seller sees every
 * possible option up-front; only the combinations the quote says fit are
 * enabled (with fee + ETA), the rest stay greyed until the dimensions change.
 * Each selected option carries one coherent shipping rule: the buyer pays, or
 * the seller provides free shipping (optionally above a spending threshold).
 * That rule is persisted onto `sku.shipping_methods`.
 */

import * as React from "react";
import { Dropdown } from "@/components/ui";
import { Input } from "@/components/ui/Input";
import { TemperatureTag } from "@/features/cart/components/TemperatureTag";
import {
	coerceWeightUnitCode,
	SHIPPING_METHOD_LABEL,
	ShippingFeePayer,
	ShippingMethod,
	type ShippingMethodValue,
	ShippingTemperature,
	WEIGHT_UNIT_LABEL,
	type WeightUnitValue,
} from "@/lib/codemeta/enums";
import { useShippingOnboardings } from "@/lib/hooks/useShippingOnboardings";
import {
	fetchShippingQuotes,
	type ProductSku,
	type ShippingQuoteOption,
	type SkuShippingMethod,
} from "@/lib/products/seller-repo";
import {
	type ShippingMethodOpenState,
	shippingMethodOpenState,
} from "@/lib/seller/shipping";

interface Props {
	sku: ProductSku;
	index: number;
	onUpdate: (index: number, patch: Partial<ProductSku>) => void;
}

/** Carriers offered in the product form (mirrors the seller app: 黑貓 + 7-11). */
const CARRIERS: ShippingMethodValue[] = [
	ShippingMethod.TAQBIN,
	ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP,
];
const TEMPS: string[] = [
	ShippingTemperature.AMBIENT,
	ShippingTemperature.CHILLED,
	ShippingTemperature.FROZEN,
];
/** Full carrier × 溫層 grid, always rendered so every option is visible. */
const FULL_MATRIX: Array<{ type: string; temperature: string }> =
	CARRIERS.flatMap((type) =>
		TEMPS.map((temperature) => ({ type, temperature })),
	);

const WEIGHT_UNIT_ITEMS: ReadonlyArray<{ value: string; label: string }> = (
	Object.keys(WEIGHT_UNIT_LABEL) as WeightUnitValue[]
).map((code) => ({ value: code, label: WEIGHT_UNIT_LABEL[code] }));

const optionKey = (type: string, temperature: string) =>
	`${type}|${temperature}`;

function methodLabel(m: string, fallback?: string): string {
	return (
		fallback?.trim() || SHIPPING_METHOD_LABEL[m as ShippingMethodValue] || m
	);
}

/** 量測欄齊全（長寬高+重量皆 > 0）才值得打試算 API。 */
function dimsReady(sku: ProductSku): boolean {
	const pos = (v?: string) => Number(v) > 0;
	return (
		pos(sku.length_cm) &&
		pos(sku.width_cm) &&
		pos(sku.height_cm) &&
		pos(sku.weight)
	);
}

type QuoteState = "idle" | "loading" | "ready" | "error";

export default function SkuShippingEditor({ sku, index, onUpdate }: Props) {
	const selected = sku.shipping_methods ?? [];

	// 後端 product_sku.unit 本身即為重量單位（公斤/台斤/公克），
	// 結帳依此換算成公克算運費，故 label 與運費試算單位都以 sku.unit 為準，
	// 單位選擇器放在重量輸入旁，避免 label 與實際儲存單位不一致。
	const unitCode = coerceWeightUnitCode(sku.unit);
	const unitLabel = WEIGHT_UNIT_LABEL[unitCode];

	// Seller's 物流設定 onboarding rows — so an unavailable combination reads
	// 「尚未開通」when the carrier × 溫層 isn't opened, instead of always blaming
	// size / weight. Until the fetch resolves we treat everything as opened so a
	// pending load can't flash a wrong「未開通」on every row.
	const { rows: onboardingRows, loaded: onboardingLoaded } =
		useShippingOnboardings();

	const [quotes, setQuotes] = React.useState<ShippingQuoteOption[]>([]);
	const [state, setState] = React.useState<QuoteState>("idle");
	const [errorMsg, setErrorMsg] = React.useState<string | null>(null);

	// Debounced quote fetch. A monotonically-increasing request id makes sure a
	// slow earlier response can't overwrite a newer one (last-write-wins).
	const reqId = React.useRef(0);
	const length = sku.length_cm ?? "";
	const width = sku.width_cm ?? "";
	const height = sku.height_cm ?? "";
	const weight = sku.weight ?? "";
	const ready = dimsReady(sku);

	React.useEffect(() => {
		if (!ready) {
			setState("idle");
			setQuotes([]);
			setErrorMsg(null);
			return;
		}
		const id = ++reqId.current;
		setState("loading");
		const timer = setTimeout(() => {
			void fetchShippingQuotes({
				length,
				width,
				height,
				weight,
				// 後端依重量旁選擇的 sku.unit 換算運費（issue #1316）。
				weightUnit: unitCode,
			})
				.then((res) => {
					if (id !== reqId.current) return;
					setQuotes(res.shippingOptions);
					setState("ready");
					setErrorMsg(null);
				})
				.catch((e) => {
					if (id !== reqId.current) return;
					setQuotes([]);
					setState("error");
					setErrorMsg(
						e instanceof Error ? e.message : "運費試算失敗，請稍後再試",
					);
				});
		}, 500);
		return () => clearTimeout(timer);
	}, [ready, length, width, height, weight, unitCode]);

	const setShipping = (next: SkuShippingMethod[]) =>
		onUpdate(index, { shipping_methods: next.length ? next : undefined });

	const toggle = (type: string, temperature: string) => {
		const key = optionKey(type, temperature);
		const exists = selected.some(
			(m) => optionKey(m.type, m.temperature) === key,
		);
		if (exists) {
			setShipping(
				selected.filter((m) => optionKey(m.type, m.temperature) !== key),
			);
		} else {
			setShipping([
				...selected,
				{
					type,
					temperature,
					fee_payer: ShippingFeePayer.BUYER,
					free_threshold: "0",
				},
			]);
		}
	};

	const patchEntry = (key: string, patch: Partial<SkuShippingMethod>) =>
		setShipping(
			selected.map((m) =>
				optionKey(m.type, m.temperature) === key ? { ...m, ...patch } : m,
			),
		);

	// Full matrix + any already-selected combo that's outside it (e.g. an
	// option created on mobile with a carrier we don't list), so nothing the
	// seller saved silently disappears.
	const cards = [
		...FULL_MATRIX,
		...selected
			.filter(
				(m) =>
					!FULL_MATRIX.some(
						(c) => c.type === m.type && c.temperature === m.temperature,
					),
			)
			.map((m) => ({ type: m.type, temperature: m.temperature })),
	];

	const selectedCount = selected.length;

	// Controlled `open` (seeded from whether shipping is already set) with an
	// onToggle sync — otherwise a re-render mid-typing would yank a manually
	// expanded <details> shut.
	const [open, setOpen] = React.useState(selectedCount > 0);

	return (
		<details
			className="group/ship border-t border-dashed border-divider pt-3"
			open={open}
			onToggle={(e) => setOpen((e.currentTarget as HTMLDetailsElement).open)}
		>
			<summary className="list-none [&::-webkit-details-marker]:hidden cursor-pointer text-sm text-foreground-muted py-1 flex items-center gap-2 select-none before:content-['▸'] before:transition-transform before:duration-150 group-open/ship:before:rotate-90">
				可配送方案
				{selectedCount > 0 ? (
					<span className="bg-hkui-primary/12 text-hkui-primary px-2 py-0.5 rounded-full text-[0.6875rem] font-bold">
						已選 {selectedCount} 個
					</span>
				) : (
					<span className="text-xs text-hk-muted font-normal">
						（黑貓 / 7-11，未設定）
					</span>
				)}
			</summary>

			<div className="mt-2 flex flex-col gap-3">
				{/* 包裝尺寸 */}
				<div className="grid grid-cols-3 gap-2 items-end">
					<DimField
						label="長 (cm)"
						value={sku.length_cm}
						onChange={(v) => onUpdate(index, { length_cm: v })}
					/>
					<DimField
						label="寬 (cm)"
						value={sku.width_cm}
						onChange={(v) => onUpdate(index, { width_cm: v })}
					/>
					<DimField
						label="高 (cm)"
						value={sku.height_cm}
						onChange={(v) => onUpdate(index, { height_cm: v })}
					/>
				</div>
				{/* 重量與單位一起填寫，運費試算依此換算 */}
				<div className="grid grid-cols-[minmax(0,2fr)_minmax(120px,1fr)] gap-2 items-end">
					<DimField
						label={`重量 (${unitLabel})`}
						value={sku.weight}
						placeholder="0"
						onChange={(v) => onUpdate(index, { weight: v })}
					/>
					<UnitField
						value={sku.unit}
						onChange={(v) => onUpdate(index, { unit: v })}
					/>
				</div>

				{/* 狀態提示（矩陣永遠顯示在下方，可選的會亮起） */}
				{state === "idle" && (
					<p className="text-xs text-hk-muted m-0">
						填寫包裝長寬高與重量（{unitLabel}
						）後，下方可配送的方式會亮起、可勾選。
					</p>
				)}
				{state === "loading" && (
					<p className="text-xs text-hk-muted m-0 flex items-center gap-2">
						<span className="inline-block w-3 h-3 rounded-full border-2 border-hkui-primary/30 border-t-hkui-primary animate-spin" />
						計算可用的配送方式…
					</p>
				)}
				{state === "error" && (
					<p
						className="text-xs text-hk-danger m-0 py-2 px-3 bg-hk-danger/8 rounded-md"
						role="alert"
					>
						{errorMsg ?? "運費試算失敗"}
					</p>
				)}

				<ul className="list-none m-0 p-0 flex flex-col gap-2">
					{cards.map(({ type, temperature }) => {
						const key = optionKey(type, temperature);
						const option = quotes.find(
							(o) => optionKey(o.shippingMethod, o.temperature) === key,
						);
						const method = selected.find(
							(m) => optionKey(m.type, m.temperature) === key,
						);
						const openState: ShippingMethodOpenState = onboardingLoaded
							? shippingMethodOpenState(onboardingRows, type, temperature)
							: "open";
						return (
							<ShippingOptionCard
								key={key}
								type={type}
								temperature={temperature}
								option={option}
								method={method}
								available={!!option}
								openState={openState}
								status={state}
								onToggle={() => toggle(type, temperature)}
								onFundingChange={(sellerPays) =>
									patchEntry(key, {
										fee_payer: sellerPays
											? ShippingFeePayer.SELLER
											: ShippingFeePayer.BUYER,
										// A buyer-paid method must never carry an orphaned
										// threshold. A seller-paid method with 0 is
										// intentionally unconditional free shipping.
										free_threshold: sellerPays
											? method?.fee_payer === ShippingFeePayer.SELLER
												? method.free_threshold || "0"
												: "0"
											: "0",
									})
								}
								onThreshold={(v) =>
									patchEntry(key, {
										fee_payer: ShippingFeePayer.SELLER,
										free_threshold: v || "0",
									})
								}
							/>
						);
					})}
				</ul>
			</div>
		</details>
	);
}

function DimField({
	label,
	value,
	placeholder,
	onChange,
}: {
	label: string;
	value?: string;
	placeholder?: string;
	onChange: (v: string) => void;
}) {
	return (
		<Input
			variant="bordered"
			size="lg"
			type="number"
			min="0"
			label={label}
			value={value ?? ""}
			placeholder={placeholder ?? "0"}
			onChange={(e) => onChange(e.target.value)}
		/>
	);
}

function UnitField({
	value,
	onChange,
}: {
	value?: string;
	onChange: (v: string) => void;
}) {
	return (
		<Dropdown
			label="重量單位"
			variant="bordered"
			size="lg"
			value={coerceWeightUnitCode(value)}
			onChange={onChange}
			items={WEIGHT_UNIT_ITEMS}
		/>
	);
}

function ShippingOptionCard({
	type,
	temperature,
	option,
	method,
	available,
	openState,
	status,
	onToggle,
	onFundingChange,
	onThreshold,
}: {
	type: string;
	temperature: string;
	option?: ShippingQuoteOption;
	method?: SkuShippingMethod;
	available: boolean;
	openState: ShippingMethodOpenState;
	status: QuoteState;
	onToggle: () => void;
	onFundingChange: (sellerPays: boolean) => void;
	onThreshold: (v: string) => void;
}) {
	const isSelected = !!method;
	// A carrier × 溫層 the seller hasn't opened in 物流設定 can never ship this
	// product, so block it regardless of the size / weight quote — and say why.
	const blockedByOnboarding =
		openState === "not_opened" || openState === "disabled";
	const effectiveAvailable = available && !blockedByOnboarding;
	// Can't pick a combo that doesn't fit or isn't opened; a previously-selected
	// one stays toggleable so the seller can remove it.
	const disabled = !effectiveAvailable && !isSelected;
	const sellerPays = method?.fee_payer === ShippingFeePayer.SELLER;
	const fundingName = React.useId();
	const thresholdInputId = React.useId();

	const onboardingMsg =
		openState === "disabled" ? "此配送方式已停用" : "尚未開通此配送方式";
	const subtext = blockedByOnboarding
		? isSelected
			? `${onboardingMsg}（已選，可取消）`
			: onboardingMsg
		: effectiveAvailable
			? `運費 NT$${option?.fee}${option?.estimatedDays ? ` · 約 ${option.estimatedDays} 天` : ""}`
			: isSelected
				? "目前尺寸不可配送（已選，可取消）"
				: status === "idle"
					? "填寫包裝尺寸後試算"
					: status === "loading"
						? "計算中…"
						: status === "error"
							? "暫時無法試算"
							: "此尺寸 / 重量不可配送";
	const subtextTone =
		!effectiveAvailable && isSelected ? "text-hk-danger" : "text-hk-muted";

	const cardTone = isSelected
		? "border-hkui-primary bg-hkui-primary/6 dark:border-hk-ink-200 dark:bg-hk-ink-200/12"
		: disabled
			? "border-hk-border-soft bg-hk-surface-muted/40 dark:border-[#f5dea5]/10 dark:bg-white/4"
			: "border-hk-border-soft bg-hk-surface-card dark:border-[#f5dea5]/14 dark:bg-white/5";

	return (
		<li
			className={`rounded-hkui-lg border p-3 flex flex-col gap-2 transition-colors ${cardTone}`}
		>
			<label
				className={`flex items-center gap-2.5 select-none ${
					disabled ? "cursor-not-allowed opacity-55" : "cursor-pointer"
				}`}
			>
				<input
					type="checkbox"
					checked={isSelected}
					disabled={disabled}
					onChange={onToggle}
					className="w-[18px] h-[18px] accent-hkui-primary shrink-0"
				/>
				<span className="flex items-center gap-2 flex-wrap min-w-0">
					<strong className="text-sm font-bold text-foreground">
						{methodLabel(type, option?.displayName)}
					</strong>
					<TemperatureTag value={temperature} />
					<span className={`text-xs ${subtextTone}`}>{subtext}</span>
				</span>
			</label>

			{isSelected && (
				<div className="flex flex-col gap-2 pl-[28px] max-[768px]:pl-0">
					<fieldset className="flex flex-wrap items-center gap-x-4 gap-y-2 border-0 p-0 m-0">
						<legend className="sr-only">運費設定</legend>
						<span className="text-xs font-semibold text-foreground-muted">
							運費設定
						</span>
						<label className="flex items-center gap-1.5 cursor-pointer select-none">
							<input
								type="radio"
								name={fundingName}
								checked={!sellerPays}
								onChange={() => onFundingChange(false)}
								className="w-[16px] h-[16px] accent-hkui-primary shrink-0"
							/>
							<span className="text-xs text-foreground-muted">
								買家負擔運費
							</span>
						</label>
						<label className="flex items-center gap-1.5 cursor-pointer select-none">
							<input
								type="radio"
								name={fundingName}
								checked={sellerPays}
								onChange={() => onFundingChange(true)}
								className="w-[16px] h-[16px] accent-hkui-primary shrink-0"
							/>
							<span className="text-xs text-foreground-muted">
								賣家提供免運
							</span>
						</label>
					</fieldset>

					{sellerPays ? (
						<div className="flex flex-wrap items-center gap-x-2 gap-y-1">
							<label
								htmlFor={thresholdInputId}
								className="text-xs text-foreground-muted whitespace-nowrap"
							>
								免運門檻 NT$
							</label>
							<input
								id={thresholdInputId}
								type="number"
								min="0"
								inputMode="numeric"
								value={
									method?.free_threshold === "0" ? "" : method?.free_threshold
								}
								placeholder="留空 = 無條件免運"
								onChange={(e) => onThreshold(e.target.value)}
								className="w-44 h-8 px-2 rounded-md border border-hk-border-soft bg-hk-surface-card text-sm text-foreground focus:outline-none focus:border-hkui-primary dark:border-[#f5dea5]/16 dark:bg-white/8 dark:text-[#f8f1df] dark:placeholder:text-[#f8f1df]/45 dark:focus:border-hk-ink-200"
							/>
							<span className="text-xs text-hk-muted">
								填入金額後，買家滿額才免運。
							</span>
						</div>
					) : null}
				</div>
			)}
		</li>
	);
}
