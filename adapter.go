package slack

import (
	"context"
	"strings"
	"sync"

	"github.com/go-joe/joe"
	"github.com/nlopes/slack"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Config struct {
	Token  string
	Debug  bool
	Logger *zap.Logger
}

type API struct {
	context context.Context
	logger  *zap.Logger
	client  *slack.Client
	rtm     *slack.RTM
	userID  string

	usersMu sync.RWMutex
	users   map[string]joe.User
}

func Adapter(token string, opts ...Option) joe.Module {
	return func(joeConf *joe.Config) error {
		conf := Config{Token: token}
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
	}
}

// NewAdapter creates a new slack adapter. Note that you will usually configure
// the slack adapter as joe.Module (i.e. using the "slack.Adapter(…)" function.
func NewAdapter(ctx context.Context, conf Config) (joe.Adapter, error) {
	a := &API{
		client:  slack.New(conf.Token, slack.OptionDebug(conf.Debug)),
		context: ctx,
		logger:  conf.Logger,
		users:   map[string]joe.User{}, // TODO: cache expiration?
	}

	if a.logger == nil {
		a.logger = zap.NewNop()
	}

	resp, err := a.client.AuthTestContext(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "slack auth test failed")
	}

	a.userID = resp.UserID
	a.rtm = a.client.NewRTM()

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
func (a *API) RegisterAt(brain *joe.Brain) {
	// Start message handling in two goroutines. They will be closed when we
	// disconnect the RTM upon adapter.Close().
	go a.rtm.ManageConnection()
	go a.handleSlackEvents(brain)
}

func (a *API) handleSlackEvents(brain *joe.Brain) {
	for msg := range a.rtm.IncomingEvents {
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

func (a *API) handleMessageEvent(ev *slack.MessageEvent, brain *joe.Brain) {
	// check if we have a DM, or standard channel post
	direct := strings.HasPrefix(ev.Msg.Channel, "D")
	if !direct && !strings.Contains(ev.Msg.Text, "<@"+a.userID+">") {
		// msg not for us!
		return
	}

	text := strings.TrimSpace(strings.TrimPrefix(ev.Text, "<@"+a.userID+">"))
	brain.Emit(joe.ReceiveMessageEvent{
		Text:    text,
		Channel: ev.Channel,
	})
}

func (a *API) userByID(userID string) joe.User {
	a.usersMu.RLock()
	user, ok := a.users[userID]
	a.usersMu.RUnlock()

	if ok {
		return user
	}

	resp, err := a.client.GetUserInfo(userID)
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
func (a *API) Send(text, channelID string) error {
	a.logger.Info("Sending message to channel",
		zap.String("channel_id", channelID),
		// do not leak actual message content since it might be sensitive
	)

	a.rtm.SendMessage(a.rtm.NewOutgoingMessage(text, channelID))
	return nil
}

// Close disconnects the adapter from the slack API.
func (a *API) Close() error {
	return a.rtm.Disconnect()
}
