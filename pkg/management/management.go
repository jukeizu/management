package management

type Service interface {
	ValidatePermissions(userId string, channelId string) error
	Clean(userId string, channelId string) error
}
