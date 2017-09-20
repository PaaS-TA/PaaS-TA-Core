package cf_tcp_router

// Error strings
const (
	ErrInvalidMapingRequest     = "Invalid mapping request"
	ErrRouterConfigFileNotFound = "Configuration file not found"
	ErrRouterEmptyConfigFile    = "Configuration file not specified"
	ErrInvalidStartFrontendPort = "Invalid start frontend port"
)

// Constants used by router-configurer
const (
	LowerBoundStartFrontendPort = 1024
)

type ErrInvalidField struct {
	Field string
}

func (err ErrInvalidField) Error() string {
	return "Invalid field: " + err.Field
}
