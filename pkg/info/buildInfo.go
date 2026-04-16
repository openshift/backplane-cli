package info

import "runtime/debug"

//go:generate go tool mockgen -destination=mocks/buildInfoMock.go -package=mocks github.com/openshift/backplane-cli/pkg/info BuildInfoService
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
