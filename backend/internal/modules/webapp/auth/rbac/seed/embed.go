// Package seed nhúng artifact permission (sinh bởi FE codegen `perm-codegen.mjs`) để
// startup seeder upsert/prune rbac_auth_item — thay cơ chế Sync-from-FE thủ công.
// Artifact được đồng bộ từ FE repo (BMAD perm-codegen-seeder, Option A/A1).
package seed

import _ "embed"

//go:embed permissions.gen.json
var PermissionsJSON []byte

//go:embed route_perm_map.json
var RoutePermMapJSON []byte
