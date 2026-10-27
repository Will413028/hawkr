"use client";

import { countyCodeOfTownCode } from "@hawkr/twaddr/lookup";
import { zodResolver } from "@hookform/resolvers/zod";
import { useSearchParams } from "next/navigation";
import * as React from "react";
import { Controller, useForm } from "react-hook-form";
import { BackLink } from "@/components/shared/BackLink";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Input } from "@/components/ui";
import Button, { buttonClasses } from "@/components/ui/Button";
import {
	type AddressFormValues,
	addressFormSchema,
} from "@/lib/addresses/schema";
import { safeRedirectPath } from "@/lib/auth/redirect";
import { toastActionResult } from "@/lib/feedback/action-toast";
import {
	formActionFailure,
	formActionSuccess,
} from "@/lib/forms/action-result";
import { useAddresses } from "@/lib/hooks/useAddresses";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { TownFields } from "./TownFields";

interface AddressFormClientProps {
	mode: "create" | "edit";
	initialId?: string;
}

function normalizePhone(value: string): string {
	return value.replace(/[^\d]/g, "");
}

export default function AddressFormClient({
	mode,
	initialId,
}: AddressFormClientProps) {
	return (
		<React.Suspense fallback={null}>
			<Inner mode={mode} initialId={initialId} />
		</React.Suspense>
	);
}

function Inner({ mode, initialId }: AddressFormClientProps) {
	const router = useObservedRouter("address_form");
	const search = useSearchParams();
	const redirect =
		safeRedirectPath(search?.get("redirect")) || "/account/addresses";
	const {
		addresses,
		create,
		update,
		loading: addressesLoading,
		error: addressesError,
	} = useAddresses();
	const editing =
		mode === "edit" ? addresses.find((a) => a.id === initialId) : undefined;
	const editRecordMissing = mode === "edit" && !addressesLoading && !editing;
	const form = useForm<AddressFormValues>({
		resolver: zodResolver(addressFormSchema),
		mode: "onChange",
		defaultValues: {
			receiverName: editing?.receiverName ?? "",
			receiverPhone: editing?.receiverPhone ?? "",
			countyCode: editing ? countyCodeOfTownCode(editing.townCode) : "",
			townCode: editing?.townCode ?? "",
			detail: editing?.addressDetail ?? "",
			isDefault: editing?.isDefault ?? false,
		},
	});
	const {
		control,
		formState: { errors, isSubmitting, submitCount, touchedFields },
		handleSubmit,
		register,
		reset,
		setValue,
		watch,
	} = form;
	const [countyCode, townCode] = watch(["countyCode", "townCode"]);
	const [formError, setFormError] = React.useState<string | null>(null);

	// Hydrate when the edited record loads
	React.useEffect(() => {
		if (mode !== "edit" || !editing) return;
		reset({
			receiverName: editing.receiverName,
			receiverPhone: editing.receiverPhone,
			countyCode: countyCodeOfTownCode(editing.townCode),
			townCode: editing.townCode,
			detail: editing.addressDetail,
			isDefault: editing.isDefault,
		});
	}, [editing, mode, reset]);

	const errorFor = (field: keyof AddressFormValues) =>
		touchedFields[field] || submitCount > 0
			? errors[field]?.message
			: undefined;

	const submitAddress = handleSubmit(async (values) => {
		if (editRecordMissing) {
			setFormError("找不到要編輯的地址，請返回地址列表後重新進入");
			return;
		}
		if (mode === "edit" && !editing) return;
		setFormError(null);
		try {
			const body = {
				receiverName: values.receiverName.trim(),
				receiverPhone: normalizePhone(values.receiverPhone),
				townCode: values.townCode,
				addressDetail: values.detail.trim(),
				isDefault: values.isDefault,
			};
			if (editing) {
				await update(editing.id, editing.etag, body);
			} else {
				await create(body);
			}
			toastActionResult(formActionSuccess(undefined, "地址已儲存"));
			router.push(redirect);
		} catch (e2) {
			const message = e2 instanceof Error ? e2.message : "儲存失敗";
			setFormError(message);
			toastActionResult(formActionFailure(message));
		}
	});

	const bannerError =
		formError ??
		addressesError ??
		(editRecordMissing ? "找不到要編輯的地址，請返回地址列表後重新進入" : null);

	return (
		<main className="addresses-page">
			<SellerPageShell
				title={mode === "edit" ? "編輯地址" : "新增地址"}
				actions={
					<BackLink
						fallbackHref="/account/addresses"
						className={buttonClasses({
							variant: "bordered",
							color: "default",
							size: "sm",
						})}
					>
						← 返回
					</BackLink>
				}
			>
				<section className="seller-card">
					<form
						className="flex flex-col gap-5"
						onSubmit={submitAddress}
						autoComplete="on"
					>
						<Input
							floatingLabel
							variant="bordered"
							size="lg"
							label="收件人姓名"
							isRequired
							maxLength={30}
							autoComplete="shipping name"
							errorMessage={errorFor("receiverName")}
							isInvalid={Boolean(errorFor("receiverName"))}
							{...register("receiverName")}
						/>

						<Input
							floatingLabel
							variant="bordered"
							size="lg"
							label="聯絡電話"
							type="tel"
							isRequired
							placeholder="09xx-xxx-xxx"
							autoComplete="shipping tel-national"
							errorMessage={errorFor("receiverPhone")}
							isInvalid={Boolean(errorFor("receiverPhone"))}
							{...register("receiverPhone")}
						/>

						<div className="grid grid-cols-2 gap-3 max-[600px]:grid-cols-1">
							<Controller
								control={control}
								name="countyCode"
								render={() => (
									<TownFields
										countyCode={countyCode}
										townCode={townCode}
										onChange={(nextCounty, nextTown) => {
											setValue("countyCode", nextCounty, {
												shouldDirty: true,
												shouldTouch: true,
												shouldValidate: true,
											});
											setValue("townCode", nextTown, {
												shouldDirty: true,
												shouldTouch: Boolean(nextTown),
												shouldValidate: true,
											});
										}}
										countyError={errorFor("countyCode")}
										townError={errorFor("townCode")}
									/>
								)}
							/>
						</div>

						<div>
							<Input
								floatingLabel
								variant="bordered"
								size="lg"
								label="詳細地址"
								isRequired
								placeholder="信義路一段 1 號 5 樓"
								maxLength={255}
								autoComplete="shipping street-address"
								errorMessage={errorFor("detail")}
								isInvalid={Boolean(errorFor("detail"))}
								{...register("detail")}
							/>
						</div>

						<label className="inline-flex items-center gap-2 text-sm text-foreground-muted cursor-pointer select-none">
							<input
								type="checkbox"
								className="w-[18px] h-[18px] accent-hkui-primary cursor-pointer"
								{...register("isDefault")}
							/>
							<span>設為預設地址</span>
						</label>

						{bannerError && (
							<p
								className="my-4 p-[12px_16px] rounded-hkui-md bg-danger-50 text-danger-700 text-sm"
								role="alert"
							>
								{bannerError}
							</p>
						)}

						<div className="flex gap-2 justify-end mt-3">
							<Button
								href="/account/addresses"
								variant="bordered"
								color="default"
							>
								取消
							</Button>
							<Button
								type="submit"
								disabled={isSubmitting || addressesLoading || editRecordMissing}
								isLoading={isSubmitting}
								loadingLabel="儲存地址中…"
							>
								儲存地址
							</Button>
						</div>
					</form>
				</section>
			</SellerPageShell>
		</main>
	);
}
