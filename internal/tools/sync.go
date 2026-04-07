package tools

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// SyncResult holds the result of syncing a single table.
type SyncResult struct {
	Table      string
	V2ToV3     int
	V3ToV2     int
	Errors     string
	V2Count    int64
	V3Count    int64
}

// tableInfo defines metadata for each syncable table.
type tableInfo struct {
	Name        string
	PK          string   // primary key column (single)
	CompositePK []string // for composite primary keys
}

// Tables to skip during sync (framework-managed, not user data).
var skipTables = map[string]bool{
	"casbin_rule":     true, // managed by Casbin enforcer in v3
	"auth_assignment": true, // Yii2 RBAC — replaced by Casbin in v3
	"auth_item":       true, // Yii2 RBAC
	"auth_item_child": true, // Yii2 RBAC
	"auth_rule":       true, // Yii2 RBAC
	"menu":            true, // Yii2 menu system — not used in v3
	"log":             true, // v2 request/response log — not used in v3
	"migration":       true, // Yii2 migration tracking
	"queue_log":       true, // v2 uses Yii2 console jobs, v3 uses Asynq — different systems
	"export":              true, // app-specific generated files — CSI vs TMS exports are different
	"export_result":       true, // app-specific export results
	"terminal":            true, // synced from TMS via Sinkronisasi CSI — not shared between v2/v3
	"terminal_parameter":  true, // child of terminal — same reason
}

// discoverSharedTables finds tables that exist in both databases and returns
// tableInfo with primary keys detected from INFORMATION_SCHEMA.
func discoverSharedTables(v2DB, v3DB *gorm.DB) ([]tableInfo, error) {
	v2Tables, err := listTables(v2DB)
	if err != nil {
		return nil, fmt.Errorf("list v2 tables: %w", err)
	}
	v3Tables, err := listTables(v3DB)
	if err != nil {
		return nil, fmt.Errorf("list v3 tables: %w", err)
	}

	// Find intersection.
	v2Set := make(map[string]bool, len(v2Tables))
	for _, t := range v2Tables {
		v2Set[t] = true
	}

	var tables []tableInfo
	for _, t := range v3Tables {
		if !v2Set[t] || skipTables[t] {
			continue
		}
		// Get primary key columns from v3.
		pks, err := getPrimaryKeys(v3DB, t)
		if err != nil || len(pks) == 0 {
			continue // skip tables without primary keys
		}
		ti := tableInfo{Name: t}
		if len(pks) == 1 {
			ti.PK = pks[0]
		} else {
			ti.CompositePK = pks
		}
		tables = append(tables, ti)
	}

	return tables, nil
}

func listTables(db *gorm.DB) ([]string, error) {
	var dbName string
	db.Raw("SELECT DATABASE()").Scan(&dbName)

	var tables []string
	err := db.Raw(
		"SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME",
		dbName,
	).Scan(&tables).Error
	return tables, err
}

func getPrimaryKeys(db *gorm.DB, table string) ([]string, error) {
	var dbName string
	db.Raw("SELECT DATABASE()").Scan(&dbName)

	var pks []string
	err := db.Raw(
		"SELECT COLUMN_NAME FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CONSTRAINT_NAME = 'PRIMARY' ORDER BY ORDINAL_POSITION",
		dbName, table,
	).Scan(&pks).Error
	return pks, err
}

// SyncDatabases performs bidirectional incremental sync between v2 and v3.
func SyncDatabases(v2DB, v3DB *gorm.DB) ([]SyncResult, error) {
	// Disable FK checks on both databases during sync.
	v2DB.Exec("SET FOREIGN_KEY_CHECKS = 0")
	v3DB.Exec("SET FOREIGN_KEY_CHECKS = 0")
	defer v2DB.Exec("SET FOREIGN_KEY_CHECKS = 1")
	defer v3DB.Exec("SET FOREIGN_KEY_CHECKS = 1")

	// Discover tables that exist in both databases.
	syncTables, err := discoverSharedTables(v2DB, v3DB)
	if err != nil {
		return nil, fmt.Errorf("discover tables: %w", err)
	}

	var results []SyncResult

	for _, t := range syncTables {
		result := syncTable(v2DB, v3DB, t)
		results = append(results, result)
		log.Info().
			Str("table", result.Table).
			Int("v2_to_v3", result.V2ToV3).
			Int("v3_to_v2", result.V3ToV2).
			Msg("table synced")
	}

	return results, nil
}

func syncTable(v2DB, v3DB *gorm.DB, t tableInfo) SyncResult {
	result := SyncResult{Table: t.Name}

	// Get row counts.
	v2DB.Raw(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", t.Name)).Scan(&result.V2Count)
	v3DB.Raw(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", t.Name)).Scan(&result.V3Count)

	// Get columns from BOTH databases.
	v2Cols, err := getColumns(v2DB, t.Name)
	if err != nil {
		result.Errors = fmt.Sprintf("get v2 columns: %v", err)
		return result
	}
	v3Cols, err := getColumns(v3DB, t.Name)
	if err != nil {
		result.Errors = fmt.Sprintf("get v3 columns: %v", err)
		return result
	}

	// Add missing columns to each side so schemas match.
	addMissingColumns(v2DB, v3DB, t.Name, v2Cols, v3Cols)
	addMissingColumns(v3DB, v2DB, t.Name, v3Cols, v2Cols)

	// Re-read columns from both sides and use only the intersection.
	v2Cols, _ = getColumns(v2DB, t.Name)
	v3Cols, _ = getColumns(v3DB, t.Name)
	columns := intersectColumns(v2Cols, v3Cols)
	if len(columns) == 0 {
		result.Errors = "no common columns"
		return result
	}

	// Relax NOT NULL constraints on both sides for shared columns
	// so rows with NULL values from one side can be inserted into the other.
	relaxNotNull(v2DB, t.Name, columns)
	relaxNotNull(v3DB, t.Name, columns)

	// Sync terminals before terminal_parameter (FK dependency).
	// This is handled by table ordering in syncTables list.

	// Sync v2 → v3 (copy rows from v2 that are missing or newer in v2).
	v2ToV3, err := syncDirection(v2DB, v3DB, t, columns)
	if err != nil {
		result.Errors = fmt.Sprintf("v2→v3: %v", err)
		return result
	}
	result.V2ToV3 = v2ToV3

	// Sync v3 → v2 (copy rows from v3 that are missing or newer in v3).
	v3ToV2, err := syncDirection(v3DB, v2DB, t, columns)
	if err != nil {
		result.Errors += fmt.Sprintf(" v3→v2: %v", err)
		return result
	}
	result.V3ToV2 = v3ToV2

	// Update final counts.
	v2DB.Raw(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", t.Name)).Scan(&result.V2Count)
	v3DB.Raw(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", t.Name)).Scan(&result.V3Count)

	return result
}

// syncDirection copies rows from srcDB to dstDB using INSERT ... ON DUPLICATE KEY UPDATE.
func syncDirection(srcDB, dstDB *gorm.DB, t tableInfo, columns []string) (int, error) {
	// Build explicit column list for SELECT.
	quotedCols := make([]string, len(columns))
	for i, c := range columns {
		quotedCols[i] = "`" + c + "`"
	}
	colList := strings.Join(quotedCols, ", ")

	rows, err := srcDB.Raw(fmt.Sprintf("SELECT %s FROM `%s`", colList, t.Name)).Rows()
	if err != nil {
		return 0, fmt.Errorf("query source: %w", err)
	}
	defer rows.Close()

	// Verify actual column count from result set matches our expectation.
	actualCols, err := rows.Columns()
	if err != nil {
		return 0, fmt.Errorf("get result columns: %w", err)
	}
	numCols := len(actualCols)

	// Build upsert using actual column count.
	placeholders := strings.Repeat("?,", numCols)
	placeholders = placeholders[:len(placeholders)-1]

	// Use actualCols for the INSERT column list to guarantee match.
	actualQuoted := make([]string, numCols)
	for i, c := range actualCols {
		actualQuoted[i] = "`" + c + "`"
	}
	actualColList := strings.Join(actualQuoted, ", ")

	// ON DUPLICATE KEY UPDATE all non-PK columns.
	var updateParts []string
	pkSet := make(map[string]bool)
	if t.PK != "" {
		pkSet[t.PK] = true
	}
	for _, pk := range t.CompositePK {
		pkSet[pk] = true
	}

	for _, c := range actualCols {
		if pkSet[c] {
			continue
		}
		updateParts = append(updateParts, fmt.Sprintf("`%s` = VALUES(`%s`)", c, c))
	}

	upsertSQL := fmt.Sprintf(
		"INSERT INTO `%s` (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		t.Name, actualColList, placeholders, strings.Join(updateParts, ", "),
	)

	if len(updateParts) == 0 {
		upsertSQL = fmt.Sprintf(
			"INSERT IGNORE INTO `%s` (%s) VALUES (%s)",
			t.Name, actualColList, placeholders,
		)
	}

	synced := 0
	for rows.Next() {
		values := make([]interface{}, numCols)
		valuePtrs := make([]interface{}, numCols)
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return synced, fmt.Errorf("scan row: %w", err)
		}

		res := dstDB.Exec(upsertSQL, values...)
		if res.Error != nil {
			return synced, fmt.Errorf("upsert row: %w", res.Error)
		}
		if res.RowsAffected > 0 {
			synced++
		}
	}

	return synced, nil
}

// getColumns returns column names for a table.
func getColumns(db *gorm.DB, table string) ([]string, error) {
	rows, err := db.Raw(fmt.Sprintf("SELECT * FROM `%s` LIMIT 0", table)).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rows.Columns()
}

// addMissingColumns checks which columns in srcCols are missing from dstCols,
// reads their definitions from srcDB, and creates them in dstDB.
func addMissingColumns(srcDB, dstDB *gorm.DB, table string, srcCols, dstCols []string) {
	dstSet := make(map[string]bool, len(dstCols))
	for _, c := range dstCols {
		dstSet[c] = true
	}

	var missing []string
	for _, c := range srcCols {
		if !dstSet[c] {
			missing = append(missing, c)
		}
	}
	if len(missing) == 0 {
		return
	}

	var srcDBName string
	srcDB.Raw("SELECT DATABASE()").Scan(&srcDBName)

	for _, col := range missing {
		// Use raw row scan to avoid GORM struct mapping issues with INFORMATION_SCHEMA.
		var colType, isNullable string
		row := srcDB.Raw(
			"SELECT COLUMN_TYPE, IS_NULLABLE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?",
			srcDBName, table, col,
		).Row()
		if row == nil {
			continue
		}
		if err := row.Scan(&colType, &isNullable); err != nil {
			continue
		}

		// Always allow NULL for added columns — existing rows won't have values.
		alterSQL := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s NULL", table, col, colType)
		if err := dstDB.Exec(alterSQL).Error; err != nil {
			if !strings.Contains(err.Error(), "1060") {
				log.Warn().Err(err).Str("table", table).Str("column", col).Msg("failed to add missing column")
			}
		} else {
			log.Info().Str("table", table).Str("column", col).Msg("added missing column")
		}
	}
}

// relaxNotNull changes NOT NULL columns (except PKs) to allow NULL so inserts from the other DB won't fail.
func relaxNotNull(db *gorm.DB, table string, columns []string) {
	var dbName string
	db.Raw("SELECT DATABASE()").Scan(&dbName)

	for _, col := range columns {
		var colType, isNullable, colKey string
		row := db.Raw(
			"SELECT COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?",
			dbName, table, col,
		).Row()
		if row == nil {
			continue
		}
		if err := row.Scan(&colType, &isNullable, &colKey); err != nil {
			continue
		}
		// Skip PKs and columns already nullable.
		if colKey == "PRI" || isNullable == "YES" {
			continue
		}
		alterSQL := fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` %s NULL", table, col, colType)
		db.Exec(alterSQL)
	}
}

// intersectColumns returns columns that exist in both slices, preserving order from a.
func intersectColumns(a, b []string) []string {
	bSet := make(map[string]bool, len(b))
	for _, c := range b {
		bSet[c] = true
	}
	var result []string
	for _, c := range a {
		if bSet[c] {
			result = append(result, c)
		}
	}
	return result
}
