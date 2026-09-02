// SPDX-License-Identifier: MPL-2.0

package sqlite

import "embed"

//go:embed migrations/*.sql
var migrationFS embed.FS
