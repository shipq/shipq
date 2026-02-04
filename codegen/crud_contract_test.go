package codegen

import "testing"

func TestCRUDContract_MethodNames(t *testing.T) {
	tests := []struct {
		name     string
		table    string
		method   func(string) string
		expected string
	}{
		// GetMethodName tests
		{"GetMethodName accounts", "accounts", CRUD.GetMethodName, "GetAccountByPublicID"},
		{"GetMethodName users", "users", CRUD.GetMethodName, "GetUserByPublicID"},
		{"GetMethodName user_profiles", "user_profiles", CRUD.GetMethodName, "GetUserProfileByPublicID"},
		{"GetMethodName categories", "categories", CRUD.GetMethodName, "GetCategoryByPublicID"},

		// ListMethodName tests
		{"ListMethodName accounts", "accounts", CRUD.ListMethodName, "ListAccounts"},
		{"ListMethodName users", "users", CRUD.ListMethodName, "ListUsers"},
		{"ListMethodName user_profiles", "user_profiles", CRUD.ListMethodName, "ListUserProfiles"},

		// CreateMethodName tests
		{"CreateMethodName accounts", "accounts", CRUD.CreateMethodName, "CreateAccount"},
		{"CreateMethodName users", "users", CRUD.CreateMethodName, "CreateUser"},
		{"CreateMethodName user_profiles", "user_profiles", CRUD.CreateMethodName, "CreateUserProfile"},

		// UpdateMethodName tests
		{"UpdateMethodName accounts", "accounts", CRUD.UpdateMethodName, "UpdateAccountByPublicID"},
		{"UpdateMethodName users", "users", CRUD.UpdateMethodName, "UpdateUserByPublicID"},
		{"UpdateMethodName user_profiles", "user_profiles", CRUD.UpdateMethodName, "UpdateUserProfileByPublicID"},

		// SoftDeleteMethodName tests
		{"SoftDeleteMethodName accounts", "accounts", CRUD.SoftDeleteMethodName, "SoftDeleteAccountByPublicID"},
		{"SoftDeleteMethodName users", "users", CRUD.SoftDeleteMethodName, "SoftDeleteUserByPublicID"},
		{"SoftDeleteMethodName user_profiles", "user_profiles", CRUD.SoftDeleteMethodName, "SoftDeleteUserProfileByPublicID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.method(tt.table)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCRUDContract_TypeNames(t *testing.T) {
	tests := []struct {
		name     string
		table    string
		method   func(string) string
		expected string
	}{
		// GetResultType tests
		{"GetResultType accounts", "accounts", CRUD.GetResultType, "GetAccountResult"},
		{"GetResultType users", "users", CRUD.GetResultType, "GetUserResult"},

		// ListParamsType tests
		{"ListParamsType accounts", "accounts", CRUD.ListParamsType, "ListAccountsParams"},
		{"ListParamsType users", "users", CRUD.ListParamsType, "ListUsersParams"},

		// ListResultType tests
		{"ListResultType accounts", "accounts", CRUD.ListResultType, "ListAccountsResult"},
		{"ListResultType users", "users", CRUD.ListResultType, "ListUsersResult"},

		// ListItemType tests
		{"ListItemType accounts", "accounts", CRUD.ListItemType, "ListAccountsItem"},
		{"ListItemType users", "users", CRUD.ListItemType, "ListUsersItem"},

		// ListCursorType tests
		{"ListCursorType accounts", "accounts", CRUD.ListCursorType, "ListAccountsCursor"},
		{"ListCursorType users", "users", CRUD.ListCursorType, "ListUsersCursor"},

		// CreateParamsType tests
		{"CreateParamsType accounts", "accounts", CRUD.CreateParamsType, "CreateAccountParams"},
		{"CreateParamsType users", "users", CRUD.CreateParamsType, "CreateUserParams"},

		// CreateResultType tests
		{"CreateResultType accounts", "accounts", CRUD.CreateResultType, "CreateAccountResult"},
		{"CreateResultType users", "users", CRUD.CreateResultType, "CreateUserResult"},

		// UpdateParamsType tests
		{"UpdateParamsType accounts", "accounts", CRUD.UpdateParamsType, "UpdateAccountParams"},
		{"UpdateParamsType users", "users", CRUD.UpdateParamsType, "UpdateUserParams"},

		// UpdateResultType tests
		{"UpdateResultType accounts", "accounts", CRUD.UpdateResultType, "UpdateAccountResult"},
		{"UpdateResultType users", "users", CRUD.UpdateResultType, "UpdateUserResult"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.method(tt.table)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCRUDContract_CursorFuncs(t *testing.T) {
	tests := []struct {
		name     string
		table    string
		method   func(string) string
		expected string
	}{
		{"EncodeCursorFunc accounts", "accounts", CRUD.EncodeCursorFunc, "EncodeAccountsCursor"},
		{"EncodeCursorFunc users", "users", CRUD.EncodeCursorFunc, "EncodeUsersCursor"},
		{"DecodeCursorFunc accounts", "accounts", CRUD.DecodeCursorFunc, "DecodeAccountsCursor"},
		{"DecodeCursorFunc users", "users", CRUD.DecodeCursorFunc, "DecodeUsersCursor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.method(tt.table)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCRUDContract_ResourceNames(t *testing.T) {
	tests := []struct {
		name     string
		table    string
		method   func(string) string
		expected string
	}{
		{"ResourceName accounts", "accounts", CRUD.ResourceName, "Account"},
		{"ResourceName users", "users", CRUD.ResourceName, "User"},
		{"ResourceName user_profiles", "user_profiles", CRUD.ResourceName, "UserProfile"},
		{"PluralResourceName accounts", "accounts", CRUD.PluralResourceName, "Accounts"},
		{"PluralResourceName users", "users", CRUD.PluralResourceName, "Users"},
		{"PluralResourceName user_profiles", "user_profiles", CRUD.PluralResourceName, "UserProfiles"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.method(tt.table)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCRUDContract_Constants(t *testing.T) {
	if RunnerFromContextFunc != "RunnerFromContext" {
		t.Errorf("RunnerFromContextFunc = %q, want %q", RunnerFromContextFunc, "RunnerFromContext")
	}
	if NewContextWithRunnerFunc != "NewContextWithRunner" {
		t.Errorf("NewContextWithRunnerFunc = %q, want %q", NewContextWithRunnerFunc, "NewContextWithRunner")
	}
}
