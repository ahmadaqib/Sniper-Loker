package tests

import (
	"github.com/goravel/framework/testing"

	"Goravel-learn/bootstrap"
)

func init() {
	bootstrap.Boot()
}

type TestCase struct {
	testing.TestCase
}
