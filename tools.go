//go:build tools

// Package tools tracks tool and future dependencies that are not yet imported
// in application code. This prevents go mod tidy from removing them.
package tools

import (
	_ "github.com/a-h/templ"
	_ "github.com/casbin/casbin/v2"
	_ "github.com/casbin/gorm-adapter/v3"
	_ "github.com/go-playground/validator/v10"
	_ "github.com/gorilla/sessions"
	_ "github.com/hibiken/asynq"
	_ "github.com/rs/zerolog"
	_ "github.com/stretchr/testify"
	_ "github.com/xuri/excelize/v2"
	_ "gopkg.in/yaml.v3"
	_ "gorm.io/driver/mysql"
	_ "gorm.io/gorm"
)
