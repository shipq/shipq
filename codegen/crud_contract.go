// Package codegen provides code generation utilities for shipq.
// crud_contract.go defines the naming conventions shared between
// handler generation and query runner generation.
package codegen

import (
	"fmt"

	"github.com/shipq/shipq/dbstrings"
)

// CRUDContract defines the naming conventions shared between
// handler generation (handler_gen.go) and query runner generation (unified_runner.go).
// Both generators MUST use these functions to ensure generated code stays in sync.
type CRUDContract struct{}

// CRUD is the singleton instance of CRUDContract.
var CRUD = CRUDContract{}

// =============================================================================
// Method Names (what handlers call on the runner)
// =============================================================================

// GetMethodName returns the method name for fetching a single record by public ID.
// Example: "accounts" -> "GetAccountByPublicID"
func (c CRUDContract) GetMethodName(tableName string) string {
	return fmt.Sprintf("Get%sByPublicID", dbstrings.ToPascalCase(dbstrings.ToSingular(tableName)))
}

// ListMethodName returns the method name for listing records.
// Example: "accounts" -> "ListAccounts"
func (c CRUDContract) ListMethodName(tableName string) string {
	return fmt.Sprintf("List%s", dbstrings.ToPascalCase(tableName))
}

// CreateMethodName returns the method name for creating a record.
// Example: "accounts" -> "CreateAccount"
func (c CRUDContract) CreateMethodName(tableName string) string {
	return fmt.Sprintf("Create%s", dbstrings.ToPascalCase(dbstrings.ToSingular(tableName)))
}

// UpdateMethodName returns the method name for updating a record by public ID.
// Example: "accounts" -> "UpdateAccountByPublicID"
func (c CRUDContract) UpdateMethodName(tableName string) string {
	return fmt.Sprintf("Update%sByPublicID", dbstrings.ToPascalCase(dbstrings.ToSingular(tableName)))
}

// SoftDeleteMethodName returns the method name for soft-deleting a record by public ID.
// Example: "accounts" -> "SoftDeleteAccountByPublicID"
func (c CRUDContract) SoftDeleteMethodName(tableName string) string {
	return fmt.Sprintf("SoftDelete%sByPublicID", dbstrings.ToPascalCase(dbstrings.ToSingular(tableName)))
}

// =============================================================================
// Type Names (param and result structs in queries package)
// =============================================================================

// GetResultType returns the type name for the get result.
// Example: "accounts" -> "GetAccountResult"
func (c CRUDContract) GetResultType(tableName string) string {
	return fmt.Sprintf("Get%sResult", dbstrings.ToPascalCase(dbstrings.ToSingular(tableName)))
}

// ListParamsType returns the type name for the list parameters.
// Example: "accounts" -> "ListAccountsParams"
func (c CRUDContract) ListParamsType(tableName string) string {
	return fmt.Sprintf("List%sParams", dbstrings.ToPascalCase(tableName))
}

// ListResultType returns the type name for the list result wrapper.
// Example: "accounts" -> "ListAccountsResult"
func (c CRUDContract) ListResultType(tableName string) string {
	return fmt.Sprintf("List%sResult", dbstrings.ToPascalCase(tableName))
}

// ListItemType returns the type name for individual items in a list result.
// Example: "accounts" -> "ListAccountsItem"
func (c CRUDContract) ListItemType(tableName string) string {
	return fmt.Sprintf("List%sItem", dbstrings.ToPascalCase(tableName))
}

// ListCursorType returns the type name for the pagination cursor.
// Example: "accounts" -> "ListAccountsCursor"
func (c CRUDContract) ListCursorType(tableName string) string {
	return fmt.Sprintf("List%sCursor", dbstrings.ToPascalCase(tableName))
}

// CreateParamsType returns the type name for the create parameters.
// Example: "accounts" -> "CreateAccountParams"
func (c CRUDContract) CreateParamsType(tableName string) string {
	return fmt.Sprintf("Create%sParams", dbstrings.ToPascalCase(dbstrings.ToSingular(tableName)))
}

// CreateResultType returns the type name for the create result.
// Example: "accounts" -> "CreateAccountResult"
func (c CRUDContract) CreateResultType(tableName string) string {
	return fmt.Sprintf("Create%sResult", dbstrings.ToPascalCase(dbstrings.ToSingular(tableName)))
}

// UpdateParamsType returns the type name for the update parameters.
// Example: "accounts" -> "UpdateAccountParams"
func (c CRUDContract) UpdateParamsType(tableName string) string {
	return fmt.Sprintf("Update%sParams", dbstrings.ToPascalCase(dbstrings.ToSingular(tableName)))
}

// UpdateResultType returns the type name for the update result.
// Example: "accounts" -> "UpdateAccountResult"
func (c CRUDContract) UpdateResultType(tableName string) string {
	return fmt.Sprintf("Update%sResult", dbstrings.ToPascalCase(dbstrings.ToSingular(tableName)))
}

// =============================================================================
// Helper function names in queries package
// =============================================================================

// EncodeCursorFunc returns the function name for encoding a cursor.
// Example: "accounts" -> "EncodeAccountsCursor"
func (c CRUDContract) EncodeCursorFunc(tableName string) string {
	return fmt.Sprintf("Encode%sCursor", dbstrings.ToPascalCase(tableName))
}

// DecodeCursorFunc returns the function name for decoding a cursor.
// Example: "accounts" -> "DecodeAccountsCursor"
func (c CRUDContract) DecodeCursorFunc(tableName string) string {
	return fmt.Sprintf("Decode%sCursor", dbstrings.ToPascalCase(tableName))
}

// =============================================================================
// Context function names
// =============================================================================

// RunnerFromContextFunc is the function name for getting runner from context.
const RunnerFromContextFunc = "RunnerFromContext"

// NewContextWithRunnerFunc is the function name for adding runner to context.
const NewContextWithRunnerFunc = "NewContextWithRunner"

// =============================================================================
// Resource name helper (for handler generation)
// =============================================================================

// ResourceName returns the PascalCase singular form used in handler naming.
// Example: "accounts" -> "Account"
func (c CRUDContract) ResourceName(tableName string) string {
	return dbstrings.ToPascalCase(dbstrings.ToSingular(tableName))
}

// PluralResourceName returns the PascalCase plural form used in handler naming.
// Example: "accounts" -> "Accounts"
func (c CRUDContract) PluralResourceName(tableName string) string {
	return dbstrings.ToPascalCase(tableName)
}
