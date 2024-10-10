package info

import "runtime/debug"

type BuildInfoService interface {
	// return the BuildInfo from Go build
	GetBuildInfo() (info *debug.BuildInfo, ok bool)
}

type DefaultBuildInfoServiceImpl struct {
}

func (b *DefaultBuildInfoServiceImpl) GetBuildInfo() (info *debug.BuildInfo, ok bool) {
	return debug.ReadBuildInfo()
}

var DefaultBuildInfoService BuildInfoService = &DefaultBuildInfoServiceImpl{}
