package client

//go:generate go tool mockgen -destination=mocks/ClientMock.go -package=mocks github.com/openshift/backplane-api/pkg/client ClientInterface
//go:generate go tool mockgen -destination=mocks/ClientWithResponsesMock.go -package=mocks github.com/openshift/backplane-api/pkg/client ClientWithResponsesInterface
