"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { DatePicker, Input, Textarea } from "@/components/ui";
import Button from "@/components/ui/Button";
import {
	inputWrapperClasses,
	TEXTAREA_RESET,
} from "@/components/ui/inputWrapperClasses";
import SendButton from "@/features/messages/components/SendButton";
import { taipeiParts } from "@/lib/date/taipei";
import { validateFreeTextAnswer } from "./answer-validation";
import HawkrAgentMark from "./HawkrAgentMark";
import { questionHasUnitOptions } from "./session-logic";
import type { ClarifyQuestion } from "./types";

interface ClarifyQuestionCardProps {
	question: ClarifyQuestion;
	pending: boolean;
	onPick: (option: {
		slotKey: string;
		optionId: string;
		optionLabel: string;
	}) => void;
	/** 卡片內的自由文字答案。crop_disambiguation 的 allowFreeText=true 時
	 * 用來重新查找品項；不帶 crop-other。 */
	onSubmitFreeText?: (freeText: string) => void;
	/** 跳過這題（僅選填欄位；必填欄位由呼叫端不傳這個 prop 來隱藏按鈕）。
	 * 未傳＝不渲染跳過按鈕。 */
	onSkip?: (payload: { slotKey: string; skip: true }) => void;
	/** quantity 複合問題卡／單位追問卡點選單位（backend-payuni#2442 §4/§5）。
	 * 複合問題卡會帶上本地已輸入的 freeText 一起送出，單位追問卡只帶
	 * unitOptionId。未傳＝不渲染單位 chips。 */
	onPickUnit?: (payload: {
		slotKey: string;
		unitOptionId: string;
		unitLabel: string;
		freeText?: string;
	}) => void;
	/** crop_disambiguation 問句選「D 其他」後的自填描述提交
	 * （品項分類消歧 gate）。未傳＝不渲染 D 子表單提交能力
	 * （但 D chip 本身仍會展開表單，只是送不出去——理論上呼叫端一律會傳）。 */
	onSubmitCropOther?: (payload: { description: string }) => void;
	/** 上一題以 chip 作答時，新卡掛載後把焦點還給第一顆選項——
	 * 鍵盤流程不落回 <body>。 */
	autoFocusFirstOption?: boolean;
}

const QUESTION_INPUT_HINTS: Record<
	string,
	{
		placeholder: string;
		inputMode?: React.ComponentProps<"textarea">["inputMode"];
	}
> = {
	item: { placeholder: "例如：芭樂" },
	crop_disambiguation: { placeholder: "例如：豬五花、帶皮五花肉" },
	quantity: { placeholder: "例如：50（只填數字）", inputMode: "numeric" },
	unit: { placeholder: "例如：籃" },
	packaging: { placeholder: "例如：每箱 10 公斤" },
	budget_per_unit: {
		placeholder: "例如：800（只填數字）",
		inputMode: "numeric",
	},
	delivery_city: { placeholder: "例如：新竹市" },
	delivery_date: {
		placeholder: "例如：2026-08-20",
		inputMode: "numeric",
	},
};

/** 到貨日改用全站共用的 DatePicker（components/ui），不再讓買家手打
 * `YYYY-MM-DD`——手打會踩到後端 `domain.Slots.Apply` 的日期格式驗證，而
 * 錯字要等一次 round trip 才知道。選出來的值本來就是 ISO，正好是 /answers
 * 期待的格式。 */
const DATE_SLOT_KEY = "delivery_date";

/** 到貨日不能挑今天以前。上限一年——再遠的採購需求在這個流程沒有意義，
 * 而日曆放到 2099 只會讓誤點的成本變高。
 *
 * 「今天」一律以**台北牆上時間**計算，不用 `getFullYear` 家族：那一族跟著
 * 瀏覽器時區跑，人在時差另一邊的買家會拿到差一天的下限（#374／#391 的同一個
 * root cause，`tests/date-formatting-taipei.test.ts` 也會擋）。這裡的日期是台灣
 * 的到貨日，台北時間才是正解。 */
function taipeiIsoDate(offsetDays: number): string {
	const parts = taipeiParts(Date.now() + offsetDays * 24 * 60 * 60 * 1000);
	if (!parts) return "";
	const m = String(parts.month).padStart(2, "0");
	const d = String(parts.day).padStart(2, "0");
	return `${parts.year}-${m}-${d}`;
}

const DATE_MIN_OFFSET_DAYS = 0;
/** 365 天：不處理閏年差一天，這是日曆上限不是結算日。 */
const DATE_MAX_OFFSET_DAYS = 365;

/** 品項分類消歧問句（crop taxonomy gate）：
 * 動態 A/B/C 候選＋固定 D「其他」。D 選項不直接送 optionId，而是在本卡展開
 * 描述子表單（見下方 showCropOtherForm）。wire 值已與後端多輪對齊確認，
 * 不得改動。 */
const CROP_DISAMBIGUATION_SLOT_KEY = "crop_disambiguation";
const CROP_OTHER_OPTION_ID = "crop-other";

/**
 * 引導式精確釐清的問題卡（B-244 Ask 3；規劃書 §3.3）。一次一題＋
 * 選項 chips：canonical value 出自伺服器白名單，client 只送 optionId。
 * 不是需求快照卡的復活——e2e 釘住需求快照的舊 class 不得存在，這裡全部用新 class。
 */
export default function ClarifyQuestionCard(props: ClarifyQuestionCardProps) {
	// 大類、小類與自行描述品項可能沿用同一個 slotKey；題目內容改變時，上一題的
	// 描述與展開狀態不能留在新題目。pending 或 callback 更新不清掉草稿。
	const questionKey = JSON.stringify(props.question);
	return <ClarifyQuestionForm key={questionKey} {...props} />;
}

function ClarifyQuestionForm({
	question,
	pending,
	onPick,
	onSubmitFreeText,
	onSkip,
	onPickUnit,
	onSubmitCropOther,
	autoFocusFirstOption = false,
}: ClarifyQuestionCardProps) {
	const firstOptionRef = React.useRef<HTMLButtonElement | null>(null);
	const firstUnitOptionRef = React.useRef<HTMLButtonElement | null>(null);
	const freeTextRef = React.useRef<HTMLTextAreaElement | null>(null);
	const cropOtherDescriptionRef = React.useRef<HTMLTextAreaElement | null>(
		null,
	);
	const [freeText, setFreeText] = React.useState("");
	const [freeTextError, setFreeTextError] = React.useState<string | null>(null);
	const [showCustomAnswer, setShowCustomAnswer] = React.useState(
		question.options.length === 0,
	);
	const inputHint = QUESTION_INPUT_HINTS[question.slotKey] ?? {
		placeholder: "請輸入答案",
	};
	const isDateQuestion = question.slotKey === DATE_SLOT_KEY;
	const dateInputId = `clarify-free-text-${question.slotKey}`;
	const [fieldFocused, setFieldFocused] = React.useState(false);
	const canSubmitFreeText =
		question.allowFreeText && Boolean(freeText.trim()) && !pending;

	// 品項分類消歧問句的 D「其他」子表單：本地展開，不經 onPick。
	const [showCropOtherForm, setShowCropOtherForm] = React.useState(false);
	const [cropOtherDescription, setCropOtherDescription] = React.useState("");
	const isCropDisambiguation =
		question.slotKey === CROP_DISAMBIGUATION_SLOT_KEY;
	const canRetryCropDescription =
		isCropDisambiguation && question.allowFreeText;
	// 沒有候選分類時，直接填寫即可，不再要求點選唯一的「其他」。
	const isCropOtherOnly =
		isCropDisambiguation &&
		!question.allowFreeText &&
		question.options.length === 1 &&
		question.options[0]?.optionId === CROP_OTHER_OPTION_ID;
	const cropOtherDescriptionId = `clarify-crop-other-description-${question.slotKey}`;
	const canSubmitCropOther =
		isCropDisambiguation && Boolean(cropOtherDescription.trim()) && !pending;

	// quantity 複合問題卡／單位追問卡（backend-payuni#2442 §4）：quantity 本身
	// 沒有 canonical options，unitOptions 是它獨立的單位子值，只有 Unit 尚未填
	// 時後端才會帶。allowFreeText 分岔出兩種型態，同一個 slotKey。
	const unitOptions = question.unitOptions ?? [];
	const hasUnitOptions = questionHasUnitOptions(question);
	const isCompoundQuantity = hasUnitOptions && question.allowFreeText;
	const isUnitFollowUp = hasUnitOptions && !question.allowFreeText;
	const quantityReady =
		!isCompoundQuantity ||
		validateFreeTextAnswer(question.slotKey, freeText) === null;

	React.useEffect(() => {
		if (!autoFocusFirstOption) return;
		const frame = requestAnimationFrame(() => {
			if (isCropOtherOnly) {
				cropOtherDescriptionRef.current?.focus();
				return;
			}
			if (question.options.length > 0) {
				firstOptionRef.current?.focus();
				return;
			}
			// 日期題的欄位在 DatePicker 內部，拿不到 ref——用它自己的 id 取。
			// 少了這段，日期題掛載時焦點會留在 <body>，鍵盤流程斷掉。
			if (isDateQuestion) {
				document.getElementById(dateInputId)?.focus();
				return;
			}
			// 單位追問卡沒有數字欄——焦點還給第一顆單位 chip。
			if (isUnitFollowUp) {
				firstUnitOptionRef.current?.focus();
				return;
			}
			freeTextRef.current?.focus();
		});
		return () => cancelAnimationFrame(frame);
	}, [
		autoFocusFirstOption,
		isCropOtherOnly,
		question.options.length,
		isDateQuestion,
		isUnitFollowUp,
		dateInputId,
	]);

	const submitFreeText = () => {
		// 複合問題卡不透過這裡送出——數字欄只做本地 staging，湊齊單位後靠點
		// chip 一次送出 freeText+unitOptionId（見下方 onPickUnit）。
		if (isCompoundQuantity) return;
		const answer = freeText.trim();
		if (!answer || !canSubmitFreeText || !onSubmitFreeText) return;
		const validationError = validateFreeTextAnswer(question.slotKey, answer);
		if (validationError) {
			setFreeTextError(validationError);
			return;
		}
		onSubmitFreeText(answer);
		setFreeText("");
		setFreeTextError(null);
	};

	const openCustomAnswer = () => {
		setShowCustomAnswer(true);
		requestAnimationFrame(() => freeTextRef.current?.focus());
	};

	const openCropOtherForm = () => {
		setShowCropOtherForm(true);
		requestAnimationFrame(() => cropOtherDescriptionRef.current?.focus());
	};

	const submitCropOther = () => {
		if (!canSubmitCropOther || !onSubmitCropOther) return;
		onSubmitCropOther({
			description: cropOtherDescription.trim(),
		});
		setShowCropOtherForm(false);
		setCropOtherDescription("");
	};

	return (
		<li className="hawkr-agent-question-row">
			<HawkrAgentMark
				label="Hawkr Agent"
				className="assistant-mark hawkr-agent-message-avatar hawkr-agent-message-avatar--agent"
			/>
			<section
				className="hawkr-agent-question-card"
				data-testid="clarify-question-card"
				data-crop-disambiguation={isCropDisambiguation || undefined}
				data-crop-other-only={isCropOtherOnly || undefined}
				data-question-kind={
					isCompoundQuantity
						? "quantity-unit"
						: isUnitFollowUp
							? "unit"
							: question.options.length > 0
								? "choice"
								: "text"
				}
				aria-labelledby={`clarify-question-${question.slotKey}`}
			>
				<header className="hawkr-agent-question-card__head">
					<h2 id={`clarify-question-${question.slotKey}`}>
						{isCropOtherOnly ? "自訂採購品項" : question.prompt}
					</h2>
					<p>
						{isCropOtherOnly
							? question.prompt
							: canRetryCropDescription
								? "選擇符合的品項，也可以補充名稱或規格重新查找。"
								: isCropDisambiguation
									? "選擇最接近的分類，或自行填寫品項。"
									: isCompoundQuantity
										? "先輸入數量，再選擇單位。"
										: isUnitFollowUp
											? "選擇這批貨要用的單位。"
											: question.options.length > 0
												? "選擇最接近的答案，也可以直接輸入。"
												: onSkip
													? "在下方輸入答案，或選擇跳過。"
													: "在下方輸入答案。"}
					</p>
				</header>

				{question.options.length > 0 ||
				question.allowFreeText ||
				onSkip ||
				hasUnitOptions ? (
					<div className="hawkr-agent-question-card__controls">
						{question.options.length > 0 && !isCropOtherOnly ? (
							<fieldset
								className="hawkr-agent-question-card__options"
								aria-label="回答選項"
							>
								{question.options.map((option, index) => {
									const isCropOtherOption =
										isCropDisambiguation &&
										option.optionId === CROP_OTHER_OPTION_ID;
									return (
										<button
											key={option.optionId}
											ref={index === 0 ? firstOptionRef : undefined}
											type="button"
											disabled={pending}
											onClick={() => {
												if (isCropOtherOption) {
													openCropOtherForm();
													return;
												}
												onPick({
													slotKey: question.slotKey,
													optionId: option.optionId,
													optionLabel: option.label,
												});
											}}
											aria-expanded={
												isCropOtherOption ? showCropOtherForm : undefined
											}
											aria-controls={
												isCropOtherOption ? cropOtherDescriptionId : undefined
											}
											className="hawkr-agent-question-card__option"
											data-selected={
												(isCropOtherOption && showCropOtherForm) || undefined
											}
										>
											<span>{option.label}</span>
											<span className="hawkr-agent-question-card__option-mark">
												<AppIcon name="check" size={16} aria-hidden="true" />
											</span>
										</button>
									);
								})}
								{question.options.length > 0 &&
								question.allowFreeText &&
								!isCropDisambiguation ? (
									<button
										type="button"
										disabled={pending}
										onClick={openCustomAnswer}
										className="hawkr-agent-question-card__option hawkr-agent-question-card__option--other"
										aria-expanded={showCustomAnswer}
										aria-controls={`clarify-free-text-${question.slotKey}`}
									>
										<span>{showCustomAnswer ? "自行輸入" : "其他…"}</span>
										<AppIcon name="edit" size={18} aria-hidden="true" />
									</button>
								) : null}
							</fieldset>
						) : null}

						{(showCustomAnswer || canRetryCropDescription) &&
						question.allowFreeText ? (
							<>
								<form
									className={[
										"hawkr-agent-question-card__free-text-form",
										question.options.length > 0
											? "hawkr-agent-question-card__free-text-form--custom"
											: "",
										// 日期題的外框由 DatePicker 自己畫；這裡再套一層
										// 就會是雙框。文字題則相反：這個 form 本身就是
										// 那個輸入框，所以直接吃共用的 wrapper 樣式，
										// 跟同一張卡裡的 DatePicker 長得一模一樣。
										isDateQuestion
											? "hawkr-agent-question-card__free-text-form--bare"
											: inputWrapperClasses({
													variant: "bordered",
													size: "lg",
													focused: fieldFocused,
													invalid: Boolean(freeTextError),
													disabled: pending || !onSubmitFreeText,
													floating: false,
													// 這是一行答案（數量／包裝／交期…），不是多行輸入。
													// textarea:true 會換成 `items-start + py-2.5 +
													// min-h-auto` 的多行度量，box 比全站其他輸入框高一截；
													// 用單行度量才會跟 DatePicker、<Input> 一樣是 lg 的
													// 48px。元素本身仍是 textarea（要吃 Enter 送出與
													// 換行組字），高度由下面的 CSS 收成一行。
													textarea: false,
												}),
									]
										.filter(Boolean)
										.join(" ")}
									onFocus={() => setFieldFocused(true)}
									onBlur={() => setFieldFocused(false)}
									onSubmit={(event) => {
										event.preventDefault();
										submitFreeText();
									}}
								>
									<label
										htmlFor={`clarify-free-text-${question.slotKey}`}
										className="sr-only"
									>
										{canRetryCropDescription
											? "重新描述品項"
											: `${question.prompt}的答案`}
									</label>
									{isDateQuestion ? (
										<DatePicker
											id={dateInputId}
											className="hawkr-agent-question-card__date"
											variant="bordered"
											size="lg"
											value={freeText}
											onChange={(next) => {
												setFreeText(next);
												setFreeTextError(null);
											}}
											minDate={taipeiIsoDate(DATE_MIN_OFFSET_DAYS)}
											maxDate={taipeiIsoDate(DATE_MAX_OFFSET_DAYS)}
											placeholder={inputHint.placeholder}
											disabled={pending || !onSubmitFreeText}
											isInvalid={Boolean(freeTextError)}
										/>
									) : (
										<textarea
											id={`clarify-free-text-${question.slotKey}`}
											// foundation 的 bare-control baseline 會給「沒有
											// data-slot 的原生控制項」畫邊框＋focus glow，因為它
											// 假設那是獨立欄位。這個 textarea 住在共用 wrapper
											// 裡（form 就是外框），少了這個屬性就會在外框裡再畫
											// 一個內縮的小框——那正是 02-reset-layout.css 該段
											// 註解在講的 double frame。共用 <Input>/<Textarea>
											// 也是靠同一個屬性豁免。
											data-slot="textarea"
											className={TEXTAREA_RESET}
											ref={freeTextRef}
											value={freeText}
											onChange={(event) => {
												setFreeText(event.currentTarget.value);
												setFreeTextError(null);
											}}
											onKeyDown={(event) => {
												if (
													event.key !== "Enter" ||
													event.shiftKey ||
													event.nativeEvent.isComposing
												)
													return;
												event.preventDefault();
												submitFreeText();
											}}
											placeholder={inputHint.placeholder}
											inputMode={inputHint.inputMode}
											autoComplete="off"
											aria-invalid={freeTextError ? "true" : undefined}
											aria-describedby={
												freeTextError
													? `clarify-free-text-error-${question.slotKey}`
													: undefined
											}
											rows={1}
											maxLength={200}
											disabled={pending || !onSubmitFreeText}
										/>
									)}
									{!isCompoundQuantity ? (
										<SendButton
											type="submit"
											disabled={!canSubmitFreeText || !onSubmitFreeText}
											loading={pending}
											ariaLabel={
												canRetryCropDescription ? "重新搜尋品項" : "送出答案"
											}
											className="hawkr-agent-question-card__free-text-send"
										/>
									) : null}
								</form>
								{freeTextError ? (
									<p
										id={`clarify-free-text-error-${question.slotKey}`}
										className="hawkr-agent-question-card__input-error"
										role="alert"
									>
										{freeTextError}
									</p>
								) : null}
							</>
						) : null}

						{showCropOtherForm || isCropOtherOnly ? (
							<form
								className="hawkr-agent-question-card__crop-form"
								aria-label="自訂採購品項"
								onSubmit={(event) => {
									event.preventDefault();
									submitCropOther();
								}}
							>
								<Textarea
									ref={cropOtherDescriptionRef}
									id={cropOtherDescriptionId}
									label="品項描述"
									placeholder="例如：冷凍草莓，製作果醬用"
									variant="bordered"
									size="md"
									minRows={3}
									description={
										<span className="hawkr-agent-question-card__crop-description-meta">
											<span>可補充品種、規格或用途</span>
											<span aria-hidden="true">
												{cropOtherDescription.length} / 200
											</span>
										</span>
									}
									value={cropOtherDescription}
									onChange={(event) =>
										setCropOtherDescription(event.currentTarget.value)
									}
									disabled={pending || !onSubmitCropOther}
									maxLength={200}
								/>
								<div className="hawkr-agent-question-card__crop-actions">
									<Button
										type="submit"
										size="lg"
										disabled={!canSubmitCropOther || !onSubmitCropOther}
										isLoading={pending}
										loadingLabel="送出中…"
										endContent={
											<AppIcon
												name="arrow-right"
												size={18}
												aria-hidden="true"
											/>
										}
										className="hawkr-agent-question-card__crop-submit"
									>
										確認品項
									</Button>
								</div>
							</form>
						) : null}

						{hasUnitOptions && onPickUnit ? (
							<fieldset
								className="hawkr-agent-question-card__options hawkr-agent-question-card__options--units"
								aria-label="選擇單位"
							>
								{unitOptions.map((option, index) => (
									<button
										key={option.optionId}
										ref={index === 0 ? firstUnitOptionRef : undefined}
										type="button"
										disabled={pending || !quantityReady}
										onClick={() =>
											onPickUnit({
												slotKey: question.slotKey,
												unitOptionId: option.optionId,
												unitLabel: option.label,
												freeText: isCompoundQuantity
													? freeText.trim()
													: undefined,
											})
										}
										className="hawkr-agent-question-card__option hawkr-agent-question-card__option--unit"
									>
										{/* 單位標籤是一到三個字，不掛右側的圈選標記：城市那種
										 * 長標籤撐得住 space-between，「盒」撐不住——標籤貼左、
										 * 圈圈貼右，中間一大片空白。這裡改成置中的緊湊 chip。 */}
										{option.label}
									</button>
								))}
							</fieldset>
						) : null}

						{onSkip ? (
							<footer className="hawkr-agent-question-card__footer">
								<span>這題之後也可以再補充</span>
								<button
									type="button"
									disabled={pending}
									onClick={() =>
										onSkip({ slotKey: question.slotKey, skip: true })
									}
									className="hawkr-agent-question-card__skip"
								>
									跳過
								</button>
							</footer>
						) : null}
					</div>
				) : null}
			</section>
		</li>
	);
}
