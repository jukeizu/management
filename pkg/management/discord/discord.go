package discord

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jukeizu/management/pkg/management"
	"github.com/rs/zerolog"
)

const ManageMessagesPermission = 0x00002000
const MessagesRequestLimit = 100

var _ management.Service = &DefaultService{}

type DefaultService struct {
	session *discordgo.Session
	logger  zerolog.Logger
}

func NewDefaultService(logger zerolog.Logger, token string) (*DefaultService, error) {
	dh := DefaultService{
		logger: logger,
	}

	discordgo.Logger = dh.discordLogger

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return &dh, err
	}

	session.LogLevel = discordgo.LogInformational
	session.State.MaxMessageCount = 20

	// Enable all intents to include privileged intents
	session.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAll)

	dh.session = session
	return &dh, nil
}

func (s *DefaultService) ValidatePermissions(userID string, channelID string) error {
	s.logger.Info().
		Str("userID", userID).
		Str("channelID", channelID).
		Msg("checking user permissions")

	permissions, err := s.session.UserChannelPermissions(userID, channelID)
	if err != nil {
		return err
	}

	if !(permissions&ManageMessagesPermission == ManageMessagesPermission) {
		s.logger.Info().
			Str("userID", userID).
			Str("channelID", channelID).
			Int64("userPermissions", int64(permissions)).
			Int64("requiredPermissions", int64(ManageMessagesPermission)).
			Msg("user did not have required permissions")

		return management.ErrUserPermissions
	}

	s.logger.Info().
		Str("userID", userID).
		Str("channelID", channelID).
		Int64("userPermissions", int64(permissions)).
		Int64("requiredPermissions", int64(ManageMessagesPermission)).
		Msg("user has required permissions")

	return nil
}

func (s *DefaultService) Clean(userID string, channelID string) error {
	s.logger.Info().
		Str("userID", userID).
		Str("channelID", channelID).
		Msg("received clean request")

	channel, err := s.session.Channel(channelID)
	if err != nil {
		return err
	}

	before := ""
	after := "0"
	singleIDTotal := 0
	bulkIDTotal := 0
	rateLimitWarningSent := false

	for {
		s.logger.Info().
			Str("userID", userID).
			Str("channelID", channelID).
			Str("before", before).
			Str("after", after).
			Int32("limit", MessagesRequestLimit).
			Msg("looking for messages in channel")

		messages, err := s.session.ChannelMessages(channelID, MessagesRequestLimit, before, after, "")
		if err != nil {
			return err
		}
		if len(messages) < 1 {
			break
		}

		s.logger.Info().
			Str("userID", userID).
			Str("channelID", channelID).
			Str("before", before).
			Str("after", after).
			Int32("limit", MessagesRequestLimit).
			Int32("count", int32(len(messages))).
			Msg("found messages in channel")

		bulkIds, singleIds, err := s.findDeletableMessages(messages)
		if err != nil {
			return err
		}

		bulkIDTotal += len(bulkIds)
		singleIDTotal += len(singleIds)

		if !rateLimitWarningSent && singleIDTotal > 120 {
			_, err := s.session.ChannelMessageSend(channelID, "This may take a while...")
			if err != nil {
				return err
			}

			rateLimitWarningSent = true
		}

		err = s.deleteMessages(channelID, bulkIds, singleIds)
		if err != nil {
			return err
		}

		before = messages[0].ID
		after = messages[len(messages)-1].ID

		if before == channel.LastMessageID {
			break
		}
	}

	s.logger.Info().
		Str("userID", userID).
		Str("channelID", channelID).
		Int32("singleIDTotal", int32(singleIDTotal)).
		Int32("bulkIDTotal", int32(bulkIDTotal)).
		Msg("finished cleaning")

	return nil
}

func (s *DefaultService) deleteMessages(channelID string, bulkIds []string, singleIds []string) error {
	if len(bulkIds) == 0 && len(singleIds) == 0 {
		s.logger.Info().
			Str("channelID", channelID).
			Msg("no deletable messages")

		return nil
	}

	s.logger.Info().
		Str("channelID", channelID).
		Int32("bulkMessageDeleteCount", int32(len(bulkIds))).
		Int32("singleMessageDeleteCount", int32(len(singleIds))).
		Msg("found deletable messages")

	s.logger.Info().
		Str("channelID", channelID).
		Msg("beginning single message delete")

	for _, id := range singleIds {
		err := s.session.ChannelMessageDelete(channelID, id)
		if err != nil {
			return err
		}
		s.logger.Info().
			Str("channelID", channelID).
			Str("messageID", id).
			Msg("deleted message")
	}

	s.logger.Info().
		Str("channelID", channelID).
		Msg("finished single message delete")

	s.logger.Info().
		Str("channelID", channelID).
		Msg("beginning bulk message delete")

	err := s.session.ChannelMessagesBulkDelete(channelID, bulkIds)
	if err != nil {
		return err
	}

	s.logger.Info().
		Str("channelID", channelID).
		Msg("finished bulk message delete")

	return nil
}

func (s *DefaultService) findDeletableMessages(messages []*discordgo.Message) ([]string, []string, error) {
	bulkIds := []string{}
	singleIds := []string{}

	for _, message := range messages {
		if s.skippable(message) {
			continue
		}

		t, err := discordgo.SnowflakeTimestamp(message.ID)
		if err != nil {
			return bulkIds, singleIds, err
		}

		// Bulk delete can only go 2 weeks back. Only go one week back here to guarantee none are skipped.
		if t.After(time.Now().UTC().Add(-time.Hour * 168)) {
			bulkIds = append(bulkIds, message.ID)
		} else {
			singleIds = append(singleIds, message.ID)
		}
	}

	return bulkIds, singleIds, nil
}

func (s *DefaultService) skippable(m *discordgo.Message) bool {
	if m.Pinned {
		s.logger.Info().
			Str("channelID", m.ChannelID).
			Str("messageID", m.ID).
			Msg("skipping pinned message")

		return true
	}

	for _, reaction := range m.Reactions {
		if reaction.Emoji.Name == "💾" {
			s.logger.Info().
				Str("channelID", m.ChannelID).
				Str("messageID", m.ID).
				Str("reaction", reaction.Emoji.Name).
				Msg("skipping message with reaction")

			return true
		}
	}

	return false
}

func (s *DefaultService) discordLogger(dgoLevel int, caller int, format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)

	s.logger.WithLevel(mapToLevel(dgoLevel)).
		Str("component", "discordgo").
		Str("version", discordgo.VERSION).
		Msg(message)
}

func mapToLevel(dgoLevel int) zerolog.Level {
	switch dgoLevel {
	case discordgo.LogError:
		return zerolog.ErrorLevel
	case discordgo.LogWarning:
		return zerolog.WarnLevel
	case discordgo.LogInformational:
		return zerolog.InfoLevel
	case discordgo.LogDebug:
		return zerolog.DebugLevel
	}

	return zerolog.InfoLevel
}
