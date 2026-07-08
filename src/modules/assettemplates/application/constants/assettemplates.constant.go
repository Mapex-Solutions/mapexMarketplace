package constants

// Listing pagination bounds. A request without perPage gets DefaultPerPage; a
// request asking for more than MaxPerPage is capped to protect the response size.
const (
	DefaultPerPage = 24
	MaxPerPage     = 100
)
