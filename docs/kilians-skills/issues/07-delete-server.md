## What to build

Add the ability to delete a server. The UI shows a confirmation step before deletion. The backend removes the server from the database. When later slices add the collector and SSH pool, their cleanup hooks will extend this flow — for now, the delete handles the data layer.

The delete endpoint and UI should handle the case where the server doesn't exist (404) gracefully.

## Acceptance criteria

- [ ] Delete button/action accessible from server card or management UI
- [ ] Confirmation dialog before deletion ("Are you sure?" with server name)
- [ ] `DELETE /api/servers/{id}` called on confirmation
- [ ] Server removed from local state immediately on success
- [ ] 404 handled gracefully if server already deleted
- [ ] Sonner toast on success/failure

## Blocked by

- #5 Add Server UI
