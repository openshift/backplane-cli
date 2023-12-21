package credentials

type Response interface {
	// String returns a friendly message outlining how users can setup cloud environment access
	String() string

	// FmtExport sets environment variables for users to export to setup cloud environment access
	FmtExport() string
}
