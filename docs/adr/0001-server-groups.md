# Server Groups — Root-Level, Flat

Groups organize servers into a flat, root-level list. We chose a flat model (`groups` table with no `parent_id` self-reference; a nullable `group_id` foreign key on `servers`) over the earlier nestable adjacency-list design because the expected scale (5-20 servers) makes nesting unnecessary: every Group is a top-level container, and Group names are globally unique.

This supersedes the original decision (nestable tree with `parent_id`, orphan-up deletion, cycle detection). There is no version-to-version migration — users re-add their Servers manually.

The API returns Groups as a flat list; the frontend renders them as a flat expandable sidebar list and builds per-group member lists. This keeps the backend simple and lets the frontend control expand/collapse state natively.

**Deletion ungroups**: when a Group is deleted, its member Servers become ungrouped (`group_id` set to NULL). Cascade deletion was rejected because accidentally deleting a Group would wipe out all servers; the servers are preserved and simply lose their Group.

Groups are excluded from the WebSocket broadcast and fetched via REST. Group structure changes rarely (user-initiated create, rename, delete) compared to the 2-second metrics broadcast, so coupling them would waste bandwidth.

**Constraints**: Group names are globally unique (enforced at the store level). Groups are not movable. Server-to-Group is many-to-one (one Server belongs to at most one Group); Servers are moved between Groups via drag-and-drop or the "Move to..." dialog.

**Server is unchanged**: a `group_id` of NULL means the Server is ungrouped and renders directly at the root of the sidebar tree alongside the root-level Groups (there are no virtual "All Servers"/"Ungrouped" nodes). A Server can be ungrouped by dropping it on blank tree space.
