package main

import (
	"database/sql"
	"fmt"
	"os"
	"text/tabwriter"
)

// TableCount holds row count comparison data for a single table between the
// v2 (veristoretools2) and v3 (veristoretools3) databases.
type TableCount struct {
	Name    string
	V2Count int64
	V3Count int64
	Match   bool
}

// tables lists every table that should exist in both databases.
var tables = []string{
	"user",
	"terminal",
	"terminal_parameter",
	"verification_report",
	"sync_terminal",
	"tms_login",
	"tms_report",
	"app_activation",
	"app_credential",
	"activity_log",
	"technician",
	"template_parameter",
	"queue_log",
	"export",
	"export_result",
	"tid_note",
	"session",
	"hash",
}

// verifyMigration connects to both v2 and v3 databases and compares row
// counts for every table in the tables list. Returns a slice of TableCount
// results or an error if either connection fails.
func verifyMigration(v2DSN, v3DSN string) ([]TableCount, error) {
	v2DB, err := sql.Open("mysql", v2DSN)
	if err != nil {
		return nil, fmt.Errorf("connect to v2 database: %w", err)
	}
	defer v2DB.Close()

	if err := v2DB.Ping(); err != nil {
		return nil, fmt.Errorf("ping v2 database: %w", err)
	}

	v3DB, err := sql.Open("mysql", v3DSN)
	if err != nil {
		return nil, fmt.Errorf("connect to v3 database: %w", err)
	}
	defer v3DB.Close()

	if err := v3DB.Ping(); err != nil {
		return nil, fmt.Errorf("ping v3 database: %w", err)
	}

	var results []TableCount
	for _, table := range tables {
		tc := TableCount{Name: table}

		tc.V2Count, err = countRows(v2DB, table)
		if err != nil {
			// Table may not exist in v2 -- record as -1.
			tc.V2Count = -1
		}

		tc.V3Count, err = countRows(v3DB, table)
		if err != nil {
			// Table may not exist in v3 -- record as -1.
			tc.V3Count = -1
		}

		tc.Match = tc.V2Count == tc.V3Count && tc.V2Count >= 0
		results = append(results, tc)
	}

	return results, nil
}

// countRows returns the number of rows in the given table.
func countRows(db *sql.DB, table string) (int64, error) {
	// Use backticks to safely quote the table name.
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", table)
	var count int64
	if err := db.QueryRow(query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// printVerifyResults writes the verification results in a nicely formatted
// table to stdout.
func printVerifyResults(results []TableCount) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TABLE\tV2 COUNT\tV3 COUNT\tMATCH")
	fmt.Fprintln(w, "-----\t--------\t--------\t-----")

	allMatch := true
	for _, tc := range results {
		v2Str := fmt.Sprintf("%d", tc.V2Count)
		v3Str := fmt.Sprintf("%d", tc.V3Count)
		if tc.V2Count < 0 {
			v2Str = "N/A"
		}
		if tc.V3Count < 0 {
			v3Str = "N/A"
		}

		matchStr := "OK"
		if !tc.Match {
			matchStr = "MISMATCH"
			allMatch = false
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", tc.Name, v2Str, v3Str, matchStr)
	}

	w.Flush()

	fmt.Println()
	if allMatch {
		fmt.Println("All table row counts match.")
	} else {
		fmt.Println("WARNING: Some tables have mismatched row counts.")
	}
}

// runVerify is the entry point for the "verify" subcommand. It reads
// database config, builds DSNs for both v2 and v3 databases, runs the
// comparison, and prints the results.
func runVerify(v2DSN, v3DSN string) error {
	results, err := verifyMigration(v2DSN, v3DSN)
	if err != nil {
		return err
	}

	printVerifyResults(results)
	return nil
}
