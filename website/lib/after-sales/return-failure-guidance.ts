import { ShippingMethod } from "@/lib/codemeta/enums";
import {
	type ShipmentErrorGuidance,
	shipmentErrorGuidance,
	TCAT_BRANCH_LOOKUP_URL,
} from "@/lib/seller/shipment";

interface ReturnFailureGuidanceInput {
	method?: string;
	providerMessage?: string;
	providerStatus?: string;
}

/**
 * Turn return-logistics failures into seller-facing guidance. This mirrors the
 * seller shipment failure pattern: show cause, next steps, and useful links
 * instead of only exposing a provider message.
 */
export function returnFailureGuidance(
	input: ReturnFailureGuidanceInput,
): ShipmentErrorGuidance {
	const message = input.providerMessage || input.providerStatus || "";
	const shipmentGuidance = message ? shipmentErrorGuidance({ message }) : null;
	if (shipmentGuidance) return shipmentGuidance;

	if (input.method === ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP) {
		return {
			cause:
				"退貨物流沒有成功完成，常見原因包含寄件代碼逾期、門市端退回、物流狀態異常，或退貨代碼需要由平台重新安排。",
			steps: [
				"先核對退貨單號、三段碼與 PayUni 回傳原因，避免誤判為已收到退貨。",
				"請買家保留商品與照片佐證，不要自行改寄其他物流；退貨方式仍需維持同原訂單物流。",
				"聯絡客服協助重建退貨代碼或重新安排同原物流的退貨流程。",
			],
			links: [],
		};
	}

	if (input.method === ShippingMethod.TAQBIN) {
		return {
			cause:
				"黑貓退貨物流未成功完成，可能是取件或派送失敗、地址或時段問題，或物流商回報異常。",
			steps: [
				"先核對退貨單號與物流商回傳原因，確認包裹是否已退回或仍在物流商端。",
				"聯絡客服協助確認是否需要重新安排黑貓退貨；退貨方式仍需維持同原訂單物流。",
				"若需要與站所確認取件或配送狀況，可查詢鄰近黑貓營業所。",
			],
			links: [{ label: "查詢黑貓營業所", href: TCAT_BRANCH_LOOKUP_URL }],
		};
	}

	return {
		cause: "退貨物流沒有成功完成，目前不能直接當作已收退貨或結案處理。",
		steps: [
			"先核對退貨單號與物流商回傳原因，確認包裹實際位置。",
			"透過客服協助重新安排退貨流程，並維持與原訂單相同的物流方式。",
			"在物流重新安排前，請勿手動改判為已收退貨；避免退款與包裹狀態不一致。",
		],
		links: [],
	};
}
