package slack

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-joe/joe"
	"github.com/nlopes/slack"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// BotAdapter implements a joe.Adapter that reads and writes messages to and
// from Slack.
type BotAdapter struct {
	context context.Context
	logger  *zap.Logger
	name    string
	userID  string

	sendMsgParams slack.PostMessageParameters

	slack  slackAPI
	events chan slack.RTMEvent

	usersMu sync.RWMutex
	users   map[string]joe.User
}

// Config contains the configuration of a BotAdapter.
type Config struct {
	Token  string
	Name   string
	Debug  bool
	Logger *zap.Logger

	// SendMsgParams contains settings that are applied to all messages sent
	// by the BotAdapter.
	SendMsgParams slack.PostMessageParameters
}

type slackAPI interface {
	AuthTestContext(context.Context) (*slack.AuthTestResponse, error)
	PostMessageContext(ctx context.Context, channelID string, opts ...slack.MsgOption) (respChannel, respTimestamp string, err error)
	GetUserInfo(user string) (*slack.User, error)
	Disconnect() error
}

// Adapter returns a new slack Adapter as joe.Module.
func Adapter(token string, opts ...Option) joe.Module {
	return joe.ModuleFunc(func(joeConf *joe.Config) error {
		conf := Config{Token: token, Name: joeConf.Name}
		conf.SendMsgParams = slack.PostMessageParameters{
			LinkNames: 1,
			Parse:     "full",
		}

		for _, opt := range opts {
			err := opt(&conf)
			if err != nil {
				return err
			}
		}

		if conf.Logger == nil {
			conf.Logger = joeConf.Logger("slack")
		}

		a, err := NewAdapter(joeConf.Context, conf)
		if err != nil {
			return err
		}

		joeConf.SetAdapter(a)
		return nil
	})
}

// NewAdapter creates a new *BotAdapter that connects to Slack. Note that you
// will usually configure the slack adapter as joe.Module (i.e. using the
// Adapter function of this package).
func NewAdapter(ctx context.Context, conf Config) (*BotAdapter, error) {
	var slackClient struct {
		*slack.Client
		*slack.RTM
	}

	slackClient.Client = slack.New(conf.Token, slack.OptionDebug(conf.Debug)) // TODO: logger option?
	slackClient.RTM = slackClient.Client.NewRTM()

	// Start managing the slack Real Time Messaging (RTM) connection.
	// This goroutine is closed when the BotAdapter disconnects from slack in
	// BotAdapter.Close()
	go slackClient.RTM.ManageConnection()
	return newAdapter(ctx, slackClient, slackClient.RTM.IncomingEvents, conf)
}

func newAdapter(ctx context.Context, client slackAPI, events chan slack.RTMEvent, conf Config) (*BotAdapter, error) {
	a := &BotAdapter{
		slack:         client,
		events:        events,
		context:       ctx,
		logger:        conf.Logger,
		sendMsgParams: conf.SendMsgParams,
		users:         map[string]joe.User{}, // TODO: cache expiration?
	}

	if a.logger == nil {
		a.logger = zap.NewNop()
	}

	resp, err := client.AuthTestContext(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "slack auth test failed")
	}

	a.userID = resp.UserID
	a.logger.Info("Connected to slack API",
		zap.String("url", resp.URL),
		zap.String("user", resp.User),
		zap.String("user_id", resp.UserID),
		zap.String("team", resp.Team),
		zap.String("team_id", resp.TeamID),
	)

	return a, nil
}

// RegisterAt implements the joe.Adapter interface by emitting the slack API
// events to the given brain.
func (a *BotAdapter) RegisterAt(brain *joe.Brain) {
	go a.handleSlackEvents(brain)
}

func (a *BotAdapter) handleSlackEvents(brain *joe.Brain) {
	for msg := range a.events {
		switch ev := msg.Data.(type) {
		case *slack.HelloEvent:
			// Ignore hello

		case *slack.MessageEvent:
			a.handleMessageEvent(ev, brain)

		case *slack.RTMError:
			a.logger.Error("Slack Real Time Messaging (RTM) error", zap.Any("event", ev))

		case *slack.InvalidAuthEvent:
			a.logger.Error("Invalid authentication error", zap.Any("event", ev))
			return

		case *slack.UserTypingEvent:
			brain.Emit(joe.UserTypingEvent{
				User:    a.userByID(ev.User),
				Channel: ev.Channel,
			})

		default:
			// Ignore other events..
		}
	}

}

func (a *BotAdapter) handleMessageEvent(ev *slack.MessageEvent, brain *joe.Brain) {
	// check if we have a DM, or standard channel post
	selfLink := a.userLink(a.userID)
	direct := strings.HasPrefix(ev.Msg.Channel, "D")
	if !direct && !strings.Contains(ev.Msg.Text, selfLink) {
		// msg not for us!
		return
	}

	text := strings.TrimSpace(strings.TrimPrefix(ev.Text, selfLink))
	brain.Emit(joe.ReceiveMessageEvent{
		Text:    text,
		Channel: ev.Channel,
	})
}

func (a *BotAdapter) userByID(userID string) joe.User {
	a.usersMu.RLock()
	user, ok := a.users[userID]
	a.usersMu.RUnlock()

	if ok {
		return user
	}

	resp, err := a.slack.GetUserInfo(userID)
	if err != nil {
		a.logger.Error("Failed to get user info by ID",
			zap.String("user_id", userID),
		)
	}

	user = joe.User{
		ID:       resp.ID,
		Name:     resp.Name,
		RealName: resp.RealName,
	}

	a.usersMu.Lock()
	a.users[userID] = user
	a.usersMu.Unlock()

	return user
}

// Send implements joe.Adapter by sending all received text messages to the
// given slack channel ID.
func (a *BotAdapter) Send(text, channelID string) error {
	a.logger.Info("Sending message to channel",
		zap.String("channel_id", channelID),
		// do not leak actual message content since it might be sensitive
	)

	_, _, err := a.slack.PostMessageContext(a.context, channelID,
		slack.MsgOptionText(text, false),
		slack.MsgOptionPostMessageParameters(a.sendMsgParams),
		slack.MsgOptionUser(a.userID),
		slack.MsgOptionUsername(a.name),
	)

	return err
}

// Close disconnects the adapter from the slack API.
func (a *BotAdapter) Close() error {
	return a.slack.Disconnect()
}

// As long as github.com/nlopes/slack does not support the "link_names=1"
// argument we have to format the user link ourselves.
// See https://api.slack.com/docs/message-formatting#linking_to_channels_and_users
func (a *BotAdapter) userLink(userID string) string {
	return fmt.Sprintf("<@%s>", userID)
}
