package model

import (
	"errors"
	"strings"

	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/luoyunpeng/go-fastdfs/pkg"
)

func CheckScene(scene string, conf *config.Config) (bool, error) {
	var scenes []string

	if len(conf.Scenes()) == 0 {
		return true, nil
	}
	for _, s := range conf.Scenes() {
		scenes = append(scenes, strings.Split(s, ":")[0])
	}

	if !pkg.Contains(scene, scenes) {
		return false, errors.New("not valid scene")
	}

	return true, nil
}
