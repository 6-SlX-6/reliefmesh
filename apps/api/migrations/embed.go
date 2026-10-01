// Package migrations embeds the ordered SQL schema migrations so the API
// binary can migrate its database without shipping loose files.
package migrations

import "embed"

// FS holds every *.sql migration. Files are applied in lexical order.
//
//go:embed *.sql
var FS embed.FS
