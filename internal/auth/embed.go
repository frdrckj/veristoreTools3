package auth

import _ "embed"

// CasbinModel contains the embedded Casbin RBAC model configuration.
//
//go:embed casbin_model.conf
var CasbinModel string

// CasbinPolicy contains the embedded Casbin RBAC policy definitions.
//
//go:embed casbin_policy.csv
var CasbinPolicy string
