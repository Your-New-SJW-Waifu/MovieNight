package main

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode"

	"github.com/zorchenhimer/MovieNight/common"
)

var (
	regexSpoiler = regexp.MustCompile(`\|\|(.*?)\|\|`)
	spoilerStart = `<span class="spoiler" onclick='$(this).removeClass("spoiler").addClass("spoiler-active")'>`
	spoilerEnd   = `</span>`
)

type Client struct {
	name          string // Display name
	conn          *chatConnection
	belongsTo     *ChatRoom
	color         string
	CmdLevel      common.CommandLevel
	IsColorForced bool
	IsNameForced  bool
	regexName     *regexp.Regexp
}

//Client has a new message to broadcast
func (cl *Client) NewMsg(data common.ClientData) {
	switch data.Type {
	case common.CdAuth:
		common.LogChatf("[chat|hidden] <%s> get auth level\n", cl.name)
		err := cl.SendChatData(common.NewChatHiddenMessage(data.Type, cl.CmdLevel))
		if err != nil {
			common.LogErrorf("Error sending auth level to client: %v\n", err)
		}
	case common.CdUsers:
		common.LogChatf("[chat|hidden] <%s> get list of users\n", cl.name)

		names := chat.GetNames()
		idx := -1
		for i := range names {
			if names[i] == cl.name {
				idx = i
			}
		}

		err := cl.SendChatData(common.NewChatHiddenMessage(data.Type, append(names[:idx], names[idx+1:]...)))
		if err != nil {
			common.LogErrorf("Error sending chat data: %v\n", err)
		}
	case common.CdMessage:
		msg := html.EscapeString(data.Message)
		msg = removeDumbSpaces(msg)
		msg = strings.Trim(msg, " ")

		// Add the spoiler tag outside of the command vs message statement
		// because the /me command outputs to the messages
		msg = addSpoilerTags(msg)

		// Don't send zero-length messages
		if len(msg) == 0 {
			return
		}

		if strings.HasPrefix(msg, "/") {
			// is a command
			msg = msg[1:len(msg)]
			fullcmd := strings.Split(msg, " ")
			cmd := strings.ToLower(fullcmd[0])
			args := fullcmd[1:len(fullcmd)]

			response := commands.RunCommand(cmd, args, cl)
			if response != "" {
				err := cl.SendChatData(common.NewChatMessage("", "",
					common.ParseEmotes(response),
					common.CmdlUser,
					common.MsgCommandResponse))
				if err != nil {
					common.LogErrorf("Error command results %v\n", err)
				}
				return
			}

		} else {
			// Trim long messages
			if len(msg) > 400 {
				msg = msg[0:400]
			}

			common.LogChatf("[chat] <%s> %q\n", cl.name, msg)

			// Enable links for mods and admins
			if cl.CmdLevel >= common.CmdlMod {
				msg = formatLinks(msg)
			}

			cl.Message(msg)
		}
	}
}

func (cl *Client) SendChatData(data common.ChatData) error {
	// Colorize name on chat messages
	if data.Type == common.DTChat {
		var err error
		data = cl.replaceColorizedName(data)
		if err != nil {
			return fmt.Errorf("could not colorize name: %v", err)
		}
	}

	cd, err := data.ToJSON()
	if err != nil {
		return fmt.Errorf("could not create ChatDataJSON of type %d: %v", data.Type, err)
	}
	return cl.Send(cd)
}

func (cl *Client) Send(data common.ChatDataJSON) error {
	err := cl.conn.WriteData(data)
	if err != nil {
		return fmt.Errorf("could not send message: %v", err)
	}
	return nil
}

func (cl *Client) SendServerMessage(s string) error {
	err := cl.SendChatData(common.NewChatMessage("", ColorServerMessage, s, common.CmdlUser, common.MsgServer))
	if err != nil {
		return fmt.Errorf("could send server message to %s: message - %#v: %v", cl.name, s, err)
	}
	return nil
}

// Make links clickable
func formatLinks(input string) string {
	newMsg := []string{}
	for _, word := range strings.Split(input, " ") {
		if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") {
			word = html.UnescapeString(word)
			word = fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, word, word)
		}
		newMsg = append(newMsg, word)
	}
	return strings.Join(newMsg, " ")
}

//Exiting out
func (cl *Client) Exit() {
	cl.belongsTo.Leave(cl.name, cl.color)
}

// Outgoing messages
func (cl *Client) Message(msg string) {
	msg = common.ParseEmotes(msg)
	cl.belongsTo.AddMsg(cl, false, false, msg)
}

// Outgoing /me command
func (cl *Client) Me(msg string) {
	msg = common.ParseEmotes(msg)
	cl.belongsTo.AddMsg(cl, true, false, msg)
}

func (cl *Client) Mod() {
	if cl.CmdLevel < common.CmdlMod {
		cl.CmdLevel = common.CmdlMod
	}
}

func (cl *Client) Unmod() {
	cl.CmdLevel = common.CmdlUser
}

func (cl *Client) Host() string {
	return cl.conn.Host()
}

func (cl *Client) setName(s string) error {
	// Case-insensitive search.  Match whole words only (`\b` is word boundary).
	regex, err := regexp.Compile(fmt.Sprintf(`(?i)\b(%s|@%s)\b`, s, s))
	if err != nil {
		return fmt.Errorf("could not compile regex: %v", err)
	}

	cl.name = s
	cl.regexName = regex
	return nil
}

func (cl *Client) setColor(s string) error {
	cl.color = s
	return cl.SendChatData(common.NewChatHiddenMessage(common.CdColor, cl.color))
}

func (cl *Client) replaceColorizedName(chatData common.ChatData) common.ChatData {
	data := chatData.Data.(common.DataMessage)
	data.Message = cl.regexName.ReplaceAllString(data.Message, `<span class="mention">$1</span>`)
	chatData.Data = data
	return chatData
}

var dumbSpaces = []string{
	"\n",
	"\t",
	"\r",
	"\u200b",
}

func removeDumbSpaces(msg string) string {
	for _, ds := range dumbSpaces {
		msg = strings.ReplaceAll(msg, ds, " ")
	}

	newMsg := ""
	for _, r := range msg {
		if unicode.IsSpace(r) {
			newMsg += " "
		} else {
			newMsg += string(r)
		}
	}
	return newMsg
}

func addSpoilerTags(msg string) string {
	return regexSpoiler.ReplaceAllString(msg, fmt.Sprintf(`%s$1%s`, spoilerStart, spoilerEnd))
}
