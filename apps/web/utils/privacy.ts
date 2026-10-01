// Client-side privacy hints. These are a convenience for the person typing;
// the server enforces the actual rules (e.g. no free text for medicine pickup).

const medicalPatterns: RegExp[] = [
  /\b\d+\s?(mg|mcg|µg|ml|iu|ie)\b/i,
  /\bprescri(ption|bed)\b/i,
  /\brezept\b/i,
  /\bdiagnos(is|ed|e)\b/i,
  /\b(insulin|metformin|ibuprofen|paracetamol|morphin|methadon|antibiotic|antidepress)\w*/i,
  /\b(diabet|epilep|cancer|hiv|dement|schizo|depress|pregnan)\w*/i,
]

/** Returns true if text looks like it may contain medical details. */
export function looksMedical(text: string): boolean {
  return medicalPatterns.some((re) => re.test(text))
}

const contactPatterns: RegExp[] = [
  /\+?\d[\d\s/-]{7,}\d/, // phone-like digit runs
  /[\w.+-]+@[\w-]+\.[\w.]+/, // e-mail
]

/** Returns true if text looks like it contains contact details. */
export function looksLikeContact(text: string): boolean {
  return contactPatterns.some((re) => re.test(text))
}
