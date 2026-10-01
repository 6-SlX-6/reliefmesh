# Accessibility

ReliefMesh is used under stress, on small screens, outdoors, with gloves, by
people with and without disabilities and with varying language skills. The
interface targets **WCAG 2.2 level AA**.

## Design rules

- **Status is never color alone.** Badges combine text, an icon and color;
  screen readers hear "Status: In progress" / "Urgency: Critical".
- **High contrast.** Design tokens in `packages/ui-tokens` are tested for at
  least 4.5:1 contrast (primary text 7:1).
- **Large targets.** Interactive controls are at least 48 px high; radio and
  checkbox choices are full-width cards.
- **Plain language.** Short labels, status explanations ("Checked by a
  coordinator. Waiting for someone to be assigned."), no jargon.
- **System fonts** at a base size of 17 px; respects browser zoom and text
  scaling; layout reflows down to 320 px width.
- **Reduced motion** is honored (`prefers-reduced-motion`).
- **Calm confirmations.** Destructive or important actions open a dialog that
  states the consequence; reasons are requested where they matter.

## Keyboard and screen readers

- "Skip to main content" link as first focusable element.
- After navigation, focus moves to the page heading.
- All form controls have visible labels; help and error texts are connected
  with `aria-describedby`; invalid fields use `aria-invalid`.
- Dialogs use the native `<dialog>` element (focus trap, Escape to close).
- Connection and sync changes are announced via a polite live region; errors
  use `role="alert"`.
- Navigation landmarks: header, main navigation (sidebar on desktop, bottom bar
  on mobile), main, emergency notice (`aside`).

## Testing

- `apps/web/e2e/accessibility.spec.ts` runs axe-core (WCAG 2.0/2.1 A and AA
  rules) on the sign-in page and the main coordinator pages and fails on
  serious or critical violations; it also checks the skip link.
- Component tests check text alternatives for status and urgency badges.
- Manual checks before releases: keyboard-only walkthrough, screen reader
  (NVDA/VoiceOver/TalkBack) walkthrough of request creation and task
  completion, 200% zoom, mobile in bright light.

## Known limitations

- The interface is English-only in v0.1.0; labels are centralized in
  `apps/web/utils/labels.ts` to prepare translations (see roadmap).
- Coordinates must be typed or taken from the device; there is no map picker
  (by design, no external map tiles).
