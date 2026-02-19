package queue

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewWorker verifies that NewWorker creates a non-nil Asynq server.
func TestNewWorker(t *testing.T) {
	srv := NewWorker(RedisConfig{Addr: "localhost:6379"})
	assert.NotNil(t, srv, "NewWorker should return a non-nil server")
}

// TestNewWorkerWithAuth verifies that NewWorker accepts password and DB config.
func TestNewWorkerWithAuth(t *testing.T) {
	srv := NewWorker(RedisConfig{Addr: "localhost:6379", Password: "secret", DB: 2})
	assert.NotNil(t, srv, "NewWorker should return a non-nil server with auth config")
}

// TestNewClient verifies that NewClient creates a non-nil Asynq client.
func TestNewClient(t *testing.T) {
	client := NewClient(RedisConfig{Addr: "localhost:6379"})
	assert.NotNil(t, client, "NewClient should return a non-nil client")
	defer client.Close()
}

// TestNewMux verifies that NewMux creates a ServeMux and registers handlers.
func TestNewMux(t *testing.T) {
	handler := asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		return nil
	})

	handlers := map[string]asynq.Handler{
		TaskImportTerminal: handler,
		TaskExportTerminal: handler,
		TaskImportMerchant: handler,
	}

	mux := NewMux(handlers)
	assert.NotNil(t, mux, "NewMux should return a non-nil ServeMux")
}

// TestTaskConstants verifies that all task type constants are defined and unique.
func TestTaskConstants(t *testing.T) {
	tasks := []struct {
		name  string
		value string
	}{
		{"TaskImportTerminal", TaskImportTerminal},
		{"TaskExportTerminal", TaskExportTerminal},
		{"TaskImportMerchant", TaskImportMerchant},
		{"TaskSyncParameter", TaskSyncParameter},
		{"TaskExportAll", TaskExportAll},
		{"TaskTMSPing", TaskTMSPing},
		{"TaskSchedulerCheck", TaskSchedulerCheck},
	}

	// Verify all constants are non-empty.
	for _, tc := range tasks {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEmpty(t, tc.value, "%s should not be empty", tc.name)
		})
	}

	// Verify all constants are unique.
	seen := make(map[string]string)
	for _, tc := range tasks {
		if existing, ok := seen[tc.value]; ok {
			t.Errorf("duplicate task type value %q: %s and %s", tc.value, existing, tc.name)
		}
		seen[tc.value] = tc.name
	}
}

// TestTaskConstantValues verifies that task constants have the expected values.
func TestTaskConstantValues(t *testing.T) {
	assert.Equal(t, "import:terminal", TaskImportTerminal)
	assert.Equal(t, "export:terminal", TaskExportTerminal)
	assert.Equal(t, "import:merchant", TaskImportMerchant)
	assert.Equal(t, "sync:parameter", TaskSyncParameter)
	assert.Equal(t, "export:all_terminals", TaskExportAll)
	assert.Equal(t, "tms:ping", TaskTMSPing)
	assert.Equal(t, "tms:scheduler_check", TaskSchedulerCheck)
}

// TestCellValue verifies the cellValue helper function.
func TestCellValue(t *testing.T) {
	row := []string{"a", "b", "c"}

	assert.Equal(t, "a", cellValue(row, 0))
	assert.Equal(t, "b", cellValue(row, 1))
	assert.Equal(t, "c", cellValue(row, 2))
	assert.Equal(t, "", cellValue(row, 3), "out of bounds should return empty string")
	assert.Equal(t, "", cellValue(row, 100), "far out of bounds should return empty string")
}

// TestIndexToColumnLetter verifies the Excel column letter conversion.
func TestIndexToColumnLetter(t *testing.T) {
	tests := []struct {
		idx      int
		expected string
	}{
		{0, "A"},
		{1, "B"},
		{5, "F"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{36, "AK"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := indexToColumnLetter(tc.idx)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestGetFieldName verifies the Excel column to TMS parameter field mapping.
func TestGetFieldName(t *testing.T) {
	// Known mappings.
	assert.Equal(t, "TP-PRINT_CONFIG-HEADER1-1", getFieldName("F"))
	assert.Equal(t, "TP-MERCHANT-TERMINAL_ID-1", getFieldName("K"))
	assert.Equal(t, "TP-MERCHANT-MERCHANT_ID-1", getFieldName("L"))
	assert.Equal(t, "TP-MERCHANT-MERCHANT_ID-10", getFieldName("AK"))

	// Unknown column.
	assert.Equal(t, "", getFieldName("B"))
	assert.Equal(t, "", getFieldName("ZZ"))
}

// TestExtractLastIndex verifies the index extraction from data names.
func TestExtractLastIndex(t *testing.T) {
	assert.Equal(t, 3, extractLastIndex("TP-HOST-HOST_NAME-3"))
	assert.Equal(t, 1, extractLastIndex("TP-MERCHANT-ENABLE-1"))
	assert.Equal(t, 10, extractLastIndex("TP-MERCHANT-TERMINAL_ID-10"))
	assert.Equal(t, 0, extractLastIndex("invalid"))
}

// TestSafeAddrPtr verifies the safe address pointer helper.
func TestSafeAddrPtr(t *testing.T) {
	assert.Nil(t, safeAddrPtr(""))
	assert.Nil(t, safeAddrPtr("null"))

	result := safeAddrPtr("123 Main St")
	require.NotNil(t, result)
	assert.Equal(t, "123 Main St", *result)
}

// TestStrPtr verifies the string pointer helper.
func TestStrPtr(t *testing.T) {
	result := strPtr("test")
	require.NotNil(t, result)
	assert.Equal(t, "test", *result)
}
