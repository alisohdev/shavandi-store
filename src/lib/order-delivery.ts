export type OrderDelivery = {
  shipping_method_id?: string | null;
  shipping_version?: number | null;
  shipping_name?: string | null;
  shipping_fee_toman?: number | null;
  recipient: string;
  phone: string;
  province: string;
  city: string;
  address: string;
  postal_code: string;
  version: number;
};
export function isOrderDelivery(value: unknown): value is OrderDelivery {
  if (typeof value !== "object" || value === null) return false;
  const shipping =
    !("shipping_fee_toman" in value) ||
    value.shipping_fee_toman === null ||
    (typeof value.shipping_fee_toman === "number" &&
      Number.isSafeInteger(value.shipping_fee_toman) &&
      value.shipping_fee_toman >= 0 &&
      value.shipping_fee_toman <= 1000000000 &&
      "shipping_name" in value &&
      typeof value.shipping_name === "string" &&
      "shipping_method_id" in value &&
      typeof value.shipping_method_id === "string" &&
      "shipping_version" in value &&
      typeof value.shipping_version === "number" &&
      Number.isSafeInteger(value.shipping_version));
  return (
    shipping &&
    "recipient" in value &&
    typeof value.recipient === "string" &&
    "phone" in value &&
    typeof value.phone === "string" &&
    "province" in value &&
    typeof value.province === "string" &&
    "city" in value &&
    typeof value.city === "string" &&
    "address" in value &&
    typeof value.address === "string" &&
    "postal_code" in value &&
    typeof value.postal_code === "string" &&
    "version" in value &&
    typeof value.version === "number" &&
    Number.isSafeInteger(value.version) &&
    value.version >= 1
  );
}
