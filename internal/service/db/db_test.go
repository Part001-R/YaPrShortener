package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_workDir_SUCCESS(t *testing.T) {

	_, err := workDir()
	require.NoErrorf(t, err, "неожиданная ошибка при определении рабочей директории: <v>", err)

}
