package lib

import (
	"database/sql"
	"log"
)

func CreateServersTable(db *sql.DB) (sql.Result, error) {
	log.Println("CreateServersTable")

	createTableStatement := `
	CREATE TABLE IF NOT EXISTS servers (
		id INTEGER NOT NULL PRIMARY KEY,
		guid TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		description TEXT NOT NULL,
		emu TEXT NOT NULL,
		host TEXT NOT NULL,
		port TEXT NOT NULL,
		type TEXT NOT NULL,
		status TEXT,
		website_url TEXT,
		discord_url TEXT,
		is_listed INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS servers_server_id ON servers (id);
	CREATE INDEX IF NOT EXISTS servers_server_name ON servers (name);
	`

	return db.Exec(createTableStatement)
}

func CreateStatusesTable(db *sql.DB) (sql.Result, error) {
	log.Println("CreateStatusesTable")

	createTableStatement := `
	CREATE TABLE IF NOT EXISTS statuses (
		id INTEGER not null primary key NOT NULL,
		server_id INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		status INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS statuses_server_id ON statuses (server_id);
	`

	return db.Exec(createTableStatement)
}

func CreateLogsTable(db *sql.DB) (sql.Result, error) {
	log.Println("CreateLogsTable")

	createTableStatement := `
	CREATE TABLE IF NOT EXISTS logs (
		id INTEGER NOT NULL PRIMARY KEY,
		message TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);
	`

	return db.Exec(createTableStatement)
}

func AlterStatusesAddRTTAndMessage(db *sql.DB) (sql.Result, error) {
	log.Println("AlterStatusesAddRTTAndMessage")

	checkIfColExistsStatement := `
	SELECT message
	FROM statuses
	LIMIT 1;
	`

	checkRes, checkErr := db.Exec(checkIfColExistsStatement)

	if checkErr == nil {
		log.Print("Skipping migration")

		return checkRes, checkErr
	}

	alterTableStatement := `
	ALTER TABLE statuses
	ADD COLUMN rtt INTEGER;
	ALTER TABLE statuses
	ADD COLUMN message TEXT;
	`

	log.Printf("Running %s", alterTableStatement)

	alterRes, alterErr := db.Exec(alterTableStatement)

	if alterErr != nil {
		log.Fatal(alterErr)
	}

	return alterRes, alterErr
}

func CreateStatusesCreatedAtIndex(db *sql.DB) (sql.Result, error) {
	log.Println("CreateStatusesCreatedAtIndex")

	createIndexStatement := `
	CREATE INDEX IF NOT EXISTS statuses_date ON statuses (date(created_at, 'unixepoch'));
	`

	return db.Exec(createIndexStatement)
}

func AutoMigrate(db *sql.DB) error {
	log.Println("AutoMigrating...")

	var err error

	_, err = CreateServersTable(db)

	if err != nil {
		return err
	}

	_, err = CreateStatusesTable(db)

	if err != nil {
		return err
	}

	_, err = CreateLogsTable(db)

	if err != nil {
		return err
	}

	_, err = AlterStatusesAddRTTAndMessage(db)

	if err != nil {
		return err
	}

	_, err = CreateStatusesCreatedAtIndex(db)

	if err != nil {
		return err
	}

	log.Println("...AutoMigration Done")

	return nil
}
