import { ShippingTemperature } from "@/lib/codemeta/enums";

// Java still requires a temperature on seller-side writes; removed in plan B4.
export const AMBIENT_TEMPERATURE_CODE = ShippingTemperature.AMBIENT;
