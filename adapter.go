// Package slack implements a slack adapter for the joe bot library.
package slack

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-joe/joe"
	"github.com/go-joe/joe/reactions"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

// BotAdapter implements a joe.Adapter that reads and writes messages to and
// from Slack using the RTM API.
type BotAdapter struct {
	context context.Context
	logger  *zap.Logger
	name    string
	userID  string

	logUnknownMessageTypes bool
	listenPassive          bool

	sendMsgParams slack.PostMessageParameters

	slack  slackAPI
	rtm    slackRTM
	events chan slackEvent

	usersMu sync.RWMutex
	users   map[string]joe.User
}

type slackEvent struct {
	Type string
	Data interface{}
}

type slackAPI interface {
	AuthTestContext(context.Context) (*slack.AuthTestResponse, error)
	PostMessageContext(ctx context.Context, channelID string, opts ...slack.MsgOption) (respChannel, respTimestamp string, err error)
	AddReactionContext(ctx context.Context, name string, item slack.ItemRef) error
	GetUserInfo(user string) (*slack.User, error)
}

type slackRTM interface {
	Disconnect() error
}

// Adapter returns a new BotAdapter as joe.Module.
//
// Apart from the typical joe.ReceiveMessageEvent event, this adapter also emits
// the joe.UserTypingEvent. The ReceiveMessageEvent.Data field is always a
// pointer to the corresponding github.com/slack-go/slack.MessageEvent instance.
func Adapter(token string, opts ...Option) joe.Module {
	return joe.ModuleFunc(func(joeConf *joe.Config) error {
		conf, err := newConf(token, "", joeConf, opts)
		if err != nil {
			return err
		}

		a, err := NewAdapter(joeConf.Context, conf)
		if err != nil {
			return err
		}

		joeConf.SetAdapter(a)
		return nil
	})
}

func newConf(token string, verificationToken string, joeConf *joe.Config, opts []Option) (Config, error) {
	conf := Config{Token: token, VerificationToken: verificationToken, Name: joeConf.Name}
	conf.SendMsgParams = slack.PostMessageParameters{
		LinkNames: 1,
		Parse:     "full",
		AsUser:    true,
	}

	for _, opt := range opts {
		err := opt(&conf)
		if err != nil {
			return conf, err
		}
	}

	if conf.Logger == nil {
		conf.Logger = joeConf.Logger("slack")
	}

	return conf, nil
}

// NewAdapter creates a new *BotAdapter that connects to Slack using the RTM API.
// Note that you will usually configure the slack adapter as joe.Module (i.e.
// using the Adapter function of this package).
func NewAdapter(ctx context.Context, conf Config) (*BotAdapter, error) {
	client := slack.New(conf.Token, slack.OptionDebug(conf.Debug))
	rtm := client.NewRTM()

	// Start managing the slack Real Time Messaging (RTM) connection.
	// This goroutine is closed when the BotAdapter disconnects from slack in
	// BotAdapter.Close()
	go rtm.ManageConnection()

	// We need to translate the RTMEvent channel into the more generic slackEvent
	// channel which is used by the BotAdapter internally.
	events := make(chan slackEvent)
	go func() {
		defer close(events)
		for evt := range rtm.IncomingEvents {
			events <- slackEvent{
				Type: evt.Type,
				Data: evt.Data,
			}

			if x, ok := evt.Data.(*slack.DisconnectedEvent); ok && x.Intentional {
				return
			}
		}
	}()

	return newAdapter(ctx, client, rtm, events, conf)
}

func newAdapter(ctx context.Context, client slackAPI, rtm slackRTM, events chan slackEvent, conf Config) (*BotAdapter, error) {
	a := &BotAdapter{
		slack:         client,
		rtm:           rtm, // may be nil
		events:        events,
		context:       ctx,
		logger:        conf.Logger,
		name:          conf.Name,
		sendMsgParams: conf.SendMsgParams,
		users:         map[string]joe.User{}, // TODO: cache expiration?
		listenPassive: conf.ListenPassive,
	}

	if a.logger == nil {
		a.logger = zap.NewNop()
	}

	resp, err := client.AuthTestContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("slack auth test failed: %w", err)
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
		case *slack.MessageEvent:
			a.handleMessageEvent(ev, brain)

		case *slack.ReactionAddedEvent:
			a.handleReactionAddedEvent(ev, brain)

		case *slack.RTMError:
			a.logger.Error("Slack Real Time Messaging (RTM) error",
				zap.Int("code", ev.Code),
				zap.String("msg", ev.Msg),
			)

		case *slack.UnmarshallingErrorEvent:
			a.logger.Error("Slack unmarshalling error", zap.Error(ev.ErrorObj))

		case *slack.InvalidAuthEvent:
			a.logger.Error("Invalid authentication error", zap.Any("event", ev))
			return

		case *slack.UserTypingEvent:
			brain.Emit(joe.UserTypingEvent{
				User:    a.userByID(ev.User),
				Channel: ev.Channel,
			})

		case *slack.DisconnectedEvent:
			if ev.Intentional {
				a.logger.Debug("Disconnected slack adapter")
				return
			}

		default:
			if a.logUnknownMessageTypes {
				a.logger.Error("Received unknown type from Real Time Messaging (RTM) system",
					zap.String("type", msg.Type),
					zap.Any("data", msg.Data),
					zap.String("go_type", fmt.Sprintf("%T", msg.Data)),
				)
			}
		}
	}
}

func (a *BotAdapter) handleMessageEvent(ev *slack.MessageEvent, brain joe.EventEmitter) {
	// check if the message comes from ourselves
	if ev.User == a.userID {
		// msg is from us, ignore it!
		return
	}

	// check if we have a DM, or standard channel post
	selfLink := a.userLink(a.userID)
	direct := strings.HasPrefix(ev.Msg.Channel, "D")
	if !direct && !strings.Contains(ev.Msg.Text, selfLink) && !a.listenPassive {
		// msg not for us!
		return
	}

	text := strings.TrimSpace(strings.TrimPrefix(ev.Msg.Text, selfLink))
	brain.Emit(joe.ReceiveMessageEvent{
		Text:     text,
		Channel:  ev.Channel,
		ID:       ev.Timestamp, // slack uses the message timestamps as identifiers within the channel
		AuthorID: ev.User,
		Data:     ev,
	})
}

// See https://api.slack.com/events/reaction_added
func (a *BotAdapter) handleReactionAddedEvent(ev *slack.ReactionAddedEvent, brain joe.EventEmitter) {
	if ev.User == a.userID {
		// reaction is from us, ignore it!
		return
	}

	if ev.Item.Type != "message" {
		// reactions for other things except messages is not supported by Joe
		return
	}

	brain.Emit(reactions.Event{
		Channel:   ev.Item.Channel,
		MessageID: ev.Item.Timestamp,
		AuthorID:  ev.User,
		Reaction:  reactions.Reaction{Shortcode: ev.Reaction},
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
		return joe.User{ID: userID}
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

// React implements joe.ReactionAwareAdapter by letting the bot attach the given
// reaction to the message.
func (a *BotAdapter) React(reaction reactions.Reaction, msg joe.Message) error {
	ref := slack.NewRefToMessage(msg.Channel, msg.ID)
	return a.slack.AddReactionContext(a.context, reaction.Shortcode, ref)
}

// Close disconnects the adapter from the slack API.
func (a *BotAdapter) Close() error {
	if a.rtm != nil {
		return a.rtm.Disconnect()
	}

	return nil
}

// As long as github.com/slack-go/slack does not support the "link_names=1"
// argument we have to format the user link ourselves.
// See https://api.slack.com/docs/message-formatting#linking_to_channels_and_users
func (a *BotAdapter) userLink(userID string) string {
	return fmt.Sprintf("<@%s>", userID)
}
