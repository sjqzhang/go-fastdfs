package api

import (
	"net/http"
	"testing"

	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestRemoveEmptyDir(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		app, router := NewApiTest()
		RemoveEmptyDir("/remove_empty_dir", router, config.NewConfig())
		result := PerformRequest(app, "GET", "/api/v1/albums?count=10")

		assert.Equal(t, http.StatusOK, result.Code)
	})

}
