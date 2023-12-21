package credentials

import "fmt"

const (
	// format strings for printing GCP credentials as a string or as environment variables
	gcpCredentialsStringFormat = `If this is your first time, run "gcloud auth login" and then
gcloud config set project %s`
	gcpExportFormat = `export CLOUDSDK_CORE_PROJECT=%s`
)

type GCPCredentialsResponse struct {
	ProjectID string `json:"project_id" yaml:"project_id"`
}

func (r *GCPCredentialsResponse) String() string {
	return fmt.Sprintf(gcpCredentialsStringFormat, r.ProjectID)
}

func (r *GCPCredentialsResponse) FmtExport() string {
	return fmt.Sprintf(gcpExportFormat, r.ProjectID)
}
