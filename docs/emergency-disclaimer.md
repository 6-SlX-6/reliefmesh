# Emergency disclaimer and product boundaries

## The notice

Every ReliefMesh screen - including the sign-in page and offline screens -
shows an emergency notice. The default English text is:

> Emergency notice: If there is immediate danger, contact your local emergency
> services. In the EU, call 112 where available. ReliefMesh is not an emergency
> dispatch service.

Emergency numbers differ by country, so administrators **must** review and
adapt the wording under *Administration > Settings & notices* (for example
"call 112 or 110" in Germany, "call 911" in the United States, "call 999" in
the United Kingdom). The notice must keep stating that ReliefMesh is not an
emergency service. The web app also stores the last known notice on each
device and falls back to the EU default if the server has never been reached.

Before a **critical** request is submitted, a second, configurable notice is
shown and must be acknowledged; the API rejects critical requests without
this acknowledgement. The dialog states explicitly that submitting does not
notify emergency services or authorities.

## What ReliefMesh never does

- It never claims that authorities or emergency services received a request.
- It never suggests delaying a call to emergency services.
- It never dispatches, assigns, routes or recommends emergency responders.
  All assignments are made manually by coordinators of the operating
  organization.
- It never performs medical triage or gives medical advice. Medicine pickup
  requests are logistics only and reject medical free text.
- It never determines urgency automatically and never prioritizes on the basis
  of demographic or health data.
- It never provides turn-by-turn routing, never recommends routes and never
  claims that a destination is safe.
- It never requires photos, biometrics or identity documents as evidence.
- It never exposes personal data publicly by default: no public maps, no public
  posting, no social sharing.

## Intended use

Authorized preparedness exercises, local aid coordination and non-emergency
community support by an organization that operates its own instance and is
responsible for its use.

## For operators

- Make the boundaries clear in training and on printed material at intake
  points (shelter desk, community hall).
- Keep the emergency notice correct for your location.
- Use exercise mode for exercises.
- Do not connect ReliefMesh to emergency dispatch systems.
