export type ShippingMethod = {
  id: string;
  name: string;
  fee_toman: number;
  active: boolean;
  version: number;
};
export function parseShippingMethods(value: unknown): ShippingMethod[] {
  if (
    typeof value !== "object" ||
    value === null ||
    !("methods" in value) ||
    !Array.isArray(value.methods) ||
    !value.methods.every(
      (item) =>
        typeof item === "object" &&
        item !== null &&
        typeof item.id === "string" &&
        typeof item.name === "string" &&
        typeof item.fee_toman === "number" &&
        Number.isSafeInteger(item.fee_toman) &&
        item.fee_toman >= 0 &&
        typeof item.active === "boolean" &&
        typeof item.version === "number" &&
        Number.isSafeInteger(item.version) &&
        item.version >= 1,
    )
  )
    throw new Error("Invalid shipping response");
  return value.methods;
}
