package globalConfig

import "unsafe"

type IHandler interface {
	InitGlobalConfig() bool
	ParseConfig()
	Config() *GlobalConfig
}

type Handler struct {
	ptr unsafe.Pointer
}