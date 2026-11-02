export type ContactDetails = {
  readonly recipient: string;
  readonly phone: string;
  readonly city: string;
  readonly address: string;
  readonly postalCode: string;
};

export type ContactField = keyof ContactDetails;
export type ContactErrors = Partial<Record<ContactField, string>>;

export const emptyContact: ContactDetails = {
  recipient: "",
  phone: "",
  city: "",
  address: "",
  postalCode: "",
};

export const sampleContact: ContactDetails = {
  recipient: "مشتری نمونه",
  phone: "۰۹۱۲۳۴۵۶۷۸۹",
  city: "تهران",
  address: "خیابان نمونه، کوچهٔ آزمایشی، پلاک ۱۲، واحد ۳",
  postalCode: "۱۲۳۴۵۶۷۸۹۰",
};

export function normalizeDigits(value: string): string {
  return value.replace(/[۰-۹٠-٩]/g, (digit) => {
    const code = digit.charCodeAt(0);
    return String(code >= 0x6f0 ? code - 0x6f0 : code - 0x660);
  });
}

type ContactResult =
  | { readonly kind: "valid"; readonly details: ContactDetails }
  | {
      readonly kind: "invalid";
      readonly errors: ContactErrors;
      readonly firstField: ContactField;
    };

export function validateContact(draft: ContactDetails): ContactResult {
  const details: ContactDetails = {
    recipient: draft.recipient.trim(),
    phone: normalizeDigits(draft.phone).trim(),
    city: draft.city.trim(),
    address: draft.address.trim(),
    postalCode: normalizeDigits(draft.postalCode).trim(),
  };
  const errors: ContactErrors = {};
  if (!details.recipient) errors.recipient = "نام گیرنده را وارد کن.";
  if (!/^09\d{9}$/.test(details.phone)) {
    errors.phone = "شمارهٔ همراه ۱۱ رقمی با ۰۹ شروع می‌شود.";
  }
  if (!details.city) errors.city = "نام شهر را وارد کن.";
  if (!details.address) errors.address = "نشانی کامل گیرنده را وارد کن.";
  if (!/^\d{10}$/.test(details.postalCode)) {
    errors.postalCode = "کد پستی باید ۱۰ رقم باشد.";
  }
  const fields: readonly ContactField[] = [
    "recipient",
    "phone",
    "city",
    "address",
    "postalCode",
  ];
  const firstField = fields.find((field) => errors[field]);
  return firstField
    ? { kind: "invalid", errors, firstField }
    : { kind: "valid", details };
}
