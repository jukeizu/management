package management

var (
	ErrUserPermissions = ValidationError{Message: "You do not have permission to clean this channel."}
)

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}
