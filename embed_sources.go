// Package shipq provides embedded source files for the shipq library packages.
// These are used by code generation to embed library code into generated projects,
// making them fully self-contained without depending on the shipq module.
package shipq

import "embed"

// Category A: packages imported by "final" generated code (handlers, server, etc.)

//go:embed handler/*.go
var HandlerFS embed.FS

//go:embed httperror/*.go
var HttperrorFS embed.FS

//go:embed httpserver/*.go
var HttpserverFS embed.FS

//go:embed logging/*.go
var LoggingFS embed.FS

//go:embed crypto/*.go
var CryptoFS embed.FS

//go:embed nanoid/*.go
var NanoidFS embed.FS

//go:embed httputil/*.go
var HttputilFS embed.FS

//go:embed filestorage/*.go
var FilestorageFS embed.FS

//go:embed channel/*.go
var ChannelFS embed.FS

// Category B: packages imported by temporary compile programs

//go:embed db/portsql/query/*.go
var QueryFS embed.FS

//go:embed db/portsql/query/compile/*.go
var QueryCompileFS embed.FS

//go:embed db/portsql/migrate/*.go
var MigrateFS embed.FS

//go:embed db/portsql/ddl/*.go
var DdlFS embed.FS

//go:embed db/portsql/ref/*.go
var RefFS embed.FS

//go:embed proptest/*.go
var ProptestFS embed.FS

// Category C: static assets (JS, CSS) for development tooling

//go:embed assets/*
var AssetsFS embed.FS

// Category D: TypeScript source files shared between admin panel and codegen

//go:embed admin/src/shipq-files.ts
var ShipqFilesTS []byte
