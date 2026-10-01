# Roles and permissions

Users can hold several roles; permissions are the union of the roles'
capabilities. All checks happen on the server (`internal/roles`, and in every
service method). The web app only hides what the API would reject.

## Capabilities by role

| Capability | Requester | Volunteer | Coordinator | Org. manager | Admin |
| --- | :-: | :-: | :-: | :-: | :-: |
| Create requests | yes | yes | yes | yes | |
| See own requests | yes | yes | yes | yes | |
| See requests linked to own assignments (reduced view) | | yes | | | |
| See all requests of the organization | | | yes | yes | |
| Review, verify, prioritize, change status, reopen | | | yes | yes | |
| Internal notes | | | yes | yes | |
| Reveal protected request data | own | with grant, active assignment | yes | yes | |
| Create offers | | yes | yes | yes | |
| See / manage all offers | | | yes | yes | |
| Create assignments, grant access | | | yes | yes | |
| Update own assignments, handover | | yes | | | |
| Volunteer list | | | yes | yes | |
| Set other volunteers' availability | | | | yes | |
| Export non-personal reports | | | | yes | |
| Dashboard with queues | | | yes | yes | aggregates only |
| Users, roles, password resets | | | | | yes |
| Settings, notices, categories | | | | | yes |
| System audit log, integrity check | | | | | yes |
| Retention, record deletion, demo data | | | | | yes |

## Notes

- **Administration is separated from coordination.** An administrator without
  the coordinator role cannot list or open requests or offers (403/404), sees
  only aggregate dashboard numbers and works with references, not content,
  when deleting records. Small teams can add the coordinator role
  deliberately (`bootstrap-admin --also-coordinator` or the user editor).
- **Volunteers do not browse.** Only requests and offers linked to their own
  assignments are visible, in a reduced projection: no requester identity,
  review internals, tags or resolution notes. Protected data requires an
  active assignment (accepted / in progress / partially delivered), a
  coordinator grant (unless the policy is relaxed) and, for contact details,
  the requester's consent (`contact_visibility = assigned_responders`).
- **Requesters see outcomes, not staff.** Timelines show role labels
  ("Coordinator", "Volunteer") instead of names, and show when protected data
  was opened.
- **Invisible records return 404.**
- **Safeguards for administrators:** an admin cannot remove their own admin
  role or deactivate themselves, and the last active administrator cannot be
  removed. Role changes and deactivation end all sessions of the user.

## Tests

`apps/api/tests` contains integration tests for allowed and denied access,
for example `TestRequestVisibilityByRole`, `TestStatusTransitionRules`,
`TestAllocationReleasedAndRequestFlow`, `TestOfferPermissions`,
`TestAdminSafeguards`, `TestSyncPullScopesData` and
`TestExportContainsNoPersonalData`.
