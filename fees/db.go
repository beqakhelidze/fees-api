package fees

import "encore.dev/storage/sqldb"

var db = sqldb.NewDatabase("pave_db", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})
